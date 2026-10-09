# DBI Backend (Go)

[English](README.md) | [Русский](README.ru.md) | [Español](README.es.md) | [Italiano](README.it.md) | **Deutsch** | [Français](README.fr.md) | [Português](README.pt.md) | [中文](README.zh.md) | [日本語](README.ja.md) | [हिन्दी](README.hi.md) | [العربية](README.ar.md)

Eine Desktop-App, mit der Sie Spiele per USB auf eine Nintendo Switch mit [DBI](https://github.com/rashevskyv/dbi) installieren. Wählen Sie einen Ordner mit `.nsp`-, `.nsz`- oder `.xci`-Dateien, schließen Sie die Switch an und installieren Sie.

Es handelt sich um eine Neuimplementierung von [lunixoid/dbibackend](https://github.com/lunixoid/dbibackend) (einem Python-Skript) in Go – mit grafischer Oberfläche, fertigen Builds für macOS und Windows und ohne dass Python oder libusb installiert werden müssen.

![DBI Backend](docs/screenshot.png)

## Funktionen

- **Grafische Oberfläche**: Ordnerauswahl (Finder, Explorer oder Drag & Drop), Liste der gefundenen Dateien mit Größe, Verbindungsstatus, Fortschrittsbalken mit Übertragungsgeschwindigkeit und ein Protokollbereich, den sich alle Tabs teilen. Der Bereich ist auf die letzte Zeile eingeklappt und lässt sich bei Bedarf aufklappen, komplett kopieren oder in einer Datei speichern.
- **Kein Neustart nötig**: Wenn DBI fertig ist, wartet die App erneut auf die Switch. Der zuletzt verwendete Ordner wird gespeichert; die Installation beginnt, sobald Sie auf **Starten** klicken.
- **11 Sprachen**: Englisch, Russisch, Spanisch, Italienisch, Deutsch, Französisch, Portugiesisch, Chinesisch, Japanisch, Hindi und Arabisch. Die Sprache richtet sich nach dem System und lässt sich im Fenster ändern.
- **Integrierter Windows-Treiber**: Erkennt die App die Switch ohne Treiber, bietet sie an, ihn zu installieren. Das ersetzt den manuellen Schritt mit Zadig (siehe unten).
- **Dateien über MTP unter macOS und Linux**: Speicher der Switch durchsuchen, Dateien und ganze Ordner hoch- und herunterladen, Dateien löschen und Spielstände sichern – ohne Android File Transfer (siehe unten).
- **Dateien über FTP auf allen Systemen**: die Switch im Netzwerk finden und sich per WLAN, ohne Kabel, mit dem FTP-Server von DBI verbinden – um die SD-Karte zu verwalten oder Spiele zu installieren (siehe unten).
- **Unterbrochenes Hochladen wird fortgesetzt**, wo es abgebrochen ist – auch nach einer erneuten Verbindung.
- **Benachrichtigungen**, wenn eine lange Installation oder Übertragung abgeschlossen ist, und Prüfung auf neue Versionen beim Start.
- **Einstellungen**: helle oder dunkle Darstellung und sechs Stile für die Dateiansicht in den Tabs MTP und FTP.
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

1. Starten Sie die App, wählen Sie den Ordner mit Ihren Spielen (Unterordner werden einbezogen) und klicken Sie auf **Starten**.
2. Öffnen Sie auf der Switch DBI und wählen Sie **Install title from USB**.
3. Verbinden Sie die Switch per USB-Kabel mit dem Computer. Der Status wechselt zu **Verbunden**.
4. Wählen Sie die Dateien in DBI aus und installieren Sie sie. Fortschritt und Geschwindigkeit werden unten im Fenster angezeigt.

## Windows: USB-Treiber

Unter Windows kann die App nur über den WinUSB-Treiber mit der Switch kommunizieren. Bisher mussten Sie ihn manuell mit [Zadig](https://zadig.akeo.ie/) installieren.

Jetzt übernimmt das die App selbst. Erkennt sie die Switch ohne Treiber, zeigt sie eine Abfrage an: Klicken Sie auf **Installieren** und bestätigen Sie die Administratoranfrage von Windows. Sie können auch jederzeit die Schaltfläche **USB-Treiber installieren** im Fenster verwenden. Die Switch muss angeschlossen sein und DBI sich im Modus **Install title from USB** befinden.

So funktioniert es: Die App weist den WinUSB-Treiber zu, der mit Windows ausgeliefert wird und von Microsoft signiert ist (genau wie **Treiber aktualisieren → Aus einer Liste verfügbarer Treiber auf meinem Computer auswählen → WinUsb Device** im Geräte-Manager). Es werden keine Zertifikate zum System hinzugefügt, und es funktioniert auch bei aktivierter Intelligenter App-Steuerung (Smart App Control).

## Dateien über MTP (macOS und Linux)

macOS bietet keine integrierte MTP-Unterstützung. Deshalb erscheint die Switch nicht im Finder, wenn DBI seinen MTP-Responder ausführt, und Android File Transfer, die übliche Notlösung, wird nicht mehr weiterentwickelt. Der Tab **Dateien (MTP)** ersetzt es:

1. Öffnen Sie auf der Switch DBI und wählen Sie **Run MTP responder**. Schließen Sie dann das Kabel an.
2. Öffnen Sie in der App den Tab **Dateien (MTP)** und klicken Sie auf **Verbinden**.
3. Wählen Sie einen Speicher (SD-Karte, NAND, Installationsziele, Spielstände usw.), öffnen Sie Ordner und laden Sie Dateien und Ordner mit **Dateien hochladen…** und **Ordner hochladen…** oder per Drag & Drop ins Fenster hoch. Sie können außerdem Dateien herunterladen und löschen sowie Ordner anlegen.

Um ein Spiel zu installieren, laden Sie es in einen Speicher hoch, dessen Name „install“ enthält: DBI installiert es, während es übertragen wird. Dateien über 4 GB werden unterstützt.

**Ein hochgeladener Ordner** wird mit dem gleichnamigen Ordner auf der Switch zusammengeführt (Groß- und Kleinschreibung spielt keine Rolle): Dateien mit gleichem Namen werden ersetzt, alles andere auf der Switch bleibt erhalten. Systemdateien wie `.DS_Store` werden übersprungen. In Installationsspeicher werden nur die Dateien gesendet, ohne die Ordner.

**Bricht das Hochladen ab** (Kabel gelöst, Verbindung unterbrochen), erscheint eine Leiste mit **Fortsetzen** – auch nach einer erneuten Verbindung. Damit wird der Rest gesendet: Bereits vollständig übertragene Dateien werden übersprungen, die gerade übertragene Datei wird erneut gesendet.

**Ein heruntergeladener Ordner** (über die Schaltfläche zum Herunterladen oder per Rechtsklick) wird mit seinem gesamten Inhalt kopiert.

**Spielstände sichern**: Zeigt DBI seinen Speicher „Saves“ an, kopiert diese Schaltfläche alle Spielstände in einen neuen Ordner „DBI saves <Datum>“ auf dem Computer.

![Dateien (MTP)](docs/screenshot-mtp.png)

**„Die Switch wird von einem anderen Programm verwendet.“** Unter macOS belegt der Kameradienst des Systems (`ptpcamerad`) MTP-Geräte, sobald sie angeschlossen werden. Klicken Sie auf **Gerät freigeben**: Die App beendet den Dienst und übernimmt die Switch; macOS startet den Dienst bei Bedarf von selbst wieder. Auch Android File Transfer belegt das Gerät, beenden Sie es also vorher.

Unter Windows ist der Tab ausgeblendet: Der Datei-Explorer zeigt die Switch im MTP-Modus bereits an.

## Dateien über FTP (alle Systeme)

DBI kann einen FTP-Server per WLAN bereitstellen. Der Tab **FTP** verbindet sich damit und bietet denselben Dateimanager wie der MTP-Tab, einschließlich des Hochladens von Ordnern:

1. Öffnen Sie auf der Switch DBI und wählen Sie **Run FTP server**. Auf dem Bildschirm wird die Adresse der Switch angezeigt.
2. Öffnen Sie in der App den Tab **FTP**, klicken Sie auf **Suchen**, um die Switch im Netzwerk zu finden (oder geben Sie ihre Adresse ein), wählen Sie den Modus und klicken Sie auf **Verbinden**:
   - **SD-Karte (Port 5000)**: SD-Karte durchsuchen, Dateien und Ordner hochladen, Dateien herunterladen und löschen, Ordner anlegen;
   - **Installation (Port 6000)**: Spiele hochladen, um sie zu installieren.

Computer und Switch müssen sich im selben Netzwerk befinden. Der Server von DBI verlangt kein Passwort; falls Sie eines festgelegt haben, geben Sie es unter **Benutzername und Passwort** ein. Über WLAN ist die Übertragung meist langsamer als über USB. Adressen, mit denen Sie sich bereits verbunden haben, bleiben im Menü des Adressfelds gespeichert.

Der FTP-Server von DBI kann keine Namen mit nicht-lateinischen Buchstaben speichern (Kyrillisch, Umlaute, Buchstaben mit Akzenten usw.): Er lehnt sie ab oder speichert sie so, dass sie nicht angezeigt werden. Vor einem solchen Hochladen fragt die App, ob diese Dateien und Ordner mit lateinischen Buchstaben umbenannt („Паспорт.pdf“ → „Pasport.pdf“) oder übersprungen werden sollen.

![FTP](docs/screenshot-ftp.png)

## Einstellungen

Die Schaltfläche ⚙ neben der Sprachauswahl öffnet die Einstellungen:

- **Allgemein**: Benachrichtigungen, wenn eine Installation oder Übertragung von mehr als 10 Sekunden abgeschlossen ist (**Testen** sendet sofort eine), und die Prüfung auf eine neue Version beim Start. Gibt es eine, erscheint neben ⚙ eine Schaltfläche mit ihrer Nummer.
- **Darstellung**: wie im System, hell oder dunkel.
- **Dateiansicht** für die Tabs MTP und FTP: klassisch, farbige Symbole, Formatkennzeichen, Tabelle, zweizeilig oder Kacheln. Eine Vorschau zeigt jeden Stil.

In jedem Stil öffnet ein Rechtsklick auf eine Datei oder einen Ordner ein Menü mit den zugehörigen Aktionen.

![Einstellungen](docs/screenshot-settings.png)

## Linux

Installieren Sie die udev-Regel, um ohne root auf die Switch zugreifen zu können (sie gilt für beide Modi: USB-Installation und MTP):

```bash
sudo cp data/linux/99-dbibackend.rules /etc/udev/rules.d/
sudo udevadm control --reload-rules
```

Meldet der Tab **Dateien (MTP)**, dass die Switch von einem anderen Programm verwendet wird, hat die Desktop-Umgebung sie möglicherweise selbst eingebunden (gvfs): Hängen Sie sie im Dateimanager aus.

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

Ist `waiting_for_switch` hoch, liegt der Engpass bei der Switch (Schreibgeschwindigkeit der microSD, NSZ-Entpacken). Ist **Debug** (im Protokollbereich) aktiviert, zeigt das Protokoll jede DBI-Anfrage mit Größe und Geschwindigkeit.

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
| `usbconn` | Öffnen der Switch über libusb (gousb) im USB-Installations- und MTP-Modus, Bulk-Transfers |
| `mtp` | MTP-Client (PTP über USB): Speicher, Ordner, Hoch- und Herunterladen, Dateien über 4 GB |
| `gui` | Fyne-Oberfläche, FTP-Client |
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
- [Fyne](https://fyne.io/) (BSD-3-Clause), [gousb](https://github.com/google/gousb) (Apache-2.0), [zenity](https://github.com/ncruces/zenity) (MIT), [jlaffaye/ftp](https://github.com/jlaffaye/ftp) (ISC).

Dieses Projekt steht in keiner Verbindung zu Nintendo. Installieren Sie nur Spiele, die Sie besitzen.
