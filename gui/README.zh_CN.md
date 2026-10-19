简体中文 · [English](README.md)

# go-voice macOS 应用

`go-voice` 是 Go Voice 的 macOS 26+ Apple Silicon 桌面宿主，基于 MyGo 构建。独立 Go module 为 `github.com/vfasky/my-ai-audio/gui`，bundle identifier 为 `yz.sh.mygo-go-voice`。

`go-voice` 原生 UI 提供设备与本次语音、词库、AI 润色和固件安装四页。界面固定为暗色，遵循 macOS 26 布局：内嵌红绿灯的悬浮 Liquid Glass 侧边栏、位于柔和滚动边缘上的工具栏，以及分组内嵌区块。石墨卡片、黑色内嵌井、白色主胶囊按钮和仅用于实时音频的橙色强调色均参照设计稿；FoloToy AI Passport 声波以中心为最新峰值向两侧镜像展开，空闲时显示暗色圆点；识别、润色和插字时同一面板显示橙色脉冲。扫描时不会自动选择设备，明确选择后的 peripheral UUID 会持久化用于重连。设备页显示固件、握手状态、波形、RMS/peak、时长、实时转写、最终 ASR 原文、润色结果和帧诊断。不保存、不回放音频。

## 构建与验证

需要 Apple Silicon、macOS 26 或更新版本、Go 1.27.1、Rust/Cargo（含 `aarch64-apple-darwin` target）、Bun（可用 `WAKU_BUN_EXECUTABLE` 指定内置运行时），以及包含 SpeechAnalyzer 和 FoundationModels 的 Xcode SDK。Go 依赖版本固定在 `go.mod` 和 `go.sum`；Rust 依赖必须固定在 `firmware-helper/Cargo.lock`。先在仓库根目录运行 `./tools/validate.sh --firmware`，生成两个已验证的固件产物。

```bash
cd gui
./scripts/validate.sh
# 或仅构建：
./scripts/build.sh
# 带签名的调试会话（watch）：
./scripts/dev.sh
```

`dev.sh` 在会话期间保留固件与原生 helper staging，构建带签名的 debug app，并通过 LaunchServices（`open`）启动，使 TCC 将蓝牙和语音权限归属于 go-voice 本身。Go 源码、`go.mod`/`go.sum`、`mygo.json`、图标或 entitlements 变化时重新构建；构建成功会先退出正在运行的应用（SIGTERM，使 helper 正常关闭）再启动新版本，构建失败则保留当前应用。应用输出写入 `gui/build/dev.log`。退出应用或按 Ctrl+C 结束会话并清理 staging。Swift 或 Rust helper 变更需重新开始会话。不使用 `mygo dev`，因为它把应用作为自身子进程启动，TCC 会把启动终端视为责任进程并终止 BluetoothHelper。`MYGO_SIGN_IDENTITY` 可指定签名身份（默认 ad hoc）。

验证脚本运行固件打包 fixture 测试、Cargo 格式/测试/Clippy、gofmt、`go test ./...`、`go vet ./...` 和 Swift 构建/测试，然后构建三个原生 helper 与 MyGo debug app。检查包内固件摘要和 manifest、内外签名及 arm64 可执行文件。结果位于 `gui/build/debug/darwin-arm64/go-voice.app`。应启动 app 包，不使用 `go run`，以正确定位原生资源和 TCC 身份。构建不会启动应用、请求系统权限、连接 FoloToy AI Passport 或访问 USB 设备。

仓库根目录也可运行 `./tools/validate.sh --gui` 执行同一门禁。GUI 验证读取 `build/FoloToy-AI-Passport.bin`，不改变任一根固件产物。构建临时生成 `gui/firmware/go-voice-app.bin` 和 `manifest.json`，将目录打包至 `Contents/Resources/firmware`，退出时清理 staging，构建失败也会清理。源码树不长期保留固件二进制。`FirmwareHelper.app` 位于 `Contents/Resources/native`，独立身份为 `yz.sh.mygo-go-voice.firmware-helper`；内层 helper 先于外层 app 签名。

## 固件安装契约

