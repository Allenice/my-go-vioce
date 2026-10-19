// Package device reduces transport and speech events; effects are owned by app.
package device

import (
	"github.com/vfasky/my-ai-audio/gui/internal/protocol"
	"time"
)

type Effect struct {
	Type, State, Text string
	ID                uint32
	Sequence          uint32
	PCM               []byte
}
type Coordinator struct {
	Ready                              bool
	Active                             bool
	Generation                         uint32
	Stream                             *protocol.Stream
	Phase                              string
	Partial, Original, Refined, Status string
	Meter                              protocol.Meter
	accepted                           bool
	started, processing                time.Time
	buffer                             []protocol.PCM
	seen                               map[uint32]bool
	order                              []uint32
	buttons                            map[uint32]bool
	buttonOrder                        []uint32
	wire                               uint32
}

func (c *Coordinator) state(state string) Effect { return Effect{Type: "host_state", State: state} }
func (c *Coordinator) Cancel(reason string) []Effect {
	if !c.Active {
		return nil
	}
	id := c.Generation
	c.Active = false
	c.Phase = "ready"
	c.Status = reason
	c.Stream = nil
	c.buffer = nil
	c.accepted = false
	return []Effect{{Type: "cancel", ID: id}, c.state("error"), c.state("ready")}
}

// CanCommit reports whether Commit would insert now: while refinement runs,
// the final ASR original is already available.
func (c *Coordinator) CanCommit() bool {
	return c.Active && c.Phase == "refining" && c.Original != ""
}

// Commit skips refinement and inserts the final ASR original. A late
// refinement_done is then ignored, as the phase is no longer refining.
func (c *Coordinator) Commit() []Effect {
	if !c.CanCommit() {
		return nil
	}
	c.Refined = c.Original
	c.Phase = "inserting"
	return []Effect{{Type: "insert", ID: c.Generation, Text: c.Original}}
}
func (c *Coordinator) Disconnect() []Effect {
	effects := c.Cancel("Disconnected; transcript not inserted")
	c.Ready = false
	c.seen = nil
	c.order = nil
	c.buttons = nil
	c.buttonOrder = nil
	c.Phase = "disconnected"
	return effects
}
func (c *Coordinator) Control(v protocol.Control, now time.Time) []Effect {
	if v.Type == "hello" {
		c.Ready = true
		return []Effect{{Type: "host_ready"}}
	}
	if !c.Ready {
		return nil
	}
	if v.Type == "button_event" {
		if c.buttons == nil {
			c.buttons = map[uint32]bool{}
		}
		if c.buttons[v.Seq] {
			return nil
		}
		c.buttons[v.Seq] = true
		c.buttonOrder = append(c.buttonOrder, v.Seq)
		if len(c.buttonOrder) > 128 {
			delete(c.buttons, c.buttonOrder[0])
			c.buttonOrder = c.buttonOrder[1:]
		}
	}
	switch v.Type {
	case "ptt_down":
		if c.Active {
			return nil
		}
		if c.seen == nil {
			c.seen = map[uint32]bool{}
		}
		if c.seen[v.SessionID] {
			return nil
		}
		c.seen[v.SessionID] = true
		c.order = append(c.order, v.SessionID)
		if len(c.order) > 128 {
			delete(c.seen, c.order[0])
			c.order = c.order[1:]
		}
		c.Generation++
		c.Active = true
		c.wire = v.SessionID
		c.Stream = protocol.NewStream(v.SessionID, now)
		c.Phase = "starting"
		c.started = now
		c.processing = time.Time{}
		c.Status = ""
		c.accepted = false
		c.buffer = nil
		c.Partial = ""
		c.Original = ""
		c.Refined = ""
		c.Meter = protocol.Meter{}
		return []Effect{{Type: "start", ID: c.Generation}, c.state("recording")}
	case "ptt_up":
		if !c.Active || c.Stream == nil || v.SessionID != c.wire {
			return nil
		}
		frames, e := c.Stream.Stop(v.FinalSequence, now)
		if e != nil {
			return c.Cancel(e.Error())
		}
		c.Phase = "tail"
		return c.consume(frames, now)
	case "device_error":
		if c.Active && v.SessionID == c.wire {
			return c.Cancel("Device error")
		}
	case "button_event":
		if v.Button == "down" && v.Event == "click" {
			return []Effect{{Type: "enter"}}
		}
		if v.Button == "ok" {
			if c.Active {
				return c.Cancel("Cancelled")
			}
			if v.Event == "long" && v.DurationMS >= 650 {
				return []Effect{{Type: "clear"}}
			}
			if v.Event == "click" {
				return []Effect{{Type: "delete"}}
			}
		}
	}
	return nil
}
func (c *Coordinator) Audio(f protocol.Frame, now time.Time) []Effect {
	if !c.Active || c.Stream == nil {
		return nil
	}
	frames, e := c.Stream.Push(f, now)
	if e != nil {
		return c.Cancel(e.Error())
	}
	return c.consume(frames, now)
}
func (c *Coordinator) Tick(now time.Time) []Effect {
	if !c.Active {
		return nil
	}
	if !c.accepted && now.Sub(c.started) >= 1200*time.Millisecond {
		return c.Cancel("Speech start timeout")
	}
	if !c.processing.IsZero() && now.Sub(c.processing) >= 60*time.Second {
		return c.Cancel("Processing timeout")
	}
	if c.Stream == nil {
		return nil
	}
	frames, e := c.Stream.Tick(now)
	if e != nil {
		return c.Cancel(e.Error())
	}
	return c.consume(frames, now)
}
func (c *Coordinator) consume(frames []protocol.PCM, now time.Time) []Effect {
	if c.Stream != nil {
		c.Meter = c.Stream.Meter
	}
	if !c.accepted {
		c.buffer = append(c.buffer, frames...)
		if len(c.buffer) > 75 {
			return c.Cancel("Early audio buffer overflow")
		}
		return nil
	}
	var out []Effect
	for _, p := range frames {
		out = append(out, Effect{Type: "pcm", ID: c.Generation, Sequence: p.Sequence, PCM: protocol.PCMBytes(p.Samples)})
	}
	if c.Stream != nil && c.Stream.Done {
		c.Stream = nil
		c.Phase = "finalizing"
		c.processing = now
		out = append(out, Effect{Type: "finish", ID: c.Generation}, c.state("processing"))
	}
	return out
}
func (c *Coordinator) Speech(kind string, id uint32, text string, now time.Time) []Effect {
	if !c.Active || id != c.Generation {
		return nil
	}
	switch kind {
	case "ready":
		if c.accepted {
			return nil
		}
		c.accepted = true
		c.processing = time.Time{}
		c.Phase = "recording"
		frames := c.buffer
		c.buffer = nil
		return c.consume(frames, now)
	case "partial":
		c.Partial = text
	case "final":
		if c.Phase != "finalizing" {
			return nil
		}
		if text == "" {
			return c.Cancel("Empty transcription")
		}
		c.Original = text
		c.Phase = "refining"
		return []Effect{{Type: "refine", ID: id, Text: text}}
	case "refinement_done":
		if c.Phase != "refining" {
			return nil
		}
		c.Refined = text
		c.Phase = "inserting"
		return []Effect{{Type: "insert", ID: id, Text: text}}
	case "inserted":
		if c.Phase != "inserting" {
			return nil
		}
		c.Active = false
		c.Phase = "ready"
		c.Status = text
		return []Effect{c.state("success"), c.state("ready")}
	case "error", "cancelled":
		return c.Cancel("Speech cancelled or unavailable")
	}
	return nil
}
