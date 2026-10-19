<p align="right">
  <strong>简体中文</strong> · <a href="CHANGELOG.md">English</a>
</p>

# go-voice 发布说明

每个 `## <版本>` 小节是一次已签名 `go-voice` 发布的说明。`scripts/release.sh` 要求两个语言文件都有该小节；应用界面为中文，因此简体中文小节会签入更新 manifest，并显示在应用内更新窗口。只写用户可感知的变化，两种语言保持一致。参见[更新与发布](updates.zh_CN.md)。

## 0.1.0

- go-voice 首个签名并公证的版本，适用于 macOS 26 及以上的 Apple 芯片 Mac。
- 通过蓝牙接收所选 FoloToy AI Passport 的语音，在本机转写、可选 AI 润色，并插入到听写时所在的应用。
- 管理词表、选择 AI 润色引擎，并通过 USB 安装内置的应用固件。
- 菜单栏“检查更新…”现在可以安装已签名的更新；正式版也会每天自动检查一次。
