简体中文 · [English](README.md)

# Go Voice 文档

Go Voice 由 AI Passport 固件和基于 MyGo 的 macOS 语音输入应用组成。工牌采集音频，Mac 负责端侧语音识别、词库注入、可选 AI 润色和文字插入。支持的硬件与软件要求见[项目首页](../README.zh_CN.md)，具体操作与开发资料见下方索引。

## 使用与构建

- [macOS 应用](../gui/README.zh_CN.md)：环境要求、构建命令、权限、词库、润色和隐私。
- [固件](go-voice-firmware.zh_CN.md)：来源归属、Go Voice 标识、显示资源、保护存储和已记录的验证结果。
- [环境搭建](development/engineering/environment-setup.zh_CN.md)：ESP-IDF 5.5.3 安装与配置。
- [构建与测试](development/engineering/build-and-test.zh_CN.md)：统一验证入口与固件产物。
- [硬件指南](hardware-design/AI_HARDWARE_DEVELOPMENT_GUIDE.zh_CN.md)与[产品规格](hardware-design/specifications.zh_CN.md)：板级接口和资源限制。引脚定义位于 `components/bsp/include/bsp_pins.h`。

## 架构与契约

- [系统架构](passport-audio-input-architecture.zh_CN.md)：工牌到 Mac 的音频流、会话归属、词库、润色和插字。
- [项目结构](project-structure.zh_CN.md)：当前固件与桌面端源码布局。
- [BLE V1 协议](ai-passport-ble-protocol.zh_CN.md)：UUID、控制消息、音频帧及兼容性的权威定义。
- [Recovery 兼容规范](development/engineering/ble-recovery-compatibility.zh_CN.md)：3 MB 应用上限、`0x356000` 的 `cardid`、`0x700000` 的永久 Recovery，以及上电长按 UP 五秒的 bootloader hook。
- [待机功耗](development/engineering/standby-power-contract.zh_CN.md)：生命周期归属、设计依据、失败策略与实机验收。

## 验证与分发

```bash
./tools/validate.sh --static    # 仓库检查与固件 host tests
./tools/validate.sh --firmware  # ESP-IDF 构建与合并镜像验证
./tools/validate.sh --gui       # 桌面端 host tests、构建与打包检查
./tools/validate.sh             # 完整门禁
```

固件验证需要已激活的 ESP-IDF 5.5.3 环境。GUI 验证需要 macOS 工具链及已验证的固件产物，详见[应用指南](../gui/README.zh_CN.md)。编译和 host tests 不能代替硬件验收；剩余检查见[桌面端验收记录](../gui/docs/acceptance.zh_CN.md)和[固件验证记录](go-voice-firmware.zh_CN.md#验证)。

- [第三方声明](THIRD_PARTY_NOTICES.zh_CN.md)、[桌面端依赖声明](../gui/docs/THIRD_PARTY_NOTICES.zh_CN.md)和[资源来源](../assets/README.zh_CN.md)：再分发条款及资源复现方法。分发前仍须取得所保留参考开机音的独立授权。
- [变更记录](CHANGELOG.zh_CN.md)。
- [产品网站](../website/README.zh_CN.md)：`go-voice.vfasky.com` 介绍与下载页面，使用 `cf` CLI 部署。
- [CI 验证](development/ci/CI-validation.zh_CN.md)与[固件发布](development/ci/CI-build-and-release.zh_CN.md)。

## 参与贡献

- [贡献指南](../.github/CONTRIBUTING.zh_CN.md)、[行为准则](../.github/CODE_OF_CONDUCT.zh_CN.md)、[安全](../.github/SECURITY.zh_CN.md)与[支持](../.github/SUPPORT.zh_CN.md)。
- [文档规范](contribution/doc-conventions.zh_CN.md)、[提交与 PR 规范](contribution/commit-and-pr.zh_CN.md)和[开发指南](development/README.zh_CN.md)。
- [Fork 指南](fork-guide.zh_CN.md)、[硬件开发参考经验](reference/README.zh_CN.md)和[上游板卡与产品背景](brand/README.zh_CN.md)。

AI 辅助开发从 [AGENTS.zh_CN.md](../AGENTS.zh_CN.md) 开始，并按其中的任务路由读取资料。
