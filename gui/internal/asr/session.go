package asr

import (
	"context"
	"errors"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

// Event is what a session reports: "ready" once the server accepts audio,
// "partial" with the transcript so far, then one "final" with the whole
// transcript, or one "error" with Err.
type Event struct {
	Type string
	Text string
	Err  error
}

// Options start a session.
type Options struct {
	Config Config
	// Key returns the API key; it runs in the session's goroutine, so it may
	// read the Keychain.
	Key func(context.Context) (string, error)
	// OnEvent receives the events in order, from the session's goroutine,
	// never after Close.
	OnEvent func(Event)
}

// chunk is 100 ms of 16 kHz 16-bit mono PCM, the packet size the realtime
// models expect.
const chunk = 3200

// queued bounds the audio waiting to be sent: 30 s of chunks.
const queued = 300

// Session is one online recognition task. PCM, Finish and Close must be
// called from one goroutine, the app's main thread.
type Session struct {
	opts     Options
	ctx      context.Context
	cancel   context.CancelFunc
	out      chan []byte
	pending  []byte
	finished bool
	closed   atomic.Bool
	timedOut atomic.Bool
	failure  error
	failOnce sync.Once
}

// finishMarker in the send queue ends the audio.
var finishMarker = []byte{}

// Start connects in the background; events report how it goes.
func Start(ctx context.Context, o Options) *Session {
	ctx, cancel := context.WithCancel(ctx)
	s := &Session{opts: o, ctx: ctx, cancel: cancel, out: make(chan []byte, queued+1)}
	go s.run()
	return s
}

// PCM queues audio; a full queue fails the session rather than block.
func (s *Session) PCM(b []byte) {
	if s.finished {
		return
	}
	s.pending = append(s.pending, b...)
	for len(s.pending) >= chunk {
		s.enqueue(append([]byte(nil), s.pending[:chunk]...))
		s.pending = s.pending[chunk:]
	}
}

// Finish sends the remaining audio and asks for the final transcript.
func (s *Session) Finish() {
	if s.finished {
		return
	}
	if len(s.pending) > 0 {
		s.enqueue(s.pending)
		s.pending = nil
	}
	s.finished = true
	s.enqueue(finishMarker)
}

// Close abandons the session; no event follows.
func (s *Session) Close() {
	s.closed.Store(true)
	s.cancel()
}

func (s *Session) enqueue(b []byte) {
	select {
	case s.out <- b:
	default:
		s.fail(errors.New("asr: send queue full"))
	}
}

// fail ends the session with err, the first failure winning.
func (s *Session) fail(err error) {
	s.failOnce.Do(func() { s.failure = err })
	s.cancel()
}

func (s *Session) emit(e Event) {
	if !s.closed.Load() && s.opts.OnEvent != nil {
		s.opts.OnEvent(e)
	}
}

func (s *Session) run() {
	defer s.cancel()
	text, err := s.recognize()
	if err == nil {
		s.emit(Event{Type: "final", Text: text})
		return
	}
	s.failOnce.Do(func() { s.failure = err })
	if s.timedOut.Load() {
		s.failure = context.DeadlineExceeded
	}
	s.emit(Event{Type: "error", Err: s.failure})
}

func (s *Session) recognize() (string, error) {
	c := s.opts.Config
	if err := c.Validate(); err != nil {
		return "", err
	}
	timer := time.AfterFunc(StartTimeout, func() { s.timedOut.Store(true); s.cancel() })
	defer timer.Stop()
	key, err := s.opts.Key(s.ctx)
	if err != nil || key == "" {
		if s.ctx.Err() != nil {
			return "", s.ctx.Err()
		}
		return "", ErrKey
	}
	if c.Engine == EngineOmni {
		return s.recognizeOmni(key, timer)
	}
	if c.Engine == EngineDoubao {
		return s.recognizeDoubao(key, timer)
	}
	if !c.Streaming() {
		timer.Stop()
		return s.recognizeWhole(key)
	}
	ws, err := dial(s.ctx, endpoint+"?model="+url.QueryEscape(c.Model), bearer(key))
	if err != nil {
		return "", err
	}
	go func() { <-s.ctx.Done(); ws.Close() }()
	task := newTaskID()
	if err := ws.WriteText(runTask(task, c.Model)); err != nil {
		return "", err
	}
	var transcript Transcript
	partial := ""
	for {
		op, b, err := ws.ReadMessage()
		if err != nil {
			if s.ctx.Err() != nil {
				return "", s.ctx.Err()
			}
			return "", err
		}
		if op != opText {
			continue
		}
		e, err := parseEvent(b)
		if err != nil {
			return "", err
		}
		switch e.Event {
		case "task-started":
			if !timer.Stop() {
				return "", context.DeadlineExceeded
			}
			go s.send(ws, task)
			s.emit(Event{Type: "ready"})
		case "result-generated":
			transcript.Add(*e.Sentence)
			if t := transcript.Text(); t != partial {
				partial = t
				s.emit(Event{Type: "partial", Text: t})
			}
		case "task-finished":
			return transcript.Text(), nil
		case "task-failed":
			return "", &TaskError{Code: e.Code, Message: e.Message}
		}
	}
}

// send writes the queued audio, then finish-task.
func (s *Session) send(ws *conn, task string) {
	for {
		select {
		case <-s.ctx.Done():
			return
		case b := <-s.out:
			var err error
			if len(b) == 0 {
				err = ws.WriteText(finishTask(task))
			} else {
				err = ws.WriteBinary(b)
			}
			if err != nil {
				s.fail(err)
				return
			}
			if len(b) == 0 {
				return
			}
		}
	}
}

// TestResult is how a connection test ended.
type TestResult struct{ Elapsed time.Duration }

// Test runs a session of half a second of silence with key, checking the
// key, the model and the network without sending any speech.
func Test(ctx context.Context, c Config, key string) (TestResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	started := time.Now()
	done := make(chan error, 1)
	var s *Session
	ready := make(chan struct{}, 1)
	s = Start(ctx, Options{
		Config: c,
		Key:    func(context.Context) (string, error) { return key, nil },
		OnEvent: func(e Event) {
			switch e.Type {
			case "ready":
				ready <- struct{}{}
			case "final":
				done <- nil
			case "error":
				done <- e.Err
			}
		},
	})
	defer s.Close()
	select {
	case <-ready:
		s.PCM(make([]byte, chunk*5))
		s.Finish()
	case err := <-done:
		return TestResult{}, err
	case <-ctx.Done():
		return TestResult{}, ctx.Err()
	}
	select {
	case err := <-done:
		return TestResult{Elapsed: time.Since(started)}, err
	case <-ctx.Done():
		return TestResult{}, ctx.Err()
	}
}
