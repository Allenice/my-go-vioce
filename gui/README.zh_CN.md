简体中文 · [English](README.md)

# go-voice macOS 应用

`go-voice` 是 Go Voice 的 macOS 26+ Apple Silicon 桌面宿主，基于 MyGo 构建。独立 Go module 为 `github.com/vfasky/my-ai-audio/gui`，正式版 bundle identifier 为 `yz.sh.mygo-go-voice`。本地构建使用 `go-voice-debug.app` 和 `yz.sh.mygo-go-voice.debug`，三个 helper 的标识也追加 `.debug`，以分离开发版与正式版的 TCC 权限记录。

`go-voice` 原生 UI 提供设备与本次语音、词库、语音识别、AI 润色和固件安装五页，以及权限引导页；侧边栏另有“应用”分区，内含“通用”页。界面固定为暗色，遵循 macOS 26 布局：内嵌红绿灯的悬浮 Liquid Glass 侧边栏、位于柔和滚动边缘上的工具栏，以及分组内嵌区块。石墨卡片、黑色内嵌井、白色主胶囊按钮和仅用于实时音频的橙色强调色均参照设计稿；FoloToy AI Passport 声波以中心为最新峰值向两侧镜像展开，空闲时显示暗色圆点；识别、润色和插字时同一面板显示橙色脉冲。扫描时不会自动选择设备，明确选择后的 peripheral UUID 会持久化用于重连。设备页显示固件、握手状态、波形、RMS/peak、时长、实时转写、最终 ASR 原文、润色结果和帧诊断。不保存、不回放音频。

## 构建与验证

需要 Apple Silicon、macOS 26 或更新版本、Go 1.27.1、Rust/Cargo（含 `aarch64-apple-darwin` target）、Bun（可用 `WAKU_BUN_EXECUTABLE` 指定内置运行时），以及包含 SpeechAnalyzer 和 FoundationModels 的 Xcode SDK。Go 依赖版本固定在 `go.mod` 和 `go.sum`；Rust 依赖必须固定在 `firmware-helper/Cargo.lock`。gojieba 需要 CGO 和 Xcode C++ 编译器；`build.sh` 会启用 CGO，并从固定版本模块复制五个词典到 `Contents/Resources/jieba`，安装后的应用不依赖构建机的 Go 模块缓存。先在仓库根目录运行 `./tools/validate.sh --firmware`，生成两个已验证的固件产物。

在仓库根目录启动带签名、自动重构建和重启的调试应用：

```bash
bun run dev
```

该命令调用 `gui/scripts/dev.sh`，不需要 JavaScript 依赖或执行 `bun install`。Go 源码与配置变化时，会重新构建并重启 `go-voice-debug.app`；按 Ctrl+C 停止。Swift/Rust helper 变化需重新运行命令。

也可直接运行构建和验证脚本：

```bash
cd gui
./scripts/validate.sh
# 或仅构建：
./scripts/build.sh
# 带签名的调试会话（watch）：
./scripts/dev.sh
```

`dev.sh` 在会话期间保留固件与原生 helper staging，构建带签名的 debug app，并通过 LaunchServices（`open`）启动，使 TCC 将蓝牙和语音权限归属于 go-voice 本身。Go 源码、`go.mod`/`go.sum`、`mygo.json`、图标或 entitlements 变化时重新构建；构建成功会先退出正在运行的应用（SIGTERM，使 helper 正常关闭）再启动新版本，构建失败则保留当前应用。应用输出写入 `gui/build/dev.log`。退出应用或按 Ctrl+C 结束会话并清理 staging。Swift 或 Rust helper 变更需重新开始会话。不使用 `mygo dev`，因为它把应用作为自身子进程启动，TCC 会把启动终端视为责任进程并终止 BluetoothHelper。`MYGO_SIGN_IDENTITY` 可指定签名身份；未指定时本地构建优先使用已有的 Apple Development 证书，没有可用证书才回退为 ad hoc。稳定的证书可使调试版跨次构建保持相同的 designated requirement；ad-hoc 重构建仍可能需要重新授权。`scripts/stage-debug-project.mjs` 单独生成临时调试配置，`mygo.json` 保留正式版名称和身份；调试版不包含更新配置。

