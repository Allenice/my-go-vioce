<p align="right">
  <strong>简体中文</strong> · <a href="CHANGELOG.md">English</a>
</p>

# go-voice 发布说明

每个 `## <版本>` 小节是一次已签名 `go-voice` 发布的说明。`scripts/release.sh` 要求两个语言文件都有该小节；应用界面为中文，因此简体中文小节会签入更新 manifest，并显示在应用内更新窗口。只写用户可感知的变化，两种语言保持一致。参见[更新与发布](updates.zh_CN.md)。

## 0.3.0

- “语音识别”页新增阿里云百炼 Omni Realtime WebSocket，可填写端点、模型与 API key；录音时实时上传，松开按键后提交，只插入输入语音的转写结果，凭据与支付宝 MaaS 分开保存。
- 本地构建命名为 `go-voice-debug`，主应用和所有 helper 使用独立的 macOS 权限身份，避免开发版覆盖正式版的权限记录。
- 在仓库根目录使用 `bun run dev` 启动带签名的 watch 调试会话；Go 源码与配置变化后自动重构建并重启调试应用。
- [产品网站](https://go-voice.vfasky.com/)提供最新 Apple 芯片版下载、使用说明及历史更新日志。

在 macOS 26 及以上系统安装 Apple 芯片版 DMG，或在已安装的正式版中选择“检查更新…”。在“语音识别”页选择阿里云百炼 Omni Realtime，填写与端点和模型匹配的 API key。构建与配置步骤见[应用指南](https://github.com/vfasky/my-ai-audio/blob/go-voice-v0.3.0/gui/README.zh_CN.md)。真实 Omni 识别、设备听写及已安装应用升级后的授权保留仍待验证。

## 0.2.0

- 新增“语音识别”页面，可选在线识别：支付宝百灵 MaaS 的 Fun-ASR、Paraformer v2 边说边返回文字，Qwen3-ASR-Flash 在松开按键后识别整段录音。默认仍使用 Apple 本地识别。在线识别会将本次语音发送至支付宝百灵 MaaS，API 密钥保存在钥匙串中，失败时说明原因，不切换引擎。
- 悬浮胶囊实时显示识别文字，新文字从右侧滑入，Apple 本地识别现在约每秒更新一次。预览气泡在 AI 润色时出现，方便直接插入识别原文。
- 词库页面可选启用 Jev，在润色前判断中文同音词纠正，关闭 AI 润色时也可生效。支持 TypeSafe 和支付宝百灵 MaaS 等兼容 System One 的服务。只发送匹配片段、周边上下文和候选词条，API 密钥保存在钥匙串中。默认关闭，超过四秒或失败时跳过。
- 新增“通用”页面的“登录时启动”开关。登录后 go-voice 常驻菜单栏，仅在缺少必需权限时打开权限引导；开关同步 macOS 登录项设置。

在 macOS 26 及以上系统安装 Apple 芯片版 DMG，或在已安装的正式版中选择“检查更新…”。配置步骤见[应用指南](https://github.com/vfasky/my-ai-audio/blob/go-voice-v0.2.0/gui/README.zh_CN.md)，构建与签名方法见[发布指南](https://github.com/vfasky/my-ai-audio/blob/go-voice-v0.2.0/gui/docs/updates.zh_CN.md)。本版本尚未验证实机听写、真实在线服务的识别质量、登录项行为及已安装应用的更新流程。

## 0.1.0

- go-voice 首个签名并公证的版本，适用于 macOS 26 及以上的 Apple 芯片 Mac。
- 通过蓝牙接收所选 FoloToy AI Passport 的语音，在本机转写、可选 AI 润色，并插入到听写时所在的应用。悬浮胶囊实时预览文字，支持取消或直接插入。
- 管理词条和本地替换规则，并将它们一起导入、导出为 JSON。关闭 AI 润色时，替换规则仍然生效。
- 可选 Apple FoundationModels 或 OpenAI 兼容润色引擎，支持 Chat Completions 和 Responses。优先按词库纠正专有名词，轻度润色去除无意义语气词，并可在保存前测试远端连接。
- 按权限引导完成设置，通过 USB 安装内置应用固件，同时保留设备身份和永久 Recovery。
- 菜单栏“检查更新…”现在可以安装已签名的更新；正式版也会每天自动检查一次。

下载 Apple 芯片版 DMG，将 go-voice 拖入“应用程序”，打开后完成权限引导，再选择你的 FoloToy AI Passport。配置步骤见[应用指南](https://github.com/vfasky/my-ai-audio/blob/go-voice-v0.1.0/gui/README.zh_CN.md)，构建与签名方法见[发布指南](https://github.com/vfasky/my-ai-audio/blob/go-voice-v0.1.0/gui/docs/updates.zh_CN.md)。
