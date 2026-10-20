// Package asr runs online speech recognition: Alipay MaaS realtime models
// over WebSocket, fed the session's 16 kHz PCM as it arrives.
package asr

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Engines: Apple SpeechAnalyzer in SpeechBridge, or Alipay MaaS online.
const (
	EngineLocal  = "local"
	EngineAlipay = "alipay"
)

// Config is the recognition engine of the settings; the zero value is the
// local engine.
type Config struct {
	Engine string `json:"engine,omitempty"`
	Model  string `json:"model,omitempty"`
}

// Online reports whether c sends audio to an online engine.
func (c Config) Online() bool { return c.Engine == EngineAlipay }

// Streaming reports whether c recognizes the audio while it arrives; the
// other online models recognize the whole recording once it ends.
func (c Config) Streaming() bool { return c.Online() && c.Model != modelFlash }

// modelFlash recognizes a whole recording in one HTTP request.
const modelFlash = "qwen3-asr-flash"

// Choice is one entry of the engine drop-down.
type Choice struct {
	ID, Label string
	Config    Config
}

// Choices are the engines and models the settings offer, local first.
var Choices = []Choice{
	{"local", "Apple 本地（SpeechAnalyzer）", Config{Engine: EngineLocal}},
	{"alipay/fun-asr-realtime", "支付宝 MaaS · Fun-ASR 实时", Config{Engine: EngineAlipay, Model: "fun-asr-realtime"}},
	{"alipay/paraformer-realtime-v2", "支付宝 MaaS · Paraformer 实时 v2", Config{Engine: EngineAlipay, Model: "paraformer-realtime-v2"}},
	{"alipay/" + modelFlash, "支付宝 MaaS · Qwen3-ASR-Flash（非流式）", Config{Engine: EngineAlipay, Model: modelFlash}},
}

// Normalize maps settings saved by an older build, or edited by hand, to a
// choice: unknown engines and models become the local engine.
func Normalize(c Config) Config {
	for _, ch := range Choices {
		if ch.Config == c {
			return c
		}
	}
	return Config{Engine: EngineLocal}
}

// ChoiceOf is the drop-down entry of c.
func ChoiceOf(c Config) Choice {
	c = Normalize(c)
	for _, ch := range Choices {
		if ch.Config == c {
			return ch
		}
	}
	return Choices[0]
}

// KeyAccount is the Keychain account of the Alipay MaaS API key; it cannot
// collide with the endpoints that key refinement.
const KeyAccount = "asr:alipay-maas"

// Host receives the audio of online sessions.
const Host = "maas-api.antdigital.com"

// endpoint is the DashScope-style duplex inference socket, and
// flashEndpoint the multimodal generation API of the whole-recording model;
// tests replace them.
var (
	endpoint      = "wss://" + Host + "/api-ws/v1/inference"
	flashEndpoint = "https://" + Host + "/api/v1/services/aigc/multimodal-generation/generation"
)

// StartTimeout bounds connecting and the task start: the coordinator buffers
// the audio meanwhile.
const StartTimeout = 4 * time.Second

// Why a session failed, for Message to explain.
var (
	ErrKey      = errors.New("asr: key unavailable")
	ErrConfig   = errors.New("asr: not an online engine")
	ErrProtocol = errors.New("asr: invalid reply")
)

// StatusError is a refused WebSocket upgrade or HTTP request.
type StatusError struct {
	Code int
	// Reason is the error code of the reply body, when it had one.
	Reason string
}

func (e *StatusError) Error() string { return fmt.Sprintf("asr: HTTP %d %s", e.Code, e.Reason) }

// TaskError is a task-failed event.
type TaskError struct{ Code, Message string }

func (e *TaskError) Error() string { return "asr: task failed: " + e.Code + " " + e.Message }

// Message tells the user, in Chinese, why online recognition failed.
func Message(err error) string {
	var status *StatusError
	var task *TaskError
	switch {
	case errors.Is(err, ErrKey):
		return "在线识别没有可用的 API key：请到“语音识别”页填写并保存。"
	case errors.Is(err, context.DeadlineExceeded):
		return "在线识别连接超时：请检查网络。"
	case errors.As(err, &status):
		switch {
		case status.Code == 401:
			return "在线识别 API key 无效（HTTP 401）。"
		case status.Code == 403:
			return "此 API key 无权使用该识别模型（HTTP 403）。"
		case status.Code == 429:
			return "在线识别请求过于频繁或额度不足（HTTP 429）。"
		case status.Code >= 500:
			return fmt.Sprintf("在线识别服务出错（HTTP %d），请稍后再试。", status.Code)
		}
		return fmt.Sprintf("在线识别被拒绝（HTTP %d）。", status.Code)
	case errors.As(err, &task):
		return "在线识别失败：" + clip(task.Code+" "+task.Message, 60)
	case errors.Is(err, ErrProtocol):
		return "在线识别返回了无法解析的内容。"
	}
	return "在线识别连接失败：请检查网络或代理设置。"
}

func clip(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