USB 安装只向 `0x10000` 写入包内应用，最大 3 MB。写入前 helper 必须验证镜像摘要/大小、ESP32-C3 身份、8 MB Flash、分区表完整性和 factory/cardid/recovery 布局，以及永久 Recovery 启动钩子标记。预检失败必须在任何擦除/写入前停止。不提供 erase-all 操作。必须保留 bootloader、分区表、设备身份（`cardid` 位于 `0x356000`）与永久 Recovery（`0x700000`）。

安装失败时，按住 UP 上电五秒进入永久 Recovery，使用小程序重新安装。小程序/社区产物仍为 `FoloToy-AI-Passport-full.bin`；桌面 app-only 镜像不能替代它。待完成的集成与真机检查见[验收记录](docs/acceptance.zh_CN.md)。

## 运行与隐私

`internal/app` 是唯一组装层。UI 回调只发送意图，后台事件通过 `mygo.RunOnMain` 回到 UI 线程。协调器管理会话代次、取消、早到音频缓存及终态。Go 把独立 ADPCM 帧解码为 16 kHz、有符号 16-bit 小端单声道 PCM。Swift 子进程直接接收 PCM，使用 SpeechAnalyzer/SpeechTranscriber 的 volatile results 和当前词库，不创建 Mac 麦克风输入。

BluetoothHelper 只使用[权威 BLE 协议](../docs/ai-passport-ble-protocol.zh_CN.md)中的 Go Voice UUID。helper 在发送 host_ready 前确认 MTU 和订阅，有响应写重试、断线退避重连及父进程退出清理。两侧进程边界均使用有界 NDJSON 与 canonical Base64。音频缓存只存在内存中。

启动模型准备会检查/下载并保留 zh-CN 资产，再预热分析器。权限在启动时检查、使用前申请，绝不在会话中途弹出：“权限”页将蓝牙、语音识别和辅助功能列为必需，自动化（System Events）为可选；逐项显示状态，仍可弹出系统请求时提供“授权”，否则打开对应的“隐私与安全性”面板，并在仍有缺失时每两秒刷新。全部状态上报后若缺少必需权限，应用会自动打开该页，设备页也提供入口。未授予语音识别时开始的会话会立即以该原因结束，而不是报启动超时。

会话期间，听写胶囊条悬浮在指针所在屏幕的底部中央、程序坞上方，在所有桌面空间可见。胶囊上方的预览气泡在说话时显示最多三行最新识别的文字，AI 润色期间显示将要插入的 ASR 原文。胶囊左侧为取消按钮（仅在会话期间全局占用 Esc），随后是小声波、会话状态（聆听、识别、润色），以及 AI 润色期间可点击的确认按钮，用于跳过润色直接插入 ASR 原文。它是不获取键盘焦点显示的 MyGo 面板，并在任何粘贴前隐藏，确保文字进入你听写的目标应用；会话被取消或失败时显示原因 1.5 秒。

go-voice 是菜单栏应用：`LSUIElement` 使其不出现在程序坞和应用切换器中，菜单栏模板图标提供“设置…”（Cmd+,，显示主窗口）、“检查更新…”（正式版打开 MyGo 签名更新窗口；调试版说明其不会自动更新）和“退出”。关闭主窗口只会隐藏，应用继续驻留菜单栏。图标 `gui/resources/tray.png` 由透明白色标志经 `"$WAKU_BUN_EXECUTABLE" gui/scripts/generate-tray-icon.mjs`（需要 FFmpeg）生成：按图案裁切，36×36 像素，黑色透明底模板图。包含 Bluetooth、Speech、Apple Events 权限文案，不包含麦克风权限或 audio-input entitlement。插字还需要用户授予辅助功能权限。Secure Input、缺少辅助功能授权或目标应用不可用时，最终文本仅留在剪贴板。粘贴优先 System Events，失败使用 CGEvent；0.5 秒后仅在剪贴板归属未变时恢复原内容。

