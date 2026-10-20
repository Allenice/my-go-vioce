package asr

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
)

// The duplex task protocol of the realtime models: run-task, binary PCM,
// finish-task; the server answers task-started, result-generated events and
// task-finished or task-failed.

func newTaskID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func runTask(id, model string) []byte {
	b, _ := json.Marshal(map[string]any{
		"header": map[string]any{"action": "run-task", "task_id": id, "streaming": "duplex"},
		"payload": map[string]any{
			"task_group": "audio", "task": "asr", "function": "recognition", "model": model,
			"parameters": map[string]any{"format": "pcm", "sample_rate": 16000},
			"input":      map[string]any{},
		},
	})
	return b
}

func finishTask(id string) []byte {
	b, _ := json.Marshal(map[string]any{
		"header":  map[string]any{"action": "finish-task", "task_id": id, "streaming": "duplex"},
		"payload": map[string]any{"input": map[string]any{}},
	})
	return b
}

// Sentence is one sentence of a result-generated event: the server resends
// the whole sentence as it grows, and marks its end.
type Sentence struct {
	ID   int    `json:"sentence_id"`
	Text string `json:"text"`
	End  bool   `json:"sentence_end"`
}

// serverEvent is one text message of the server.
type serverEvent struct {
	Event    string
	Code     string
	Message  string
	Sentence *Sentence
}

func parseEvent(b []byte) (serverEvent, error) {
	var m struct {
		Header struct {
			Event        string `json:"event"`
			ErrorCode    string `json:"error_code"`
			ErrorMessage string `json:"error_message"`
		} `json:"header"`
		Payload struct {
			Output struct {
				Sentence *Sentence `json:"sentence"`
			} `json:"output"`
		} `json:"payload"`
	}
	if json.Unmarshal(b, &m) != nil || m.Header.Event == "" {
		return serverEvent{}, ErrProtocol
	}
	e := serverEvent{Event: m.Header.Event, Code: m.Header.ErrorCode, Message: m.Header.ErrorMessage}
	if e.Event == "result-generated" {
		if m.Payload.Output.Sentence == nil {
			return serverEvent{}, ErrProtocol
		}
		e.Sentence = m.Payload.Output.Sentence
	}
	return e, nil
}

// Transcript joins the sentences of a session in order.
type Transcript struct {
	sentences map[int]string
}

// Add records the latest text of a sentence.
func (t *Transcript) Add(s Sentence) {
	if t.sentences == nil {
		t.sentences = map[int]string{}
	}
	t.sentences[s.ID] = strings.TrimSpace(s.Text)
}

// Text is every sentence so far, the last one possibly still growing.
func (t *Transcript) Text() string {
	ids := make([]int, 0, len(t.sentences))
	for id := range t.sentences {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	var b strings.Builder
	last := rune(0)
	for _, id := range ids {
		s := t.sentences[id]
		if s == "" {
			continue
		}
		// English sentences keep a space between them; Chinese ones do not.
		if first := []rune(s)[0]; last != 0 && last < 0x80 && first < 0x80 && first != ' ' {
			b.WriteByte(' ')
		}
		b.WriteString(s)
		r := []rune(s)
		last = r[len(r)-1]
	}
	return b.String()
}
