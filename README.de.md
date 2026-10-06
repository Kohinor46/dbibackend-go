# DBI Backend (Go)

[English](README.md) | [Русский](README.ru.md) | [Español](README.es.md) | [Italiano](README.it.md) | **Deutsch** | [Français](README.fr.md) | [Português](README.pt.md) | [中文](README.zh.md) | [日本語](README.ja.md) | [हिन्दी](README.hi.md) | [العربية](README.ar.md)

Eine Desktop-App, mit der Sie Spiele per USB auf eine Nintendo Switch mit [DBI](https://github.com/rashevskyv/dbi) installieren. Wählen Sie einen Ordner mit `.nsp`-, `.nsz`- oder `.xci`-Dateien, schließen Sie die Switch an und installieren Sie.

Es handelt sich um eine Neuimplementierung von [lunixoid/dbibackend](https://github.com/lunixoid/dbibackend) (einem Python-Skript) in Go – mit grafischer Oberfläche, fertigen Builds für macOS und Windows und ohne dass Python oder libusb installiert werden müssen.

![DBI Backend](docs/screenshot.png)

## Funktionen

- **Grafische Oberfläche**: Ordnerauswahl (Finder, Explorer oder Drag & Drop), Liste der gefundenen Dateien mit Größe, Verbindungsstatus, Fortschrittsbalken mit Übertragungsgeschwindigkeit und ein Protokoll.
- **Kein Neustart nötig**: Wenn DBI fertig ist, wartet die App erneut auf die Switch. Der zuletzt verwendete Ordner wird gespeichert, und der Server startet beim Öffnen der App automatisch.
- **11 Sprachen**: Englisch, Russisch, Spanisch, Italienisch, Deutsch, Französisch, Portugiesisch, Chinesisch, Japanisch, Hindi und Arabisch. Die Sprache richtet sich nach dem System und lässt sich im Fenster ändern.
- **Integrierter Windows-Treiber**: Erkennt die App die Switch ohne Treiber, bietet sie an, ihn zu installieren. Das ersetzt den manuellen Schritt mit Zadig (siehe unten).
- **Kommandozeilenmodus** (`-cli`), der sich wie das Original-Skript verhält – für die Automatisierung.
- **Sicherer als das Original**: Die Switch kann nur Dateien aus dem gewählten Ordner anfordern.

## Download

Fertige Builds finden Sie auf der Seite [Releases](../../releases).

| System | Datei | Hinweise |
|---|---|---|
| macOS 12+ (Apple Silicon und Intel) | `DBI Backend.app` | Nicht mit einem Apple-Entwicklerzertifikat signiert. Beim ersten Start: Rechtsklick → **Öffnen** oder `xattr -dr com.apple.quarantine "DBI Backend.app"` ausführen. |
| Windows 10/11 (x64 und ARM64) | `DBI Backend.exe` | Eine einzige Datei, sonst muss nichts installiert werden. Der USB-Treiber wird aus der App heraus installiert (siehe unten). |
| Linux | aus dem Quellcode kompilieren | Siehe [Kompilieren](#kompilieren). |

## Verwendung

1. Starten Sie die App und wählen Sie den Ordner mit Ihren Spielen. Unterordner werden einbezogen.
2. Öffnen Sie auf der Switch DBI und wählen Sie **Install title from USB**.
3. Verbinden Sie die Switch per USB-Kabel mit dem Computer. Der Status wechselt zu **Verbunden**.
4. Wählen Sie die Dateien in DBI aus und installieren Sie sie. Fortschritt und Geschwindigkeit werden unten im Fenster angezeigt.

## Windows: USB-Treiber

Unter Windows kann die App nur über den WinUSB-Treiber mit der Switch kommunizieren. Bisher mussten Sie ihn manuell mit [Zadig](https://zadig.akeo.ie/) installieren.

Jetzt übernimmt das die App selbst. Erkennt sie die Switch ohne Treiber, zeigt sie eine Abfrage an: Klicken Sie auf **Installieren** und bestätigen Sie die Administratoranfrage von Windows. Sie können auch jederzeit die Schaltfläche **USB-Treiber installieren** im Fenster verwenden. Die Switch muss angeschlossen sein und DBI sich im Modus **Install title from USB** befinden.

So funktioniert es: Die App weist den WinUSB-Treiber zu, der mit Windows ausgeliefert wird und von Microsoft signiert ist (genau wie **Treiber aktualisieren → Aus einer Liste verfügbarer Treiber auf meinem Computer auswählen → WinUsb Device** im Geräte-Manager). Es werden keine Zertifikate zum System hinzugefügt, und es funktioniert auch bei aktivierter Intelligenter App-Steuerung (Smart App Control).

## Linux

Installieren Sie die udev-Regel, um ohne root auf die Switch zugreifen zu können:

```bash
sudo cp data/linux/99-dbibackend.rules /etc/udev/rules.d/
sudo udevadm control --reload-rules
```

## Übertragungsgeschwindigkeit

Die Geschwindigkeit wird hauptsächlich durch die Switch begrenzt, nicht durch den Computer:

- Über **USB 2.0** liegt die Obergrenze bei etwa **40 MB/s**. Bei unkomprimierten `.nsp`- / `.xci`-Dateien hält die App den Bus etwa 80 % der Zeit ausgelastet und erreicht durchschnittlich etwa 32 MB/s.
- **`.nsz`-Dateien** werden etwa dreimal langsamer installiert: Die Switch entpackt jeden Block selbst.
- DBI fordert Daten in Blöcken von bis zu 1 MB an und verlangt den nächsten erst, wenn der vorherige verarbeitet ist. Der Computer kann das nicht beschleunigen.

Am Ende jeder Sitzung zeigt das Protokoll eine Zeile `Session stats`:

| Feld | Bedeutung |
|---|---|
| `usb_MB/s` | Geschwindigkeit während der Datenübertragung |
| `avg_MB/s` | Durchschnittsgeschwindigkeit von der ersten bis zur letzten Anfrage |
| `time_data` / `time_handshake` / `waiting_for_switch` | Zeitanteil für das Senden von Daten, für den Protokollaustausch und für das Warten auf die Switch |
| `active` / `idle` | Installationszeit sowie Leerlaufzeit davor und danach |

Ist `waiting_for_switch` hoch, liegt der Engpass bei der Switch (Schreibgeschwindigkeit der microSD, NSZ-Entpacken). Ist **Debug** aktiviert, zeigt das Protokoll jede DBI-Anfrage mit Größe und Geschwindigkeit.

## Kommandozeilenmodus

```bash
dbibackend -cli ~/Switch          # wait for the Switch, serve one session, exit
dbibackend -cli -debug ~/Switch   # the same with a detailed log
dbibackend ~/Switch               # open the window with this folder
dbibackend -install-driver        # Windows: install the WinUSB driver for the connected Switch
```

Unter macOS kann der Server automatisch starten, sobald die Switch angeschlossen wird: Passen Sie den Pfad in `data/darwin/com.dbibackend.usb.agent.plist` an und kopieren Sie die Datei nach `~/Library/LaunchAgents/`.

## Kompilieren

Sie benötigen Go 1.25+ und einen C-Compiler (libusb und Fyne erfordern cgo).

```bash
# macOS
brew install libusb pkg-config
# Debian/Ubuntu (plus the Fyne dependencies: https://docs.fyne.io/started/)
sudo apt install libusb-1.0-0-dev pkg-config

make build        # ./dbibackend for the current system
make test         # tests
make release      # dist/DBI Backend.app and dist/DBI Backend.exe
```

`make release` läuft unter macOS. Es kompiliert libusb aus dem Quellcode und linkt es statisch, sodass das Ergebnis keine installierten Abhängigkeiten benötigt. Für den Windows-Build werden zusätzlich `brew install mingw-w64` und das Tool `fyne` (`go install fyne.io/tools/cmd/fyne@latest`) benötigt.

## Projektstruktur

| Paket | Zweck |
|---|---|
| `dbi` | DBI0-Protokoll (LIST / FILE_RANGE / EXIT), Ordnerscan, Sitzungsstatistik |
| `usbconn` | Öffnen der Switch über libusb (gousb), Bulk-Transfers |
| `gui` | Fyne-Oberfläche |
| `i18n` | Übersetzungen (`i18n/locales/*.json`) |
| `winusb` | Installation von WinUSB unter Windows (SetupAPI) |
| `cmd/winusb-helper` | Nativer ARM64-Helfer für Windows zur Treiberinstallation |

Um eine Übersetzung hinzuzufügen oder zu korrigieren, bearbeiten Sie `i18n/locales/<code>.json`. `go test ./i18n/` prüft, ob jede Sprache alle Schlüssel enthält.

## Unterschiede zum Original

- Die Switch kann nur Dateien aus dem gewählten Ordner anfordern. Das Original öffnete jeden Pfad, den das Gerät sendete.
- Der Erweiterungsfilter wurde korrigiert: Das Original prüfte `nsz` ohne Punkt.
- Haben Dateien in verschiedenen Unterordnern denselben Namen, wird die zuerst gefundene verwendet (DBI sieht nur die Dateinamen).

## Danksagungen und Lizenzen

- **Dieses Projekt**: [MIT](LICENSE).
- [lunixoid/dbibackend](https://github.com/lunixoid/dbibackend) (MIT): die ursprüngliche Implementierung des Protokolls.
- [DBI](https://github.com/rashevskyv/dbi) von duckbill: der Installer auf der Switch.
- [libusb](https://libusb.info/) (LGPL-2.1) ist statisch in die Builds gelinkt. Der Quellcode dieses Projekts ist offen, sodass Sie es mit Ihrer eigenen libusb-Version neu kompilieren können.
- [Fyne](https://fyne.io/) (BSD-3-Clause), [gousb](https://github.com/google/gousb) (Apache-2.0), [zenity](https://github.com/ncruces/zenity) (MIT).

Dieses Projekt steht in keiner Verbindung zu Nintendo. Installieren Sie nur Spiele, die Sie besitzen.
