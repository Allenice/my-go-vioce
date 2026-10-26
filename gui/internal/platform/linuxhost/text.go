package linuxhost

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/vfasky/my-ai-audio/gui/internal/platform/bridge"
)

// A runner never invokes a shell. Text is sent to a clipboard tool over stdin,
// never as a process argument, shell expansion, or a temporary file.
type runner func(context.Context, string, []string, string, bool) (string, error)

func run(ctx context.Context, name string, args []string, input string, capture bool) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = strings.NewReader(input)
	cmd.WaitDelay = 250 * time.Millisecond
	var out bytes.Buffer
	if capture {
		cmd.Stdout = &out
	}
	err := cmd.Run()
	if err != nil {
		return "", errors.New("desktop command unavailable")
	}
	return strings.TrimSpace(out.String()), nil
}

type textSession struct {
	mu      sync.Mutex
	id      uint32
	ctx     context.Context
	cancel  context.CancelFunc
	target  string
	run     runner
	wayland bool
	out     emitter
	workers sync.WaitGroup
}

func RunText(parent context.Context, in io.Reader, out io.Writer) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	input := commands(ctx, cancel, in)
	t := &textSession{run: run, wayland: os.Getenv("WAYLAND_DISPLAY") != "" || os.Getenv("XDG_SESSION_TYPE") == "wayland", out: emitter{out: out}}
	defer func() { t.stop(0); t.workers.Wait() }()
	t.out.send(bridge.Event{Type: "ready"})
	for {
		select {
		case <-ctx.Done():
			return nil
		case c := <-input:
			switch c.Type {
			case "shutdown":
				return nil
			case "permissions", "request-permission":
				// Speech/Automation are not Linux permissions. Speech here reports
				// only that the app may use its explicitly selected online engine.
				t.out.permission("speech", "granted")
				t.out.permission("automation", "granted")
				tool := "xclip"
				if t.wayland {
					tool = "wl-copy"
				}
				state := "granted"
				if _, err := exec.LookPath(tool); err != nil {
					state = "denied"
				}
				t.out.permission("accessibility", state)
			case "warmup":
				t.out.send(bridge.Event{Type: "model", Message: "Linux 文字输出已就绪"})
			case "begin":
				t.begin(ctx, c.SessionID)
			case "cancel":
				t.stop(c.SessionID)
			case "insert":
				t.insert(c.SessionID, c.Text)
			case "refine":
				t.out.send(bridge.Event{Type: "refined", SessionID: c.SessionID})
			case "start":
				t.out.send(bridge.Event{Type: "error", SessionID: c.SessionID, Message: "Linux 不支持 Apple 本地识别，请选择在线引擎"})
			case "enter", "delete", "clear":
				if t.wayland {
					continue
				}
				keys := map[string][]string{"enter": {"Return"}, "delete": {"BackSpace"}, "clear": {"ctrl+a", "BackSpace"}}[c.Type]
				_, _ = t.run(ctx, "xdotool", append([]string{"key", "--clearmodifiers"}, keys...), "", false)
			}
		}
	}
}
func (t *textSession) begin(parent context.Context, id uint32) {
	t.stop(0)
	ctx, cancel := context.WithCancel(parent)
	t.mu.Lock()
	t.ctx = ctx
	t.cancel = cancel
	t.id = id
	t.target = ""
	t.mu.Unlock()
	if !t.wayland {
		target, _ := t.run(ctx, "xdotool", []string{"getactivewindow"}, "", true)
		if _, err := strconv.ParseUint(target, 10, 64); err == nil {
			t.mu.Lock()
			if t.id == id {
				t.target = target
			}
			t.mu.Unlock()
		}
	}
}
func (t *textSession) stop(id uint32) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if id != 0 && id != t.id {
		return
	}
	if t.cancel != nil {
		t.cancel()
	}
	t.id = 0
	t.cancel = nil
	t.target = ""
}
func (t *textSession) insert(id uint32, text string) {
	t.mu.Lock()
	if id == 0 || id != t.id || t.ctx == nil {
		t.mu.Unlock()
		return
	}
	ctx, target := t.ctx, t.target
	t.mu.Unlock()
	if text == "" || len(text) > 12000 || !utf8.ValidString(text) || strings.ContainsRune(text, 0) {
		t.out.send(bridge.Event{Type: "error", SessionID: id})
		return
	}
	t.workers.Add(1)
	go func() {
		defer t.workers.Done()
		tool, args := "xclip", []string{"-selection", "clipboard", "-in"}
		if t.wayland {
			tool = "wl-copy"
			args = []string{"--type", "text/plain;charset=utf-8"}
		}
		if ctx.Err() != nil {
			return
		}
		_, err := t.run(ctx, tool, args, text, false)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			t.out.send(bridge.Event{Type: "error", SessionID: id, Message: "无法写入剪贴板，请安装剪贴板工具"})
			return
		}
		outcome := "已复制到剪贴板，请在目标应用中粘贴"
		if !t.wayland && target != "" {
			current, err := t.run(ctx, "xdotool", []string{"getactivewindow"}, "", true)
			// Do not activate a window or paste into a new app the user switched
			// to while dictating. The clipboard remains available either way.
			if err == nil && current == target && ctx.Err() == nil {
				if _, err = t.run(ctx, "xdotool", []string{"key", "--window", target, "--clearmodifiers", "ctrl+v"}, "", false); err == nil {
					outcome = "已粘贴；文字保留在剪贴板"
				}
			}
		}
		t.mu.Lock()
		defer t.mu.Unlock()
		if t.id != id || ctx.Err() != nil {
			return
		}
		t.out.send(bridge.Event{Type: "inserted", SessionID: id, Text: outcome})
		t.cancel()
		t.id = 0
		t.cancel = nil
	}()
}
