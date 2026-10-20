package jev

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/vfasky/my-ai-audio/gui/internal/refinement"
)

// liveCases are made-up dictations, each span marked whether the speaker
// said the term there. They tune the question and the thresholds.
var liveCases = []struct {
	text  string
	terms []string
	want  []bool
}{
	{TestText, TestTerms, []bool{true, false}},
	{"帮我打开施玉宝后台看一下今天的订单", []string{"私域宝"}, []bool{true}},
	{"我一直在等你回消息", []string{"伊智"}, []bool{false}},
	{"把这个客户加到七维里，然后发个欢迎语", []string{"企微"}, []bool{true}},
	{"这款车的气味有点大，开窗通通风", []string{"企微"}, []bool{false}},
	{"今天的课情日历里有三条回访任务", []string{"客情"}, []bool{true}},
	{"这件事可轻可重，你自己看着办", []string{"客情"}, []bool{false}},
	{"明天去一致开会，记得带电脑", []string{"伊智"}, []bool{true}},
	{"两边的数据完全一致，没有问题", []string{"伊智"}, []bool{false}},
	{"用私欲宝给老客户发优惠券", []string{"私域宝"}, []bool{true}},
}

// TestLiveJudgments asks the real Jev about liveCases, only with
// GOVOICE_JEV_LIVE=1 and TYPESAFE_API_KEY set: each run is billed.
// GOVOICE_JEV_ENDPOINT and GOVOICE_JEV_MODEL choose another provider. It
// fails when a threshold would act wrongly; an uncertain span is only
// logged.
func TestLiveJudgments(t *testing.T) {
	key := os.Getenv("TYPESAFE_API_KEY")
	if os.Getenv("GOVOICE_JEV_LIVE") != "1" || key == "" {
		t.Skip("set GOVOICE_JEV_LIVE=1 and TYPESAFE_API_KEY to ask the real Jev")
	}
	for _, c := range liveCases {
		if n := len(refinement.Spans(c.text, c.terms)); n != len(c.want) {
			t.Fatalf("%s: %d spans, %d expectations", c.text, n, len(c.want))
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		res, err := Check(ctx, Config{Endpoint: os.Getenv("GOVOICE_JEV_ENDPOINT"), Model: os.Getenv("GOVOICE_JEV_MODEL")}, c.text, c.terms, key)
		cancel()
		if err != nil {
			t.Fatalf("%s: %v", c.text, err)
		}
		for i, d := range res.Decisions {
			action := "uncertain"
			switch {
			case d.P >= Replace:
				action = "replace"
			case d.OriginalP >= Replace:
				action = "keep"
			}
			t.Logf("p=%.2f %-9s want replace=%-5v %s → %s in %s", d.P, action, c.want[i], d.Heard, d.Term, c.text)
			if (action == "replace" && !c.want[i]) || (action == "keep" && c.want[i]) {
				t.Errorf("wrong %s of %s in %s (p=%.2f)", action, d.Heard, c.text, d.P)
			}
		}
	}
}
