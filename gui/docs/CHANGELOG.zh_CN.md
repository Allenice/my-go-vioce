<p align="right">
  <strong>简体中文</strong> · <a href="CHANGELOG.md">English</a>
</p>

# go-voice 发布说明

每个 `## <版本>` 小节是一次已签名 `go-voice` 发布的说明。`scripts/release.sh` 要求两个语言文件都有该小节；应用界面为中文，因此简体中文小节会签入更新 manifest，并显示在应用内更新窗口。只写用户可感知的变化，两种语言保持一致。参见[更新与发布](updates.zh_CN.md)。

## 0.1.0

- go-voice 首个签名并公证的版本，适用于 macOS 26 及以上的 Apple 芯片 Mac。
- 通过蓝牙接收所选 FoloToy AI Passport 的语音，在本机转写、可选 AI 润色，并插入到听写时所在的应用。悬浮胶囊实时预览文字，支持取消或直接插入。
- 管理词条和本地替换规则，并将它们一起导入、导出为 JSON。关闭 AI 润色时，替换规则仍然生效。
- 可选 Apple FoundationModels 或 OpenAI 兼容润色引擎，支持 Chat Completions 和 Responses。优先按词库纠正专有名词，轻度润色去除无意义语气词，并可在保存前测试远端连接。
- 按权限引导完成设置，通过 USB 安装内置应用固件，同时保留设备身份和永久 Recovery。
- 菜单栏“检查更新…”现在可以安装已签名的更新；正式版也会每天自动检查一次。

下载 Apple 芯片版 DMG，将 go-voice 拖入“应用程序”，打开后完成权限引导，再选择你的 FoloToy AI Passport。配置步骤见[应用指南](https://github.com/vfasky/my-ai-audio/blob/go-voice-v0.1.0/gui/README.zh_CN.md)，构建与签名方法见[发布指南](https://github.com/vfasky/my-ai-audio/blob/go-voice-v0.1.0/gui/docs/updates.zh_CN.md)。
