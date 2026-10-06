# DBI Backend (Go)

[English](README.md) | [Русский](README.ru.md) | [Español](README.es.md) | [Italiano](README.it.md) | [Deutsch](README.de.md) | [Français](README.fr.md) | [Português](README.pt.md) | [中文](README.zh.md) | **日本語** | [हिन्दी](README.hi.md) | [العربية](README.ar.md)

[DBI](https://github.com/rashevskyv/dbi) を実行している Nintendo Switch に、USB 経由でゲームをインストールするためのデスクトップ アプリです。`.nsp` / `.nsz` / `.xci` ファイルのあるフォルダーを選び、Switch を接続すればインストールできます。

[lunixoid/dbibackend](https://github.com/lunixoid/dbibackend)（Python スクリプト）を Go で書き直したもので、グラフィカルなインターフェースと macOS・Windows 向けのビルド済みファイルを備えています。Python や libusb をインストールする必要はありません。

![DBI Backend](docs/screenshot.png)

## 特長

- **グラフィカルなインターフェース**：フォルダーの選択（Finder、エクスプローラー、またはドラッグ＆ドロップ）、見つかったファイルとサイズの一覧、接続状態、転送速度付きの進行状況バー、ログを備えています。
- **再起動は不要**：DBI の処理が終わると、アプリは再び Switch の接続を待ちます。最後に使ったフォルダーを記憶し、起動時にはサーバーが自動的に開始されます。
- **11 言語に対応**：英語、ロシア語、スペイン語、イタリア語、ドイツ語、フランス語、ポルトガル語、中国語、日本語、ヒンディー語、アラビア語。言語はシステムの設定に従い、ウィンドウ内で切り替えることもできます。
- **Windows ドライバーを内蔵**：ドライバーのない Switch を検出すると、アプリがドライバーのインストールを提案します。Zadig を使った手動の手順は不要になりました（後述）。
- **コマンドライン モード**（`-cli`）：オリジナルのスクリプトと同じように動作するため、自動化に使えます。
- **オリジナルより安全**：Switch が要求できるのは、選択したフォルダー内のファイルだけです。

## ダウンロード

ビルド済みのファイルは [Releases](../../releases) ページにあります。

| システム | ファイル | 備考 |
|---|---|---|
| macOS 12 以降（Apple Silicon および Intel） | `DBI Backend.app` | Apple のデベロッパ証明書で署名されていません。初回起動時は、右クリックして **開く** を選ぶか、`xattr -dr com.apple.quarantine "DBI Backend.app"` を実行してください。 |
| Windows 10/11（x64 および ARM64） | `DBI Backend.exe` | 単一のファイルで、ほかにインストールするものはありません。USB ドライバーはアプリからインストールします（後述）。 |
| Linux | ソースからビルド | [ビルド](#ビルド) を参照してください。 |

## 使い方

1. アプリを起動し、ゲームが入ったフォルダーを選択します。サブフォルダーも対象になります。
2. Switch で DBI を開き、**Install title from USB** を選択します。
3. USB ケーブルで Switch をコンピューターに接続します。状態が **接続済み** に変わります。
4. DBI でファイルを選んでインストールします。進行状況と速度はウィンドウの下部に表示されます。

## Windows: USB ドライバー

Windows では、アプリは WinUSB ドライバーを介してのみ Switch と通信できます。以前は [Zadig](https://zadig.akeo.ie/) を使って手動でインストールする必要がありました。

現在はアプリが自動で行います。ドライバーのない Switch を検出するとメッセージが表示されるので、**インストール** をクリックし、Windows の管理者権限の確認を承認してください。ウィンドウの **USB ドライバーをインストール** ボタンを使えば、いつでもインストールできます。その際は、Switch が接続され、DBI が **Install title from USB** モードになっている必要があります。

仕組み：アプリは、Windows に標準で含まれ Microsoft の署名が付いた WinUSB ドライバーをデバイスに割り当てます（デバイス マネージャーで **ドライバーの更新 → コンピューター上の利用可能なドライバーの一覧から選択します → WinUsb デバイス** を選ぶのと同じ操作です。英語版 Windows では Update driver → Let me pick → WinUsb Device）。システムに証明書を追加することはなく、スマート アプリ コントロール（Smart App Control）が有効な環境でも動作します。

## Linux

udev ルールをインストールすると、root 権限なしで Switch にアクセスできるようになります。

```bash
sudo cp data/linux/99-dbibackend.rules /etc/udev/rules.d/
sudo udevadm control --reload-rules
```

## 転送速度

速度を制限しているのは、主にコンピューターではなく Switch 側です。

- **USB 2.0** での上限は約 **40 MB/s** です。非圧縮の `.nsp` / `.xci` ファイルでは、アプリはバスを約 80% の時間使用し続け、平均で約 32 MB/s に達します。
- **`.nsz` ファイル** はインストールに約 3 倍の時間がかかります。Switch がすべてのブロックを自分で展開するためです。
- DBI はデータを最大 1 MB ずつ要求し、前のデータを処理し終えてから次を要求します。コンピューター側でこれを速くすることはできません。

各セッションの終了時に、ログに `Session stats` 行が出力されます。

| フィールド | 意味 |
|---|---|
| `usb_MB/s` | データ送信中の速度 |
| `avg_MB/s` | 最初の要求から最後の要求までの平均速度 |
| `time_data` / `time_handshake` / `waiting_for_switch` | データ送信、プロトコルのやり取り、Switch の待機にそれぞれ費やした時間の割合 |
| `active` / `idle` | インストールにかかった時間と、その前後のアイドル時間 |

`waiting_for_switch` の値が大きい場合、ボトルネックは Switch 側（microSD の書き込み速度、NSZ の展開）にあります。**デバッグ** を有効にすると、DBI からの各要求がサイズと速度とともにログに表示されます。

## コマンドライン モード

```bash
dbibackend -cli ~/Switch          # wait for the Switch, serve one session, exit
dbibackend -cli -debug ~/Switch   # the same with a detailed log
dbibackend ~/Switch               # open the window with this folder
dbibackend -install-driver        # Windows: install the WinUSB driver for the connected Switch
```

macOS では、Switch を接続したときにサーバーを自動で起動させることもできます。`data/darwin/com.dbibackend.usb.agent.plist` 内のパスを編集し、`~/Library/LaunchAgents/` にコピーしてください。

## ビルド

Go 1.25 以降と C コンパイラが必要です（libusb と Fyne には cgo が必要なため）。

```bash
# macOS
brew install libusb pkg-config
# Debian/Ubuntu (plus the Fyne dependencies: https://docs.fyne.io/started/)
sudo apt install libusb-1.0-0-dev pkg-config

make build        # ./dbibackend for the current system
make test         # tests
make release      # dist/DBI Backend.app and dist/DBI Backend.exe
```

`make release` は macOS で実行します。libusb をソースからビルドして静的リンクするため、生成されたアプリは何もインストールせずに動作します。Windows 版をビルドするには、さらに `brew install mingw-w64` と `fyne` ツール（`go install fyne.io/tools/cmd/fyne@latest`）が必要です。

## プロジェクト構成

| パッケージ | 役割 |
|---|---|
| `dbi` | DBI0 プロトコル（LIST / FILE_RANGE / EXIT）、フォルダーのスキャン、セッション統計 |
| `usbconn` | libusb（gousb）による Switch のオープン、バルク転送 |
| `gui` | Fyne によるインターフェース |
| `i18n` | 翻訳（`i18n/locales/*.json`） |
| `winusb` | Windows への WinUSB のインストール（SetupAPI） |
| `cmd/winusb-helper` | ドライバーのインストールに使う、Windows ARM64 ネイティブのヘルパー |

翻訳を追加・修正するには、`i18n/locales/<code>.json` を編集してください。`go test ./i18n/` で、すべての言語にすべてのキーがそろっているかを確認できます。

## オリジナル版との違い

- Switch が要求できるのは、選択したフォルダー内のファイルだけです。オリジナル版は、デバイスから送られてきたパスを何でも開いていました。
- 拡張子フィルターを修正しました。オリジナル版は `nsz` をドットなしで照合していました。
- 異なるサブフォルダーに同じ名前のファイルがある場合は、最初に見つかったものが使われます（DBI からはファイル名しか見えないため）。

## 謝辞とライセンス

- **このプロジェクト**：[MIT](LICENSE)。
- [lunixoid/dbibackend](https://github.com/lunixoid/dbibackend)（MIT）：プロトコルのオリジナル実装。
- [DBI](https://github.com/rashevskyv/dbi)（作者：duckbill）：Switch 用のインストーラー。
- [libusb](https://libusb.info/)（LGPL-2.1）：ビルドに静的リンクされています。本プロジェクトのソースコードは公開されているため、任意のバージョンの libusb で再ビルドできます。
- [Fyne](https://fyne.io/)（BSD-3-Clause）、[gousb](https://github.com/google/gousb)（Apache-2.0）、[zenity](https://github.com/ncruces/zenity)（MIT）。

本プロジェクトは Nintendo とは一切関係ありません。インストールするのは、ご自身が所有しているゲームだけにしてください。
