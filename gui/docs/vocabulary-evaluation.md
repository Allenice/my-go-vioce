[简体中文](vocabulary-evaluation.zh_CN.md) · English

# Vocabulary retrieval and evaluation

## Runtime boundary

Apple recognition keeps its ranked Top 100 hints. The session separately freezes all explicit terms and unique replacement targets (up to 2,000 of each). Wrong spellings are never indexed as terms. After local explicit correction, a worker builds a pinyin index and retrieves terms for the complete transcript. The index groups by first folded syllable and character count, then verifies every character reading; polyphonic readings do not expand into a Cartesian product.

All personal competitors for a matching span remain available. Literal personal matches protect correctly spelled longer terms. The non-Chinese Top 100 hints remain available to refinement for transliteration, case and spacing; local pinyin retrieval does not solve those cases. Other non-Chinese terms can be retrieved by literal or case/whitespace-normalized matches. Jev still receives only matching Chinese candidates, full transcript and segmentation, never the full vocabulary.

More than 256 selected personal terms, 4,096 personal span/term matches or 8,000 transcript characters declines both model steps and retains explicit local correction. Refinement also declines a prompt over 128 KiB. These limits never select a convenient subset of competing terms. Existing Jev limits (24 groups, 63 alternatives per group and 128 KiB) remain in effect. The original remains an option, and uncertainty is not permission to replace it.

## Bundled public packs

The Common and IT packs default to enabled, including for settings created before this feature. Vocabulary > Public Dictionaries shows their counts and saves switches for the next session. They never enter personal import/export or recognition hints. With both refinement and Jev off, public packs do not edit text; explicit personal replacements still work.

| Pack | Pinned source | Selection | Terms |
| --- | --- | --- | --- |
| Common | Jieba dictionary shipped with gojieba v1.4.7 | 2–8 Han characters, frequency >= 10, available readings | 96,336 |
| IT | THUOCL `a30ce79d895d01ab5132a5c74c29703ff7efb4cc` | 2–8 Han characters, positive document frequency, available readings | 12,854 |

The union contains 108,623 distinct terms. Two embedded compressed files total 1,228,204 bytes. The [manifest](../internal/wordlists/data/manifest.json) records source and output SHA-256 digests; [third-party notices](THIRD_PARTY_NOTICES.md) retain MIT attribution. No other industry packs or neural weights are included.

Character readings are compiled with the pinned go-pinyin version, without tones. These remain character readings, not phrase-level disambiguation. The first worker use validates and inflates the embedded files and builds an immutable in-memory index once per process. Subsequent dictations reuse it, without network access or dependence on a developer's filesystem. If public loading or segmentation fails, the session reports it and continues with personal evidence.

Public retrieval requires complete token boundaries, identical tone-free readings and no overlap with a correctly spelled personal term. A public entry does not protect the ASR original: an ordinary word can still yield to a personal name. Lower/equal-frequency alternatives are suppressed only when the original exists in the same source; raw Jieba frequencies and THUOCL document frequencies are never added or compared across sources. IT suggestions precede Common suggestions, and within each source the ranking is log-normalized frequency. No frequency is treated as a probability.

Public retrieval keeps at most eight alternatives per span and 96 per dictation. Personal candidates have priority in the prompt and question budget. If public spans make a mixed Jev group too large, that group's public layer is removed and personal occurrences are regrouped rather than discarded. Public candidates retain `source` and `rank` in prompts, separately from personal `vocabulary`. The original remains a choice. After Jev changes an occurrence, its obsolete public candidate is removed before refinement.

Tests cover IT recall without personal terms, ordinary-word preservation through a mocked judge, source metadata, personal protection, independent switches, failed saves, session snapshots and source reproducibility. One Apple M5 run of `BenchmarkPublicVocabulary` averaged 0.261 ms per warm lookup for the fixed short fixture, with segmentation and cold loading excluded. This is not a production P95 or measured model accuracy.

Rebuild the packs from `gui/`, using the pinned inputs and a temporary download directory:

```bash
wordlist_tmp="$(mktemp -d)"
curl --fail --location https://raw.githubusercontent.com/thunlp/THUOCL/a30ce79d895d01ab5132a5c74c29703ff7efb4cc/data/THUOCL_IT.txt -o "$wordlist_tmp/THUOCL_IT.txt"
go run ./tools/build-wordlists -jieba "$(go list -m -f '{{.Dir}}' github.com/yanyiwu/gojieba)/deps/cppjieba/dict/jieba.dict.utf8" -it "$wordlist_tmp/THUOCL_IT.txt"
go test ./internal/wordlists ./internal/refinement ./internal/jev ./internal/app ./internal/ui
```

