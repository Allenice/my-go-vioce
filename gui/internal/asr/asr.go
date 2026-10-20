// Package asr runs online speech recognition over WebSocket or HTTPS,
// fed the session's 16 kHz PCM as it arrives.
package asr

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Engines: Apple SpeechAnalyzer, Alipay MaaS, or Alibaba Cloud Omni Realtime.
const (
	EngineLocal  = "local"
	EngineAlipay = "alipay"
	EngineOmni   = "aliyun-omni"
)

const DefaultOmniEndpoint = "wss://dashscope.aliyuncs.com/api-ws/v1/realtime"
const defaultOmniModel = "qwen3-omni-flash-realtime"

// Config is the recognition engine of the settings; the zero value is the
// local engine.
type Config struct {
	Engine   string `json:"engine,omitempty"`
	Model    string `json:"model,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
}

// Online reports whether c sends audio to an online engine.
func (c Config) Online() bool { return c.Engine == EngineAlipay || c.Engine == EngineOmni }

// Streaming reports whether c recognizes the audio while it arrives; the
// other online models recognize the whole recording once it ends.
func (c Config) Streaming() bool {
	return c.Engine == EngineOmni || (c.Engine == EngineAlipay && c.Model != modelFlash)
}

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
	{"aliyun-omni", "阿里云百炼 · Omni Realtime（WebSocket）", Config{Engine: EngineOmni, Model: defaultOmniModel}},
}

// Normalize fills Omni defaults and maps unknown preset engines/models to
// local. Custom Omni endpoints and model names are checked by Validate.
func Normalize(c Config) Config {
	if c.Engine == EngineOmni {
		c.Endpoint, c.Model = strings.TrimSpace(c.Endpoint), strings.TrimSpace(c.Model)
		if c.Endpoint == DefaultOmniEndpoint {
			c.Endpoint = ""
		}
		if c.Model == "" {
			c.Model = defaultOmniModel
		}
		return c
	}
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
		if ch.Config == c || (c.Engine == EngineOmni && ch.Config.Engine == EngineOmni) {
			ch.Config = c
			return ch
		}
	}
	return Choices[0]
}

// KeyAccount separates providers and custom Omni endpoints. Existing Alipay
// credentials keep their original account.
func KeyAccount(c Config) string {
	if c.Engine == EngineOmni {
		return "asr:aliyun-omni:" + c.ServerURL()
	}
	return "asr:alipay-maas"
}

func (c Config) Provider() string {
	if c.Engine == EngineOmni {
		return "阿里云百炼"
	}
	return "支付宝 MaaS"
}

func (c Config) ServerURL() string {
	if strings.TrimSpace(c.Endpoint) == "" {
		return DefaultOmniEndpoint
	}
	return strings.TrimSpace(c.Endpoint)
}

func (c Config) Host() string {
	if c.Engine == EngineOmni {
		u, err := url.Parse(c.ServerURL())
		if err == nil {
			return u.Host
		}
		return ""
	}
	return Host
}

// Validate refuses endpoints with credentials, queries or fragments before
// looking up a key. The model is passed separately in the connection URL.
func (c Config) Validate() error {
	if !c.Online() || Normalize(c) != c {
		return ErrConfig
	}
	if c.Engine == EngineOmni {
		u, err := url.Parse(c.ServerURL())
		if err != nil || u.Scheme != "wss" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" || len(c.ServerURL()) > 1024 {
			return ErrEndpoint
		}
		if len(c.Model) > 128 || strings.ContainsAny(c.Model, " \t\r\n/?#") {
			return ErrConfig
		}
	}
	return nil
}

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
	ErrConfig   = errors.New("asr: invalid engine or model")
	ErrEndpoint = errors.New("asr: invalid WebSocket endpoint")
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
	case errors.Is(err, ErrEndpoint):
		return "WebSocket 端点必须是 wss:// 地址，不能包含凭据、查询参数或片段；模型名请单独填写。"
	case errors.Is(err, ErrConfig):
		return "在线识别引擎或模型配置无效，请检查“语音识别”设置。"
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
