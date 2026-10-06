# DBI Backend (Go)

[English](README.md) | [Русский](README.ru.md) | [Español](README.es.md) | [Italiano](README.it.md) | [Deutsch](README.de.md) | [Français](README.fr.md) | [Português](README.pt.md) | **中文** | [日本語](README.ja.md) | [हिन्दी](README.hi.md) | [العربية](README.ar.md)

一款桌面应用，用于通过 USB 将游戏安装到运行 [DBI](https://github.com/rashevskyv/dbi) 的 Nintendo Switch 上。选择包含 `.nsp` / `.nsz` / `.xci` 文件的文件夹，连接 Switch，即可开始安装。

本项目是 [lunixoid/dbibackend](https://github.com/lunixoid/dbibackend)（一个 Python 脚本）的 Go 重写版，带有图形界面，提供 macOS 和 Windows 的现成构建版本，无需安装 Python 或 libusb。

![DBI Backend](docs/screenshot.png)

## 功能

- **图形界面**：文件夹选择（Finder、资源管理器或拖放）、已找到文件的列表及其大小、连接状态、带传输速度的进度条，以及日志。
- **无需重启**：DBI 完成后，应用会重新等待 Switch 连接。上次使用的文件夹会被记住，启动时服务器会自动运行。
- **11 种语言**：英语、俄语、西班牙语、意大利语、德语、法语、葡萄牙语、中文、日语、印地语和阿拉伯语。默认跟随系统语言，也可以在窗口中切换。
- **内置 Windows 驱动安装**：应用检测到未安装驱动的 Switch 时，会提示安装驱动。这取代了使用 Zadig 手动安装的步骤（见下文）。
- **命令行模式**（`-cli`）：行为与原始脚本一致，便于自动化。
- **比原版更安全**：Switch 只能请求所选文件夹中的文件。

## 下载

现成的构建版本可在 [Releases](../../releases) 页面获取。

| 系统 | 文件 | 说明 |
|---|---|---|
| macOS 12+（Apple Silicon 和 Intel） | `DBI Backend.app` | 未使用 Apple 开发者证书签名。首次启动时：右键点击 → **打开**，或运行 `xattr -dr com.apple.quarantine "DBI Backend.app"`。 |
| Windows 10/11（x64 和 ARM64） | `DBI Backend.exe` | 单个文件，无需安装任何其他组件。USB 驱动可在应用内安装（见下文）。 |
| Linux | 从源码构建 | 参见[构建](#构建)。 |

## 使用方法

1. 启动应用，选择存放游戏的文件夹。子文件夹也会包含在内。
2. 在 Switch 上打开 DBI，选择 **Install title from USB**。
3. 用 USB 数据线将 Switch 连接到电脑。状态会变为 **已连接**。
4. 在 DBI 中选择文件并安装。进度和速度显示在窗口底部。

## Windows：USB 驱动

在 Windows 上，应用只能通过 WinUSB 驱动与 Switch 通信。以前需要使用 [Zadig](https://zadig.akeo.ie/) 手动安装该驱动。

现在应用会自动完成这一步。检测到未安装驱动的 Switch 时，应用会弹出提示：点击 **安装**，并确认 Windows 的管理员权限请求。你也可以随时使用窗口中的 **安装 USB 驱动** 按钮。此时 Switch 必须已连接，并且 DBI 处于 **Install title from USB** 模式。

工作原理：应用为设备分配 Windows 自带、由 Microsoft 签名的 WinUSB 驱动（等同于在设备管理器中执行 **更新驱动程序 → 让我从列表中选取… → WinUsb Device**）。它不会向系统添加任何证书，并且在启用智能应用控制的情况下也能正常工作。

## Linux

安装 udev 规则后，无需 root 权限即可访问 Switch：

```bash
sudo cp data/linux/99-dbibackend.rules /etc/udev/rules.d/
sudo udevadm control --reload-rules
```

## 传输速度

速度主要受 Switch 限制，而不是电脑：

- 使用 **USB 2.0** 时，上限约为 **40 MB/s**。对于未压缩的 `.nsp` / `.xci` 文件，应用能让总线约 80% 的时间保持繁忙，平均速度约为 32 MB/s。
- **`.nsz` 文件** 的安装速度约慢三倍：Switch 需要自行解压每个数据块。
- DBI 以最大 1 MB 的分块请求数据，并且只有在处理完上一块后才会请求下一块。电脑无法加快这一过程。

每次会话结束时，日志会输出一行 `Session stats`：

| 字段 | 含义 |
|---|---|
| `usb_MB/s` | 发送数据期间的速度 |
| `avg_MB/s` | 从第一个请求到最后一个请求的平均速度 |
| `time_data` / `time_handshake` / `waiting_for_switch` | 分别用于发送数据、协议交互和等待 Switch 的时间占比 |
| `active` / `idle` | 安装时间，以及安装前后的空闲时间 |

如果 `waiting_for_switch` 较高，瓶颈就在 Switch（microSD 写入速度、NSZ 解压）。启用 **调试** 后，日志会显示 DBI 的每个请求及其大小和速度。

## 命令行模式

```bash
dbibackend -cli ~/Switch          # wait for the Switch, serve one session, exit
dbibackend -cli -debug ~/Switch   # the same with a detailed log
dbibackend ~/Switch               # open the window with this folder
dbibackend -install-driver        # Windows: install the WinUSB driver for the connected Switch
```

在 macOS 上，可以让服务器在连接 Switch 时自动启动：修改 `data/darwin/com.dbibackend.usb.agent.plist` 中的路径，然后将该文件复制到 `~/Library/LaunchAgents/`。

## 构建

需要 Go 1.25+ 和 C 编译器（libusb 和 Fyne 依赖 cgo）。

```bash
# macOS
brew install libusb pkg-config
# Debian/Ubuntu (plus the Fyne dependencies: https://docs.fyne.io/started/)
sudo apt install libusb-1.0-0-dev pkg-config

make build        # ./dbibackend for the current system
make test         # tests
make release      # dist/DBI Backend.app and dist/DBI Backend.exe
```

`make release` 需在 macOS 上运行。它会从源码构建 libusb 并静态链接，因此生成的程序无需安装任何依赖。构建 Windows 版本还需要 `brew install mingw-w64` 和 `fyne` 工具（`go install fyne.io/tools/cmd/fyne@latest`）。

## 项目结构

| 包 | 用途 |
|---|---|
| `dbi` | DBI0 协议（LIST / FILE_RANGE / EXIT）、文件夹扫描、会话统计 |
| `usbconn` | 通过 libusb（gousb）打开 Switch，批量（bulk）传输 |
| `gui` | Fyne 界面 |
| `i18n` | 翻译（`i18n/locales/*.json`） |
| `winusb` | 在 Windows 上安装 WinUSB（SetupAPI） |
| `cmd/winusb-helper` | 用于安装驱动的原生 ARM64 Windows 辅助程序 |

如需添加或修正翻译，请编辑 `i18n/locales/<code>.json`。`go test ./i18n/` 会检查每种语言是否包含所有键。

## 与原版的区别

- Switch 只能请求所选文件夹中的文件。原版会打开设备发送的任意路径。
- 修复了扩展名过滤：原版匹配的是不带点的 `nsz`。
- 如果不同子文件夹中有同名文件，将使用最先找到的那个（DBI 只能看到文件名）。

## 致谢与许可证

- **本项目**：[MIT](LICENSE)。
- [lunixoid/dbibackend](https://github.com/lunixoid/dbibackend)（MIT）：协议的原始实现。
- [DBI](https://github.com/rashevskyv/dbi)（作者 duckbill）：Switch 上的安装程序。
- [libusb](https://libusb.info/)（LGPL-2.1）以静态方式链接到构建版本中。本项目源代码开放，因此你可以使用自己的 libusb 版本重新构建。
- [Fyne](https://fyne.io/)（BSD-3-Clause）、[gousb](https://github.com/google/gousb)（Apache-2.0）、[zenity](https://github.com/ncruces/zenity)（MIT）。

本项目与 Nintendo 无任何关联。请仅安装你拥有的游戏。
