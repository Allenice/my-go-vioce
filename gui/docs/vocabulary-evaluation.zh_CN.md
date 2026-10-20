[English](vocabulary-evaluation.md) · 简体中文

# 词库检索与评测

## 运行边界

Apple 识别继续使用排序后的 Top 100 提示词。会话另行固定所有显式词条及去重后的替换目标，两者各最多 2,000 条。错误写法不作为词条索引。完成本地显式纠正后，后台任务建立拼音索引，为完整转写检索相关词。索引先按折叠后的首音节和字数分组，再逐字核对读音；多音字不会展开成所有读音的笛卡尔积。

同一片段的个人竞争词都会保留。个人词条的字面命中保护已经正确写出的较长词条。Top 100 中的非纯中文提示词继续供润色处理音译、大小写和空格，本地拼音检索不解决这些问题；其他非纯中文词可通过字面、忽略大小写或空白标准化后的匹配召回。Jev 仍只接收命中的中文候选、完整转写和分词，不接收完整词库。

超过 256 个选中个人词、4,096 个片段与个人词条的匹配或 8,000 字转写时，跳过两种模型处理，保留本地显式纠正结果。润色提示超过 128 KiB 时也跳过。这些限制不会从竞争词中随意截取部分选项。Jev 原有的 24 组、每组 63 个替换选项和 128 KiB 限制继续有效。保留原文始终是一个选项，不确定不能作为替换依据。

## 内置公共词库

通用库和 IT 包默认开启，旧版本创建的设置也采用此默认值。“词库 > 公共词库”显示数量并保存开关，下次会话生效。公共词不进入个人导入导出及识别提示。AI 润色和 Jev 都关闭时，公共词库不修改文字；个人显式替换仍然生效。

| 词包 | 固定来源 | 筛选条件 | 词数 |
| --- | --- | --- | --- |
| 通用 | gojieba v1.4.7 随附的 Jieba 词典 | 2～8 个汉字、词频 >= 10、有读音 | 96,336 |
| IT | THUOCL `a30ce79d895d01ab5132a5c74c29703ff7efb4cc` | 2～8 个汉字、文档频率为正、有读音 | 12,854 |

合并去重后为 108,623 个词。两个内嵌压缩文件合计 1,228,204 字节。[清单](../internal/wordlists/data/manifest.json)记录源文件与生成文件的 SHA-256 摘要，[第三方声明](THIRD_PARTY_NOTICES.zh_CN.md)保留 MIT 署名。没有包含其他行业包或神经模型权重。

构建时使用固定版本的 go-pinyin 编译无声调读音。目前仍是单字读音，不是词级多音字消歧。首次后台使用时校验、解压内嵌数据并建立只读内存索引，每个进程只建立一次；后续转写复用索引，不联网、不依赖开发者文件路径。公共数据加载或分词失败时，会提示并继续使用个人词库依据。

公共检索要求完整词边界、无声调读音完全相同，且不覆盖已正确写出的个人词条。公共词不会保护 ASR 原文：普通词仍可让位给个人专名。只有原词存在于同一来源时，才排除该来源中频率不高于原词的候选；Jieba 词频与 THUOCL 文档频率不会跨来源相加或直接比较。IT 建议优先于通用建议，各来源内使用对数归一化词频排序，频率不作为概率。

每处最多召回八个公共候选，每次转写最多 96 个。个人候选在润色提示和问题名额中优先。若公共片段使某个 Jev 混合分组超限，会去掉该组的公共层并重新组织个人候选，而不丢弃个人候选。公共候选在提示中保留 `source` 与 `rank`，与个人 `vocabulary` 分离。保留原文始终是选项。Jev 修改某处后，进入润色前会移除该处已经失效的公共候选。

测试覆盖无个人词条时的 IT 召回、模拟判断器保留普通词、来源字段、个人词保护、独立开关、保存失败、会话快照和来源可复现性。一次 Apple M5 测量中，`BenchmarkPublicVocabulary` 固定短句的热检索均值为 0.261 ms，不含分词和冷加载。这不是生产 P95 或模型准确率。

在 `gui/` 中使用固定输入和临时下载目录重新生成词包：

```bash
wordlist_tmp="$(mktemp -d)"
curl --fail --location https://raw.githubusercontent.com/thunlp/THUOCL/a30ce79d895d01ab5132a5c74c29703ff7efb4cc/data/THUOCL_IT.txt -o "$wordlist_tmp/THUOCL_IT.txt"
go run ./tools/build-wordlists -jieba "$(go list -m -f '{{.Dir}}' github.com/yanyiwu/gojieba)/deps/cppjieba/dict/jieba.dict.utf8" -it "$wordlist_tmp/THUOCL_IT.txt"
go test ./internal/wordlists ./internal/refinement ./internal/jev ./internal/app ./internal/ui
```

