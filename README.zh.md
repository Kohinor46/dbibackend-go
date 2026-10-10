# DBI Backend (Go)

[English](README.md) | [Русский](README.ru.md) | [Español](README.es.md) | [Italiano](README.it.md) | [Deutsch](README.de.md) | [Français](README.fr.md) | [Português](README.pt.md) | **中文** | [日本語](README.ja.md) | [हिन्दी](README.hi.md) | [العربية](README.ar.md)

一款桌面应用，用于通过 USB 将游戏安装到运行 [DBI](https://github.com/rashevskyv/dbi) 的 Nintendo Switch 上。选择包含 `.nsp` / `.nsz` / `.xci` 文件的文件夹，连接 Switch，即可开始安装。

本项目是 [lunixoid/dbibackend](https://github.com/lunixoid/dbibackend)（一个 Python 脚本）的 Go 重写版，带有图形界面，提供 macOS 和 Windows 的现成构建版本，无需安装 Python 或 libusb。

![DBI Backend](docs/screenshot.png)

## 功能

- **图形界面**：文件夹选择（Finder、资源管理器或拖放）、已找到文件的列表及其大小、连接状态、带传输速度的进度条，以及所有标签页共用的日志面板。日志面板默认折叠，只显示最新一行，需要时可以展开、复制全部日志或保存到文件。
- **无需重启**：DBI 完成后，应用会重新等待 Switch 连接。上次使用的文件夹会被记住；点击 **开始** 后才会开始安装。
- **11 种语言**：英语、俄语、西班牙语、意大利语、德语、法语、葡萄牙语、中文、日语、印地语和阿拉伯语。默认跟随系统语言，也可以在窗口中切换。
- **内置 Windows 驱动安装**：应用检测到未安装驱动的 Switch 时，会提示安装驱动。这取代了使用 Zadig 手动安装的步骤（见下文）。
- **在 macOS 和 Linux 上通过 MTP 管理文件**：浏览 Switch 的存储，上传和下载文件及整个文件夹，删除文件，备份游戏存档，无需 Android File Transfer（见下文）。
- **在所有系统上通过 FTP 管理文件**：在网络中查找 Switch，通过 Wi-Fi 连接 DBI 的 FTP 服务器，无需数据线，即可管理 SD 卡或安装游戏（见下文）。
- **断点续传**：中断的上传会从停下的位置继续，重新连接后也可以。
- **通知**：耗时较长的安装或传输完成后发出通知，启动时还会检查新版本。
- **设置**：浅色或深色外观，以及适用于 MTP 和 FTP 标签页的六种文件视图样式。
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

1. 启动应用，选择存放游戏的文件夹（子文件夹也会包含在内），然后点击 **开始**。
2. 在 Switch 上打开 DBI，选择 **Install title from USB**。
3. 用 USB 数据线将 Switch 连接到电脑。状态会变为 **已连接**。
4. 在 DBI 中选择文件并安装。进度和速度显示在窗口底部。

## Windows：USB 驱动

在 Windows 上，应用只能通过 WinUSB 驱动与 Switch 通信。以前需要使用 [Zadig](https://zadig.akeo.ie/) 手动安装该驱动。

现在应用会自动完成这一步。检测到未安装驱动的 Switch 时，应用会弹出提示：点击 **安装**，并确认 Windows 的管理员权限请求。你也可以随时使用窗口中的 **安装 USB 驱动** 按钮。此时 Switch 必须已连接，并且 DBI 处于 **Install title from USB** 模式。

工作原理：应用为设备分配 Windows 自带、由 Microsoft 签名的 WinUSB 驱动（等同于在设备管理器中执行 **更新驱动程序 → 让我从列表中选取… → WinUsb Device**）。它不会向系统添加任何证书，并且在启用智能应用控制的情况下也能正常工作。

## 通过 MTP 管理文件（macOS 和 Linux）

macOS 没有内置 MTP 支持，因此 DBI 开启 MTP 模式时，Switch 不会出现在 Finder 中；而常用的替代方案 Android File Transfer 也已不再更新。**文件（MTP）** 标签页可以取代它：

1. 在 Switch 上打开 DBI，选择 **Run MTP responder**，然后连接数据线。
2. 在应用中打开 **文件（MTP）** 标签页，点击 **连接**。
3. 选择一个存储（SD 卡、NAND、安装目标、存档等），打开文件夹，然后通过 **上传文件…** 和 **上传文件夹…** 按钮或将文件和文件夹拖到窗口中来上传。你也可以下载和删除文件，以及新建文件夹。

要安装游戏，请将其上传到名称中带有“install”的存储：DBI 会边接收边安装。支持超过 4 GB 的文件。

**上传文件夹时**，它会与 Switch 上的同名文件夹合并（不区分大小写）：同名文件会被替换，Switch 上的其他内容保持不变。`.DS_Store` 等系统文件会被跳过。上传到安装存储时只会发送文件，不会创建文件夹。

**如果上传中断**（数据线脱落、连接断开），会出现一个带 **继续** 按钮的提示栏，重新连接后也会出现。点击后会发送剩余内容：已完整传输的文件会被跳过，中断时正在传输的文件会重新发送。

**下载文件夹**（使用下载按钮或右键菜单）会将其连同所有内容一起复制。

**备份存档**：当 DBI 显示其“Saves”存储时，点击此按钮会将所有游戏存档复制到电脑上新建的“DBI saves <日期>”文件夹中。

![文件（MTP）](docs/screenshot-mtp.png)

**“Switch 正被其他程序占用。”** 在 macOS 上，系统相机服务（`ptpcamerad`）会在 MTP 设备连接后立即将其占用。点击 **释放设备**：应用会停止该服务并接管 Switch；macOS 会在需要时自行重新启动该服务。Android File Transfer 也会占用设备，请先退出它。

在 Windows 上，此标签页会被隐藏：文件资源管理器已经能在 MTP 模式下显示 Switch。

## 通过 FTP 管理文件（所有系统）

DBI 可以通过 Wi-Fi 运行 FTP 服务器。**FTP** 标签页会连接到该服务器，并提供与 MTP 标签页相同的文件管理器，同样支持上传文件夹：

1. 在 Switch 上打开 DBI，选择 **Run FTP server**。屏幕上会显示 Switch 的地址。
2. 在应用中打开 **FTP** 标签页，点击 **查找** 在网络中搜索 Switch（或手动输入地址），选择模式，然后点击 **连接**：
   - **SD 卡（端口 5000）**：浏览 SD 卡，上传文件和文件夹，下载和删除文件，新建文件夹；
   - **安装（端口 6000）**：上传游戏即可安装。

电脑和 Switch 必须连接到同一网络。DBI 的服务器默认无需密码即可登录；如果你设置了密码，请在 **用户名和密码** 中填写。通过 Wi-Fi 传输通常比通过 USB 慢。连接过的地址会保存在地址栏的菜单中。

DBI 的 FTP 服务器无法保存含非拉丁字母的名称（汉字、西里尔字母、带变音符号的字母等）：它会拒绝这些名称，或者保存后无法显示。上传此类文件前，应用会询问是将这些文件和文件夹改用拉丁字母重命名（“Паспорт.pdf” → “Pasport.pdf”），还是跳过它们。

![FTP](docs/screenshot-ftp.png)

## 设置

点击语言选择器旁边的 ⚙ 按钮即可打开设置：

- **常规**：安装或传输超过 10 秒时，完成后发出通知（点击 **测试** 可立即发送一条），以及启动时检查新版本。有新版本时，⚙ 旁边会出现一个显示其版本号的按钮。
- **外观**：跟随系统、浅色或深色。
- **文件视图**（用于 MTP 和 FTP 标签页）：经典、彩色图标、格式标签、表格、双行或平铺。预览会展示每种样式的效果。

无论使用哪种样式，右键点击文件或文件夹都会打开包含相应操作的菜单。

![设置](docs/screenshot-settings.png)

## Linux

安装 udev 规则后，无需 root 权限即可访问 Switch（该规则同时适用于 USB 安装和 MTP 两种模式）：

```bash
sudo cp data/linux/99-dbibackend.rules /etc/udev/rules.d/
sudo udevadm control --reload-rules
```

如果 **文件（MTP）** 标签页提示 Switch 正被其他程序占用，可能是桌面环境自动挂载了它（gvfs）：请在文件管理器中将其卸载。

## 传输速度

速度主要受 Switch 限制，而不是电脑：

- 使用 **USB 2.0**（Switch 默认使用的模式）时，上限约为 **40 MB/s**。对于未压缩的 `.nsp` / `.xci` 文件，应用能让总线约 80% 的时间保持繁忙，平均速度约为 32 MB/s。
- **`.nsz` 文件** 的安装速度更慢：Switch 需要自行解压每个数据块。
- DBI 以最大 1 MB 的分块请求数据，并且只有在处理完上一块后才会请求下一块。电脑无法加快这一过程。
- 通过 **FTP**（Wi-Fi）传输约为 6 MB/s；通过 **MTP**（USB 2.0）约为 27 MB/s。

### USB 3.0

Switch 的接口支持 USB 3.0（5 Gbit/s），但系统将其限制在 USB 2.0 模式。使用 Atmosphère 时，可以在 `atmosphere/config/system_settings.ini` 中启用它，然后重启 Switch：

```ini
[usb]
usb30_force_enabled = u8!0x1
```

数据线也必须支持 USB 3：许多 USB-C 充电线只能以 USB 2.0 传输数据。如果 `Session stats` 中的 `usb_MB/s` 明显超过 40，就说明已经生效。

在初版 Switch 上使用 DBI 902、安装到 Samsung EVO Plus microSD 卡的实测结果：

| 文件 | `usb_MB/s`（数据线传输） | `avg_MB/s`（安装） | `waiting_for_switch` |
|---|---|---|---|
| `.nsp`，10 GB | 246 | **38.9**（USB 2.0 下为 32） | 80% |
| `.nsz`，2.2 GB | 220 | **23.4** | 86% |

数据线不再是瓶颈，但安装速度只提升约 20%：大部分时间都花在 Switch 本身（写入 microSD 卡、解压 `.nsz`）。

USB 3.0 会干扰 2.4 GHz 无线信号：Switch 连接期间，无线 Joy-Con 和 2.4 GHz Wi-Fi 可能会出现延迟。这正是 Nintendo 默认关闭 USB 3.0 的原因。

每次会话结束时，日志会输出一行 `Session stats`：

| 字段 | 含义 |
|---|---|
| `usb_MB/s` | 发送数据期间的速度 |
| `avg_MB/s` | 从第一个请求到最后一个请求的平均速度 |
| `time_data` / `time_handshake` / `waiting_for_switch` | 分别用于发送数据、协议交互和等待 Switch 的时间占比 |
| `active` / `idle` | 安装时间，以及安装前后的空闲时间 |

如果 `waiting_for_switch` 较高，瓶颈就在 Switch（microSD 写入速度、NSZ 解压）。启用 **调试**（位于日志面板中）后，日志会显示 DBI 的每个请求及其大小和速度。

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
| `usbconn` | 在 USB 安装和 MTP 模式下通过 libusb（gousb）打开 Switch，批量（bulk）传输 |
| `mtp` | MTP 客户端（基于 USB 的 PTP）：存储、文件夹、上传和下载、超过 4 GB 的文件 |
| `gui` | Fyne 界面、FTP 客户端 |
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
- [Fyne](https://fyne.io/)（BSD-3-Clause）、[gousb](https://github.com/google/gousb)（Apache-2.0）、[zenity](https://github.com/ncruces/zenity)（MIT）、[jlaffaye/ftp](https://github.com/jlaffaye/ftp)（ISC）。

本项目与 Nintendo 无任何关联。请仅安装你拥有的游戏。
