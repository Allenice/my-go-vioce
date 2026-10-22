package refinement

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/vfasky/my-ai-audio/gui/internal/wordlists"
)

// liveCases are made-up dictations with what the refined text must and must
// not contain. None of them is an example in the prompt: they check that
// structured mode fixes words recognition misheard, including real words
// that do not fit, and leaves the same words where they do fit.
var liveCases = []struct {
	text           string
	want, unwanted []string
}{
	// 豆瓣 for 豆包 is fixed only once 豆包 reaches the prompt as a vocabulary
	// term: alone, a fast model keeps a well-known name that sounds alike.
	{"增加豆瓣语音识别的介入。", []string{"接入"}, []string{"介入"}},
	{"把这个需求同步到非书文档里", []string{"飞书"}, []string{"非书"}},
	{"调用高的地图的路线规划接口", []string{"高德"}, []string{"高的"}},
	{"我在豆瓣上看到这部电影评分很高", []string{"豆瓣"}, []string{"豆包"}},
	{"监管部门已经介入调查了", []string{"介入"}, []string{"接入"}},
	{"这个交户逻辑有点问题我觉得应该先登路再跳转", []string{"交互", "登录"}, []string{"交户", "登路"}},
	{"小程序改完以后需要重新布署到测试环境", []string{"部署"}, []string{"布署"}},
	{"我们明天，不对，后天去上海开会", []string{"后天"}, []string{"明天"}},
	{"把今天的会议纪要上传到飞书文档", []string{"飞书文档", "会议纪要"}, nil},
}

// TestLiveStructuredCorrections refines liveCases with a real
// OpenAI-compatible model, only with GOVOICE_REFINE_LIVE=1 and
// GOVOICE_REFINE_KEY set: each run is billed. GOVOICE_REFINE_ENDPOINT,
// GOVOICE_REFINE_MODEL and GOVOICE_REFINE_API (chat or responses) choose
// the model, as on the settings page; GOVOICE_REFINE_MODE another mode, GOVOICE_REFINE_STREAM=1 a streamed request.
func TestLiveStructuredCorrections(t *testing.T) {
	key := os.Getenv("GOVOICE_REFINE_KEY")
	if os.Getenv("GOVOICE_REFINE_LIVE") != "1" || key == "" {
		t.Skip("set GOVOICE_REFINE_LIVE=1, GOVOICE_REFINE_KEY, GOVOICE_REFINE_ENDPOINT and GOVOICE_REFINE_MODEL to ask a real model")
	}
	c := Config{Engine: "remote", Mode: "structured", Endpoint: os.Getenv("GOVOICE_REFINE_ENDPOINT"), Model: os.Getenv("GOVOICE_REFINE_MODEL"), API: os.Getenv("GOVOICE_REFINE_API"), Stream: os.Getenv("GOVOICE_REFINE_STREAM") == "1"}
	if mode := os.Getenv("GOVOICE_REFINE_MODE"); mode != "" {
		c.Mode = mode
	}
	for _, lc := range liveCases {
		evidence, err := PrepareEvidence(context.Background(), lc.text, nil, nil, wordlists.Config{})
		if err != nil {
			t.Fatalf("%s: %v", lc.text, err)
		}
		out, status := RunEvidence(context.Background(), c, lc.text, evidence, fixedKey(key), nil)
		t.Logf("%-17s %s → %s", status, lc.text, out)
		if status != "refined" {
			t.Errorf("%s: %s", lc.text, status)
			continue
		}
		for _, w := range lc.want {
			if !strings.Contains(out, w) {
				t.Errorf("%s → %s: missing %s", lc.text, out, w)
			}
		}
		for _, w := range lc.unwanted {
			if strings.Contains(out, w) {
				t.Errorf("%s → %s: kept %s", lc.text, out, w)
			}
		}
	}
}