验证脚本运行固件打包 fixture 测试、Cargo 格式/测试/Clippy、gofmt、`go test ./...`、`go vet ./...` 和 Swift 构建/测试，然后构建三个原生 helper 与 MyGo debug app。检查包内固件摘要和 manifest、内外签名及 arm64 可执行文件。结果位于 `gui/build/debug/darwin-arm64/go-voice-debug.app`。应启动 app 包，不使用 `go run`，以正确定位原生资源和 TCC 身份。构建不会启动应用、请求系统权限、连接 FoloToy AI Passport 或访问 USB 设备。

仓库根目录也可运行 `./tools/validate.sh --gui` 执行同一门禁。GUI 验证读取 `build/FoloToy-AI-Passport.bin`，不改变任一根固件产物。构建临时生成 `gui/firmware/go-voice-app.bin` 和 `manifest.json`，将目录打包至 `Contents/Resources/firmware`，退出时清理 staging，构建失败也会清理。源码树不长期保留固件二进制。`FirmwareHelper.app` 位于 `Contents/Resources/native`，正式版身份为 `yz.sh.mygo-go-voice.firmware-helper`（本地构建为 `yz.sh.mygo-go-voice.firmware-helper.debug`）；内层 helper 先于外层 app 签名。

## 固件安装契约

USB 安装只向 `0x10000` 写入包内应用，最大 3 MB。写入前 helper 必须验证镜像摘要/大小、ESP32-C3 身份、8 MB Flash、分区表完整性和 factory/cardid/recovery 布局，以及永久 Recovery 启动钩子标记。预检失败必须在任何擦除/写入前停止。不提供 erase-all 操作。必须保留 bootloader、分区表、设备身份（`cardid` 位于 `0x356000`）与永久 Recovery（`0x700000`）。

安装失败时，按住 UP 上电五秒进入永久 Recovery，使用小程序重新安装。小程序/社区产物仍为 `FoloToy-AI-Passport-full.bin`；桌面 app-only 镜像不能替代它。待完成的集成与真机检查见[验收记录](docs/acceptance.zh_CN.md)。

## 运行与隐私

`internal/app` 是唯一组装层。UI 回调只发送意图，后台事件通过 `mygo.RunOnMain` 回到 UI 线程。协调器管理会话代次、取消、早到音频缓存及终态。Go 把独立 ADPCM 帧解码为 16 kHz、有符号 16-bit 小端单声道 PCM。默认由 Swift 子进程直接接收 PCM，使用 SpeechAnalyzer/SpeechTranscriber 的 volatile results 和当前词库，不创建 Mac 麦克风输入。

在“语音识别”页选择在线引擎后，由应用自身把这些 PCM 按 100 ms 一包，经 WebSocket（`wss://maas-api.antdigital.com/api-ws/v1/inference`，DashScope 双工任务协议：run-task、二进制音频、finish-task）实时发送到支付宝 MaaS 实时模型（`fun-asr-realtime` 或 `paraformer-realtime-v2`）；中间结果到达即显示在胶囊中，task-finished 后得到最终转写。非流式的 `qwen3-asr-flash` 则把本次音频留在内存中，松开按键后封装为一个 WAV（Base64 data URL）发送到 `https://maas-api.antdigital.com/api/v1/services/aigc/multimodal-generation/generation`；说话时不显示中间文字，请求最长等待 45 秒。此时 SpeechBridge 只登记会话和插字目标（`begin`），AI 润色与插字流程与本地识别相同，本次会话也不需要“语音识别”权限。明确选择并保存在线引擎即允许把每次语音的音频发送到支付宝 MaaS；不发送词库，也不保存音频。开始时最多留 4 秒建立连接，期间早到的音频会先缓存；缺少或无效的 API key、key 无权使用该模型、超时或任务失败都会以中文原因结束本次会话，不会自动改用本地识别。页面上的“测试连接”只发送半秒静音。API key 仅存入 macOS 钥匙串（账户 `asr:alipay-maas`）。

