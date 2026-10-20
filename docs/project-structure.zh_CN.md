简体中文 · [English](project-structure.md)

# 项目结构

仓库根目录构建 AI Passport 的 ESP-IDF 固件，`gui/` 是基于 MyGo 的 macOS 应用的独立 Go module。两套构建各自独立，无须嵌套或移动项目。

```text
main/                         固件应用、BLE 音频、按键与状态界面
components/bsp/               可复用板级驱动与公开硬件接口
bootloader_components/        永久 Recovery 的 bootloader hook
assets/                       品牌图稿、固件图片、字体与开机音
sdkconfig.defaults            ESP32-C3、蓝牙与功耗默认配置
partitions.csv                应用、设备身份与 Recovery 保护布局
dependencies.lock             固定 ESP-IDF managed components 版本
tests/                        固件 host tests 与 ESP-IDF/LVGL 测试替身
tools/                        统一检查、固件校验与资源生成脚本
gui/
  main.go                     桌面端入口
  go.mod, go.sum              独立 Go module 与锁定依赖
  mygo.json                   应用标识与打包配置
  internal/
    app/                      服务组合、helper 生命周期与 UI 事件分发
    device/                   BLE 会话协调与取消
    protocol/                 控制消息校验、ADPCM、分片与流排序
    vocabulary/               持久化词条、来源与用户锁定
    refinement/               本地/远端配置、保真检查与回退
    firmware/                 USB helper 协议与安装器进程管理
    permission/               权限状态与引导
    platform/bridge/          原生 helper 进程与有界消息传输
    platform/keychain/        macOS Keychain 访问
    ui/                       原生页面、权限流程、HUD 与波形
  macos/
    BluetoothHelper/          CoreBluetooth 子进程
    SpeechBridge/             Speech、FoundationModels 与 TextInjector 子进程
    BridgeCore/               原生共享消息与权限契约
    Tests/                    原生契约测试
  firmware-helper/            Rust 应用分区 USB 安装器，固定 espflash 依赖
  resources/                  桌面与菜单栏图标
  scripts/                    构建、开发启动、打包与验证
  docs/                       桌面端验收与依赖声明
website/                      介绍与下载网站（Cloudflare Workers 静态资源）
vokie-plugin/                 导入的上游参考，不参与桌面端运行
docs/                         产品、协议、硬件与开发文档
.github/                      CI、Issue 表单与社区文档
LICENSE, LICENSES/            项目与第三方许可证及声明
```

## 职责归属

`main/vokie_ble.c` 负责工牌音频会话与 BLE 传输；内部保留 `vokie_` 命名以沿用导入代码结构，对外标识为 Go Voice。硬件事实属于 BSP 头文件，尤其是 `components/bsp/include/bsp_pins.h`；应用状态、按键、动画和任务属于 `main/`。

Mac 侧由 `internal/app` 组合服务。设备协调器负责会话标识、取消和终态，纯 Go 协议层负责校验与解码音频。UI 发出操作意图并在主线程接收更新，不持有 helper 或 Speech 会话。Swift 提供 CoreBluetooth 与端侧 Speech，Rust 提供 USB 固件安装器。凭据仅存 Keychain；音频和转录状态只在内存中保留，没有转录历史模块。

固件与桌面端共同遵循权威 [BLE V1 协议](ai-passport-ble-protocol.zh_CN.md)。不要在 GUI 文档中另建 UUID 或帧格式规范。

## 构建产物

- `build/FoloToy-AI-Passport-full.bin`：已验证的合并固件，用于文档规定的小程序/Recovery 流程。
- `build/FoloToy-AI-Passport.bin`：已验证的应用镜像，用于桌面端打包。
- `gui/build/debug/darwin-arm64/go-voice-debug.app`：开发版桌面应用包，使用独立的 `.debug` 权限身份，内含签名原生 helper 与仅应用分区的固件。

构建目录、暂存 helper、临时固件打包内容和生成图片实验均排除在版本控制之外。相关操作见[构建与测试](development/engineering/build-and-test.zh_CN.md)、[桌面端构建指南](../gui/README.zh_CN.md#构建与验证)和[系统架构](passport-audio-input-architecture.zh_CN.md)。
