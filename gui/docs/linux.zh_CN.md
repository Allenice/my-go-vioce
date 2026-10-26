简体中文 · [English](linux.md)

# Linux 桌面预览版

此源码构建预览版面向 Ubuntu/Debian 的 amd64、arm64 桌面。AI Passport
工牌继续使用现有 Go Voice 固件。Linux 宿主沿用相同的 UUID、BLE V1 帧格式、
Go 解码、会话限制、词库、在线识别和远程润色实现。

当前还不是经过真机验证的 Linux 正式版。主机测试和 helper 交叉编译无法验证蓝牙适配器、
BlueZ 权限、桌面焦点、系统钥匙环或真实识别服务。正式用于语音输入前，请在目标电脑上
完成下方验收清单。

## 环境要求

从 [Go 官方发行页面](https://go.dev/dl/)安装 **Go 1.27.1**，再检查 `go version`。
发行版自带 Go 可能较旧。gojieba 使用 CGO，构建需要本机 C/C++ 编译器，应在 Linux 上运行。
不需要 Bun、Xcode、Swift、Rust、ESP-IDF，也不需要重新编译工牌固件。

Ubuntu/Debian 桌面安装依赖：

```bash
sudo apt update
sudo apt install build-essential pkg-config libgtk-3-dev libgl1 libepoxy0 \
  libayatana-appindicator3-1 bluez dbus-user-session \
  libsecret-tools gnome-keyring xclip xdotool wl-clipboard
```

使用普通用户登录的桌面会话，打开蓝牙，并确保 Secret Service 钥匙环正在运行且已解锁。
GNOME 通常会管理登录钥匙环；其他桌面需提供兼容 Secret Service 的钥匙环。
不要通过 `sudo` 启动应用。本实现不需要额外的蓝牙 capability、系统级 D-Bus 策略或
`uinput` 权限；若发行版策略拒绝 BlueZ 访问，应通过桌面正常的蓝牙配置处理。

## 构建与安装

在仓库根目录运行：

```bash
./tools/validate.sh --static
./tools/validate.sh --gui
```

第二条命令会选择 Linux 门禁：Go 测试、vet、本机构建、包内词典验证和压缩包生成。
只构建可执行文件与安装包时：

```bash
cd gui
./scripts/build-linux.sh
```

amd64 产物为 `gui/build/linux/go-voice-linux-amd64/` 和相邻的
`go-voice-linux-amd64.tar.gz`；arm64 使用对应后缀。在仓库根目录运行：

```bash
./gui/build/linux/go-voice-linux-amd64/launch.sh
# 可选：复制到当前用户的应用目录，并添加桌面菜单入口。
./gui/build/linux/go-voice-linux-amd64/install.sh
```

arm64 请替换后缀。保留整个目录，包括 `go-voice`、`native/`、`resources/`、
`licenses/`、`docs/`。安装脚本复制到 `${XDG_DATA_HOME:-$HOME/.local/share}/go-voice`，
并在同级 `applications` 目录写入桌面入口。应在与目标相同版本的发行版，或更旧且兼容的
发行版上构建：CGO 产物依赖目标系统的 libc/C++ 运行库。
此预览版暂不提供自动更新或 macOS 开发监听；更新时先关闭应用，再构建并运行安装脚本。
安装后从桌面应用菜单启动，「构建产物目录」只是构建输出。「通用」页可开「登录时打开」
（为当前运行的可执行文件写 XDG 自启项），并在「运行版本」中显示正在运行的版本号和目录，
可据此区分构建目录与安装版；窗口隐藏时再次启动（例如点菜单项）会重新显示运行中的窗口。

仓库完整的 `./tools/validate.sh` 还会编译固件，仍需 ESP-IDF 5.5.3；
仅打包 Linux 应用端不需要运行完整固件门禁。

## 首次使用

1. 启动应用，打开系统蓝牙，扫描并明确选择工牌。按 Go Voice 服务筛选设备，保存已选的
   BlueZ 设备路径用于重连；不会替你选择扫描到的第一个工牌。
2. 在“语音识别”页选择在线服务商，输入 API key 并保存。初始未选择状态不会向任何服务
   发送音频。Linux 不提供 Apple 本地识别；请求失败不会切换服务商。
3. 保持 AI 润色关闭，或明确配置 OpenAI 兼容端点。只有所选在线识别服务接收音频；
   开启的远程润色接收转写和相关词库。不保存音频或转写历史。
4. 聚焦目标输入框，通过 UP 键开始、结束语音，然后检查结果。

设置和词库保存在 `${XDG_CONFIG_HOME:-$HOME/.config}/MyGo-GoVoice/`。
凭据通过 `secret-tool` 存入 Secret Service，密钥只经 stdin 传入。钥匙环缺失或锁定时
会报错，不回退到明文存储。“已保存 key”提示只搜索元数据，不读取密钥。

## 桌面行为与限制

| 能力 | X11 | Wayland |
| --- | --- | --- |
| 剪贴板 | `xclip` | `wl-copy` |
| 自动粘贴 | 使用 `xdotool`，仅原目标窗口仍聚焦时执行；终端用 Ctrl+Shift+V | 复制后手动粘贴 |
| 工牌回车、删除、清空 | 使用 `xdotool` 发往当前聚焦应用 | 暂未实现 |
| Apple 本地模型 | 不可用 | 不可用 |
| USB 固件安装 | 不可用 | 不可用 |

输出后文字保留在剪贴板，此预览版不恢复之前的剪贴板内容。X11 下若说话期间切换了窗口，
只复制结果，不切回窗口或自动粘贴。终端读取 Ctrl+V 为控制字符而非粘贴，因此识别到
终端窗口（WM_CLASS，经 `xprop` 或新版 `xdotool` 读取，如 ghostty、gnome-terminal、
konsole、xterm、kitty、alacritty、wezterm）时改用 Ctrl+Shift+V。其他粘贴快捷键仍不是
Ctrl+V 的应用可能需要手动粘贴。Wayland 也需手动粘贴，XWayland 不会让 X11 输入方式
适用于原生 Wayland 应用。缺少 `xdotool` 时降级为剪贴板输出；若桌面不显示托盘图标，
请保持窗口打开。

蓝牙 helper 使用 BlueZ D-Bus、带响应的 Control 写入，以及 Audio 的 `AcquireNotify`。
要求协商后的 MTU 至少为 185，验证 info 中的 `hello`，订阅 Control 和 Audio 后才发送
`host_ready`，并复用有界帧组装器。不改变适配器电源、不配对设备、不安装系统策略，也不
修改固件。BLE 安全限制仍遵循现有[协议](../../docs/ai-passport-ble-protocol.zh_CN.md)。

## 在 Linux 电脑上的验收

- 记录发行版、CPU 架构、`echo "$XDG_SESSION_TYPE"`、`go version`、`bluetoothctl --version`。
- 构建压缩包，分别启动解压目录和已安装的桌面入口；移除构建目录或模块缓存后确认词典仍可用。
- 检查明确选择设备、握手、多次录音、取消、断线重连及录音中退出。
- 检查 key 保存、复用和删除、钥匙环锁定、缺少服务商凭据及服务失败；真实服务测试可能计费。
- 检查 X11 原窗口粘贴、切换窗口后只复制，或 Wayland 复制与手动粘贴；还需检查缺少剪贴板工具
  及粘贴快捷键不是 Ctrl+V 的目标应用。
- 确认 Linux 页面不会要求 Apple 权限或提供 USB 写入。

构建、主机测试与设备测试分别记录。交叉编译或模拟 helper 测试不代表 Linux 真机验证成功。