默认使用 Apple FoundationModels 本地润色，提供关闭、专名、轻度（默认）和结构化模式。专名只按词库纠正专有名词；轻度先按词库纠正，再去掉没有意义的语气词（叹词和犹豫声、只用来停顿的“那个”“就是”、说错重来的重复词、因犹豫声被识别出的单个字母，如“Y”），并修正标点，不改写、不调整语序、不添加内容；结构化在此基础上按内容分段或列点。忠实度检查不把去掉的语气词算作丢失内容。无论本地还是远端，所有启用的模式都会优先按本次词库纠正专有名词（同音字、音译、大小写和空格），但不会加入没说过的词。调用模型前，应用先比对拼音（合并 z/zh、c/ch、s/sh、n/l 和 -n/-ng），把读音像中文词库词的片段列为候选，要求模型除非原词明显正确，否则替换成词库词。远端请求会要求关闭思考模式（`reasoning_effort`/`reasoning.effort: none`、`thinking: disabled`、`enable_thinking: false` 和对话模板参数）；服务商以 HTTP 400/422 拒绝这些参数时自动减少参数重试，并按端点和模型记住可接受的级别；回复中的 `<think>` 内容会被去掉；忠实度检查把这类替换视为纠正，并设有上限，词库词不能替代原本的听写内容。失败、超时、空输出或字符忠实度过低都保留最终 ASR 原文。本地失败不会调用远端。明确选择并保存 `remote` 后，才允许把最终转写和当前词库发往配置的 HTTPS OpenAI-compatible 端点；音频不发送。设置页将其标注为 OpenAI 兼容接口，并提供 Chat Completions（默认）和 Responses 两种接口格式。填写例如 `https://provider.example/v1` 的基础端点和模型名，应用按所选格式追加 `/chat/completions` 或 `/responses`。Responses 请求设置 `store: false`，且只接受恰好一个已完成的 `message` 输出。“测试连接”在保存前按页面当前填写的内容检查接口，优先使用输入框中的 API key，否则使用已保存的 key：即使当前选择本地引擎，也只发送一段固定示例句，从不发送听写内容或词库；成功时显示润色后的示例，失败时给出可能原因（缺少端点/模型/key、HTTP 401/403/404/429/5xx、超时，或返回了另一种接口格式）。“保存设置”会把输入的 API key 一并按端点存入钥匙串；“测试连接”“保存设置”“保存到钥匙串”“删除凭据”的结果都显示在按钮下方，表单再次修改后自动隐藏。API key 仅通过 Security.framework 存入 macOS Keychain，不写入设置、日志、测试夹具或 `mygo.json`。

设置和词库以受限权限保存在 `~/Library/Application Support/MyGo-GoVoice/`。词库保留来源、权重、锁定、创建和使用时间，按标准化术语去重，每次会话最多注入 100 个 contextual strings。词库页分为“词条”和“替换词”两个标签页。“替换词”页的每条规则由两个输入框填写：原词（识别出的错误写法，例如“施玉宝”）和替换词（正确写法，例如“私域宝”）。实时和最终识别结果会在本地直接替换，不经过 AI，并先于胶囊预览、插入和润色生效，较长的原词优先；某条规则替换出的结果还会继续被其他规则修正，AI 润色结果也会再替换一次；英文原词不区分大小写，只匹配完整单词。原词不能是词库中的词条，同一原词只能有一条规则。早前版本登记在词条里的替换词会在读取时自动迁移成规则。“导出”把词条（含锁定和权重）和替换词写入一个 JSON 文件（`"format": "go-voice-vocabulary"`），不含使用记录；“导入”合并这样的文件或应用自己的 `vocabulary.json`：新增没有的条目，更新已有条目的锁定、权重和替换目标，不删除任何内容，跳过无效或冲突的条目并报告数量。其他格式的文件不会改动词库。转写仅显示本次会话，不持久保存音频或转写历史。

## 发布与更新

正式版使用团队 Developer ID 和 hardened runtime 签名，经过公证并 staple，并从已签名的 `go-voice-v*` GitHub release 自动更新，每天检查一次。`./scripts/release.sh` 负责测试、构建（`./scripts/build.sh --release`）、公证、验证并上传草稿 release；公开草稿后更新才会推送。密钥、准备工作、发布流程与更新验收见[更新、签名与公证](docs/updates.zh_CN.md)。

## 文档

- [实现与验收记录](docs/acceptance.zh_CN.md)
- [更新、签名与公证](docs/updates.zh_CN.md)与[发布说明](docs/CHANGELOG.zh_CN.md)
- [第三方声明](docs/THIRD_PARTY_NOTICES.zh_CN.md)
- [根架构文档](../docs/passport-audio-input-architecture.zh_CN.md)
- [根目录契约](../docs/project-structure.zh_CN.md)

构建与主机单测成功不代表真实设备或 TCC 验收通过。剩余检查详见验收记录。