新增的阿里云百炼 Omni Realtime 选项遵循 [Omni Realtime 交互协议](https://help.aliyun.com/zh/model-studio/omni-realtime-interaction-process)及其[输入转写事件](https://help.aliyun.com/zh/model-studio/server-events)。默认模型为 `qwen3-omni-flash-realtime`，端点为 `wss://dashscope.aliyuncs.com/api-ws/v1/realtime`；页面支持填写其他 Omni Realtime 模型及 WSS 端点，包括服务商提供的业务空间地域地址。API key 必须对应所选地域和业务空间。端点不能包含凭据、查询参数或片段，模型名会单独添加。每个 Omni 端点使用独立的钥匙串账户，不复用支付宝密钥。收到 `session.created` 后发送 `session.update`，配置 16 kHz PCM、开启输入转写、设置 `turn_detection: null`；等待 `session.updated` 后，通过 `input_audio_buffer.append` 发送 Base64 PCM。松开按键后发送 `input_audio_buffer.commit`。预览事件以 `text + stash` 替换当前句子，只有已提交消息项的最终输入转写才结束听写。不发送 `response.create`，不使用模型回答或生成音频。收尾最多等待 15 秒，取消时即使写入阻塞也会关闭连接。“测试连接”只发送半秒静音。真实服务行为及设备听写仍待验证。

BluetoothHelper 只使用[权威 BLE 协议](../docs/ai-passport-ble-protocol.zh_CN.md)中的 Go Voice UUID。helper 在发送 host_ready 前确认 MTU 和订阅，有响应写重试、断线退避重连及父进程退出清理。两侧进程边界均使用有界 NDJSON 与 canonical Base64。音频缓存只存在内存中。

启动模型准备会检查/下载并保留 zh-CN 资产，再预热分析器。权限在启动时检查、使用前申请，绝不在会话中途弹出：“权限”页将蓝牙、语音识别和辅助功能列为必需，自动化（System Events）为可选；逐项显示状态，仍可弹出系统请求时提供“授权”，否则打开对应的“隐私与安全性”面板，并在仍有缺失时每两秒刷新。全部状态上报后若缺少必需权限，应用会自动打开该页，设备页也提供入口。未授予语音识别时开始的会话会立即以该原因结束，而不是报启动超时。

会话期间，听写胶囊条悬浮在指针所在屏幕的底部中央、程序坞上方，在所有桌面空间可见。胶囊左侧为取消按钮（仅在会话期间全局占用 Esc），随后是小声波和一行定宽文字，然后是 AI 润色期间可点击的确认按钮，用于跳过润色直接插入 ASR 原文。说话和识别收尾时，这行文字实时显示识别出的内容：最新的字贴在右侧并平滑滑入，较早的字向左滚出并在左缘渐隐；尚未识别出文字时显示“请说话…”，其他阶段显示会话状态（准备、识别、润色）。AI 润色期间，胶囊上方的预览气泡显示将要插入的 ASR 原文（最多三行）。它是不获取键盘焦点显示的 MyGo 面板，并在任何粘贴前隐藏，确保文字进入你听写的目标应用；会话被取消或失败时显示原因 1.5 秒。

go-voice 是菜单栏应用：`LSUIElement` 使其不出现在程序坞和应用切换器中，菜单栏模板图标提供“设置…”（Cmd+,，显示主窗口）、“检查更新…”（正式版打开 MyGo 签名更新窗口；调试版说明其不会自动更新）和“退出”。关闭主窗口只会隐藏，应用继续驻留菜单栏。侧边栏底部连接状态卡片右侧的电源按钮也可退出应用，走同样的退出流程；固件安装期间该按钮不可用。“通用”页提供“登录时打开”开关：通过 MyGo（`SMAppService`，系统设置 › 通用 › 登录项）把 go-voice 加入当前用户的登录项，开关始终显示系统报告的状态，在系统设置中关闭也会同步。macOS 要求用户允许新的登录项时，页面会说明并提供“打开登录项”，直到允许为止。登录时启动的 go-voice 只驻留菜单栏、不打开窗口；缺少必需权限时才显示权限页。该设置不写入 `settings.json`。图标 `gui/resources/tray.png` 由透明白色标志经 `"$WAKU_BUN_EXECUTABLE" gui/scripts/generate-tray-icon.mjs`（需要 FFmpeg）生成：按图案裁切，36×36 像素，黑色透明底模板图。包含 Bluetooth、Speech、Apple Events 权限文案，不包含麦克风权限或 audio-input entitlement。插字还需要用户授予辅助功能权限。Secure Input、缺少辅助功能授权或目标应用不可用时，最终文本仅留在剪贴板。粘贴优先 System Events，失败使用 CGEvent；0.5 秒后仅在剪贴板归属未变时恢复原内容。

默认使用 Apple FoundationModels 本地润色，提供关闭、专名、轻度（默认）和结构化模式。专名只按词库纠正专有名词；轻度先按词库纠正，再去掉没有意义的语气词（叹词和犹豫声、只用来停顿的“那个”“就是”、说错重来的重复词、因犹豫声被识别出的单个字母，如“Y”），并修正标点，不改写、不调整语序、不添加内容；结构化在此基础上按内容分段或列点。忠实度检查不把去掉的语气词算作丢失内容。无论本地还是远端，所有启用的模式都会优先按本次词库纠正专有名词（同音字、音译、大小写和空格），但不会加入没说过的词。调用模型前，应用先比对拼音（合并 z/zh、c/ch、s/sh、n/l 和 -n/-ng），提供候选片段和 gojieba 本地分词。完整词候选排在切断词语的候选之前；后者仍作为提示保留，但需要更强的上下文支持，已经正确写出的词条受保护。关闭 Jev 时也生效。本次词表中的中文词会加入本地分词词典；词集合变化时重新建立词典，删除或改名的条目不会残留到后续调用。远端请求会要求关闭思考模式（`reasoning_effort`/`reasoning.effort: none`、`thinking: disabled`、`enable_thinking: false` 和对话模板参数）；服务商以 HTTP 400/422 拒绝这些参数时自动减少参数重试，并按端点和模型记住可接受的级别；回复中的 `<think>` 内容会被去掉；忠实度检查把这类替换视为纠正，并设有上限，词库词不能替代原本的听写内容。失败、超时、空输出或字符忠实度过低都保留最终 ASR 原文。本地失败不会调用远端。明确选择并保存 `remote` 后，才允许把最终转写和当前词库发往配置的 HTTPS OpenAI-compatible 端点；音频不发送。设置页将其标注为 OpenAI 兼容接口，并提供 Chat Completions（默认）和 Responses 两种接口格式。填写例如 `https://provider.example/v1` 的基础端点和模型名，应用按所选格式追加 `/chat/completions` 或 `/responses`。Responses 请求设置 `store: false`，且只接受恰好一个已完成的 `message` 输出。“测试连接”在保存前按页面当前填写的内容检查接口，优先使用输入框中的 API key，否则使用已保存的 key：即使当前选择本地引擎，也只发送一段固定示例句，从不发送听写内容或词库；成功时显示润色后的示例，失败时给出可能原因（缺少端点/模型/key、HTTP 401/403/404/429/5xx、超时，或返回了另一种接口格式）。“保存设置”会把输入的 API key 一并按端点存入钥匙串；“测试连接”“保存设置”“保存到钥匙串”“删除凭据”的结果都显示在按钮下方，表单再次修改后自动隐藏。API key 仅通过 Security.framework 存入 macOS Keychain，不写入设置、日志、测试夹具或 `mygo.json`。

设置和词库以受限权限保存在 `~/Library/Application Support/MyGo-GoVoice/`。词库保留来源、权重、锁定、创建和使用时间，按标准化术语去重，每次会话最多注入 100 个 contextual strings：先选择排序后的显式词条，再用去重后的替换目标补齐剩余名额。同一词表供 Apple 本地识别、分词、Jev 和润色使用；不注入错误写法，也不会把派生目标另存为词条。词库页分为“词条”“替换词”和“Jev 判断”三个标签页。“替换词”页的每条规则由两个输入框填写：原词（识别出的错误写法，例如“施玉宝”）和替换词（正确写法，例如“私域宝”）。实时和最终识别结果会在本地直接替换，不经过 AI，并先于胶囊预览、插入和润色生效，较长的原词优先；某条规则替换出的结果还会继续被其他规则修正，AI 润色结果也会再替换一次；英文原词不区分大小写，只匹配完整单词。原词不能是词库中的词条，同一原词只能有一条规则。最终纠正还会按完整词边界匹配被水平空白拆开的中文原词，保留标点和换行、不切断已有词语，只应用已配置的映射，不自行推测新的同音替换。这一步使用词库快照在后台运行，关闭 Jev 和 AI 润色时也生效。字面规则优先，实时预览仍按字面规则替换；超过 8,000 字或词典不可用时跳过额外分词，保留字面纠正。早前版本登记在词条里的替换词会在读取时自动迁移成规则。“导出”把词条（含锁定和权重）和替换词写入一个 JSON 文件（`"format": "go-voice-vocabulary"`），不含使用记录；“导入”合并这样的文件或应用自己的 `vocabulary.json`：新增没有的条目，更新已有条目的锁定、权重和替换目标，不删除任何内容，跳过无效或冲突的条目并报告数量。其他格式的文件不会改动词库。“Jev 判断”页可开启 TypeSafe 的 Jev 模型判断同音词替换（默认关闭）：每次语音结束后，先用 gojieba 本地分词提供词边界，再由 Jev 结合完整转写，对每处重叠的候选词与保留原文进行一道 Choice 选择题。词边界作为线索而非硬性过滤，保留被切碎或与邻字合并的陌生专名。只匹配两个字以上中文词条的同音或近音，已经正确写出的词库词受保护。候选词的选择概率达到 70% 才直接替换；保留原文的概率达到 70% 则要求 AI 润色不要改，拿不准的交给润色。关闭 AI 润色时也生效。接口地址和模型可以修改：默认 TypeSafe 官方 `https://api.typesafe.ai/v1/systemone` 和 `jev-latest`，另有“支付宝 MaaS”预设（`https://maas-api.antdigital.com/v1/systemone`、`jev-1.13`），也可以填写其他兼容 System One 接口的服务；只填域名时自动补上 `/v1/systemone`，地址必须是 https 且不含账号、查询参数，否则不保存。请求包含完整转写、本地分词结果和命中的候选词，不发送音频和无关词库；没有命中时不发送请求。每次最多判断 24 处，每处最多 63 个候选加保留原文；候选过多的整组留给润色，不截掉部分选项。文字超过 8,000 个 Unicode 字符或请求超过 128 KiB 时跳过，不截断上下文。超过 4 秒或失败时跳过这一步并在状态栏说明原因。“测试连接”按页面上填写的地址和模型只发送固定示例句，并显示每处的概率和处理结果。API key 仅存入 macOS 钥匙串，按接口地址分别保存（TypeSafe 官方为账户 `vocabulary:typesafe-jev`，其他为 `vocabulary:jev:<接口地址>`）。转写仅显示本次会话，不持久保存音频或转写历史。

## 发布与更新

正式版使用团队 Developer ID 和 hardened runtime 签名，经过公证并 staple，并从已签名的 `go-voice-v*` GitHub release 自动更新，每天检查一次。`./scripts/release.sh` 负责测试、构建（`./scripts/build.sh --release`）、公证、验证并上传草稿 release；公开草稿后更新才会推送。密钥、准备工作、发布流程与更新验收见[更新、签名与公证](docs/updates.zh_CN.md)。

## 文档

- [实现与验收记录](docs/acceptance.zh_CN.md)
- [更新、签名与公证](docs/updates.zh_CN.md)与[发布说明](docs/CHANGELOG.zh_CN.md)
- [第三方声明](docs/THIRD_PARTY_NOTICES.zh_CN.md)
- [根架构文档](../docs/passport-audio-input-architecture.zh_CN.md)
- [根目录契约](../docs/project-structure.zh_CN.md)

构建与主机单测成功不代表真实设备或 TCC 验收通过。剩余检查详见验收记录。