编译工具拒绝不符合预期哈希的输入，压缩输出可确定性复现。主动更新来源时，应同步修改版本、哈希、数量、声明及测试。THUOCL 属于较早期的通用 IT 语料；英文、混合词及较新的 AI 名称不属于这份筛选后的中文词包，仍需个人词库补充。

## 离线回归样例

[`vocabulary-cases.json`](../internal/refinement/testdata/vocabulary-cases.json) 包含 40 条手写样例，开发集与评测集各 20 条，纠正和普通词保留样例各占一半。标签覆盖产品、地名、多音字、近音、重叠专名、重复出现、正确词保护、标点及中英混合。这些是人工构造的回归种子，不是真实录音，不代表生产准确率，也不能替代规模更大、独立标注的语料。

本地测试把相关词放在 2,000 词词库的第 100 条之后，对比完整词库检索和只查询 Top 100，检查所需词的召回，并验证筛选前后完整候选集合一致。另有穷举检索参照测试检查索引等价性。

在 `gui/` 中运行：

```bash
GOVOICE_JEV_LIVE=0 go test ./internal/vocabulary ./internal/refinement ./internal/jev ./internal/app
GOVOICE_JEV_LIVE=0 go test ./internal/refinement -run '^TestVocabularyCorpusRecall$' -v
go test ./internal/refinement -run '^$' -bench '^BenchmarkVocabularyRetrieval$' -benchmem
```

在刻意超出 Top 100 的手写样例中，所需词召回为 43/43，原 Top 100 查询为 0/43。这证明检索上限已解除，不代表模型能选对这些词。

一次 Apple M5 / darwin-arm64 测量，转写约 90 字、Go 1.27.1、`-benchtime=200ms`：

| 词数 | 建索引并检索，平均值 | 复用已有索引检索，平均值 |
| --- | --- | --- |
| 100 | 0.132 ms | 0.033 ms |
| 2,000 | 2.106 ms | 0.190 ms |
| 4,000 | 4.220 ms | 0.351 ms |

个人词库后台流程目前每次转写完成后建立一次索引；公共索引单独缓存。这些是本地检索均值，不是端到端延迟、P95 或其他文本和硬件的性能保证。分词、Jev 和润色另有开销。

## 按需启用模型评测

常规验证跳过 `TestLiveVocabularyCorpus`。明确需要调用收费接口时，通过环境变量提供 API key 后运行：

```bash
GOVOICE_JEV_LIVE=1 GOVOICE_JEV_SPLIT=eval go test ./internal/jev -run '^TestLiveVocabularyCorpus$' -count=1 -v
```

`TYPESAFE_API_KEY` 提供 key；`GOVOICE_JEV_ENDPOINT` 和 `GOVOICE_JEV_MODEL` 可选择其他兼容接口。凭据不应进入代码、样例和报告。调试阈值使用 `GOVOICE_JEV_SPLIT=dev`；`GOVOICE_JEV_PACKS=all`（默认）、`off`、`base` 或 `it` 可选择公共词库层进行配对比较；评测集与调参分离，记录不含凭据的实际端点和模型配置。

测试使用生产检索与 Jev 路径，分别报告纠正、误改、未纠正、保持正确、错误编辑、请求失败及各样例总耗时的 P95。无候选样例计入延迟，但不产生网络请求。错误编辑或接口失败会使测试失败；错误输入被保留计为未纠正。这只衡量 Jev，不包含后续生成式润色和 ASR。固定的 70% 阈值尚未重新校准。

以后比较模型时，应使用相同的独立标注输入和候选集，逐例对比、分别报告纠正与退步，并包含预期应保留原文的歧义样例。先扩展到 300～500 条多样化样例用于回归覆盖；判断细微的模型质量差距需要明显更大的评测集。不自动收集真实用户录音或转写。

## 后续模型工作

设计借鉴了 [chinese-ime-lm](https://github.com/metasequoiaime/chinese-ime-lm/tree/4e745be72f1c49e9c7b5dadea819fbb1fcda0e13) 的评测与介入条件经验，没有引入上游代码、语料或权重。词级读音、领域释义、经校准的近音惩罚、用户确认后的调频及本地模型比较仍是后续工作。本地汉字语言模型进入纠错流程前，需要针对语音识别进行评测，包括后文、中英混合及罕见用户专名。