The compiler refuses unexpected input hashes and emits deterministic compressed files. Update source pins, hashes, counts, notices and tests together when intentionally refreshing a source. THUOCL is an older general IT corpus; English/mixed terms and newer AI names are not part of this filtered Han pack and remain personal vocabulary work.

## Offline regression seeds

[`vocabulary-cases.json`](../internal/refinement/testdata/vocabulary-cases.json) contains 40 authored cases: 20 development and 20 evaluation cases, with corrections and ordinary-word preservation equally represented. Tags cover products, places, polyphonic characters, near-homophones, overlapping names, repeated occurrences, known-word protection, punctuation and mixed text. These are synthetic regression seeds, not recorded speech, not an estimate of production accuracy, and not a substitute for a larger independently labelled corpus.

The local test deliberately puts relevant entries after the first 100 in a 2,000-term vocabulary. It compares full-vocabulary retrieval with Top-100-only retrieval, checks required terms and verifies that retrieval preserves the candidate set of a full search. A separate exhaustive-search oracle checks indexing equivalence.

From `gui/`:

```bash
GOVOICE_JEV_LIVE=0 go test ./internal/vocabulary ./internal/refinement ./internal/jev ./internal/app
GOVOICE_JEV_LIVE=0 go test ./internal/refinement -run '^TestVocabularyCorpusRecall$' -v
go test ./internal/refinement -run '^$' -bench '^BenchmarkVocabularyRetrieval$' -benchmem
```

On the authored overflow-of-Top-100 fixture, required-term recall is 43/43 versus 0/43. That demonstrates the retrieval limit being removed; it says nothing about a model choosing the correct term.

One Apple M5 / darwin-arm64 run, roughly 90 transcript characters, Go 1.27.1, `-benchtime=200ms`:

| Terms | Build index and retrieve, mean | Retrieve with an existing index, mean |
| --- | --- | --- |
| 100 | 0.132 ms | 0.033 ms |
| 2,000 | 2.106 ms | 0.190 ms |
| 4,000 | 4.220 ms | 0.351 ms |

The personal-vocabulary worker currently builds once per completed dictation; the public index is cached separately. These are local retrieval averages, not end-to-end latency, P95, or a guarantee for other texts/hardware. Segmentation, Jev and refinement have separate costs.

## Opt-in model evaluation

`TestLiveVocabularyCorpus` is skipped by normal validation. With explicit intent to call a paid provider, supply its API key through the environment and run:

```bash
GOVOICE_JEV_LIVE=1 GOVOICE_JEV_SPLIT=eval go test ./internal/jev -run '^TestLiveVocabularyCorpus$' -count=1 -v
```

`TYPESAFE_API_KEY` supplies the key; `GOVOICE_JEV_ENDPOINT` and `GOVOICE_JEV_MODEL` select another compatible provider. No credentials belong in source, fixtures or reports. Set `GOVOICE_JEV_SPLIT=dev` for tuning. `GOVOICE_JEV_PACKS=all` (default), `off`, `base` or `it` selects the public layer for paired comparisons. Keep evaluation separate from threshold tuning and report the exact endpoint/model configuration without credentials.

The test uses the production retrieval and Jev paths and reports recovered, broken, missed, unchanged, incorrect edits, request failures and P95 across case elapsed times. Cases without candidates are included in that latency and make no network request. An incorrect edit or provider error fails the test; abstaining on a wrong input is counted as missed. This measures Jev only, not later generative refinement or ASR. The fixed 70% threshold has not been recalibrated.

Future model comparisons should share independently labelled inputs and candidate sets, compare decisions case by case, report corrections and regressions separately, and include ambiguous examples whose expected action is preservation. First expand to 300–500 diverse cases for regression coverage; small model-quality differences need substantially larger evaluation sets. No real user recordings or transcripts are collected automatically.

## Deferred model work

The design draws on the evaluation and gating lessons in [chinese-ime-lm](https://github.com/metasequoiaime/chinese-ime-lm/tree/4e745be72f1c49e9c7b5dadea819fbb1fcda0e13); no upstream code, corpus or weights were imported. Phrase-level readings, domain descriptions, calibrated near-homophone penalties, confirmed-user-feedback ranking and local model comparisons remain separate work. A local Han language model needs ASR-specific evaluation, including suffix context, mixed text and rare user names, before entering the correction path.
