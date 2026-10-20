package jev

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/vfasky/my-ai-audio/gui/internal/refinement"
	"github.com/vfasky/my-ai-audio/gui/internal/wordlists"
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

// TestLiveVocabularyCorpus reports recovered and broken cases separately on
// the held-out authored seeds. It is opt-in and never records user dictations.
// GOVOICE_JEV_SPLIT=dev selects the tuning split; eval is the default.
func TestLiveVocabularyCorpus(t *testing.T) {
	key := os.Getenv("TYPESAFE_API_KEY")
	if os.Getenv("GOVOICE_JEV_LIVE") != "1" || key == "" {
		t.Skip("set GOVOICE_JEV_LIVE=1 and TYPESAFE_API_KEY to ask the real Jev")
	}
	split := os.Getenv("GOVOICE_JEV_SPLIT")
	if split == "" {
		split = "eval"
	}
	if split != "dev" && split != "eval" {
		t.Fatal("GOVOICE_JEV_SPLIT must be dev or eval")
	}
	b, err := os.ReadFile("../refinement/testdata/vocabulary-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		ID, Split, Text, Gold string
		Terms                 []string
	}
	if err := json.Unmarshal(b, &cases); err != nil {
		t.Fatal(err)
	}
	packs := wordlists.Config{}
	switch os.Getenv("GOVOICE_JEV_PACKS") {
	case "", "all":
	case "off":
		packs.BaseDisabled, packs.ITDisabled = true, true
	case "base":
		packs.ITDisabled = true
	case "it":
		packs.BaseDisabled = true
	default:
		t.Fatal("GOVOICE_JEV_PACKS must be all, off, base or it")
	}
	recovered, broken, missed, unchanged, failed, wrong := 0, 0, 0, 0, 0, 0
	var elapsed []time.Duration
	var fillers []string
	for i := 0; i < 120; i++ {
		fillers = append(fillers, "测试条目"+string(rune(0x4e00+i)))
	}
	for _, c := range cases {
		if c.Split != split {
			continue
		}
		started := time.Now()
		evidence, err := refinement.PrepareEvidence(context.Background(), c.Text, append(slices.Clone(fillers), c.Terms...), fillers[:100], packs)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), Timeout)
		res, err := CheckEvidence(ctx, Config{Endpoint: os.Getenv("GOVOICE_JEV_ENDPOINT"), Model: os.Getenv("GOVOICE_JEV_MODEL")}, c.Text, evidence, key)
		cancel()
		elapsed = append(elapsed, time.Since(started))
		if err != nil {
			failed++
			t.Errorf("%s: %v", c.ID, err)
			continue
		}
		switch {
		case c.Text == c.Gold && res.Text != c.Gold:
			broken++
		case c.Text == c.Gold:
			unchanged++
		case res.Text == c.Gold:
			recovered++
		default:
			missed++
		}
		if res.Text != c.Text && res.Text != c.Gold {
			wrong++
			t.Errorf("%s: incorrect edit", c.ID)
		}
	}
	slices.Sort(elapsed)
	if len(elapsed) == 0 {
		t.Fatal("empty evaluation split")
	}
	t.Logf("authored split=%s cases=%d recovered=%d broken=%d missed=%d unchanged=%d wrong_edits=%d errors=%d p95=%s", split, len(elapsed), recovered, broken, missed, unchanged, wrong, failed, elapsed[(95*len(elapsed)+99)/100-1])
}
