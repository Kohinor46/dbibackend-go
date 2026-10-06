# DBI Backend (Go)

[English](README.md) | [Русский](README.ru.md) | [Español](README.es.md) | **Italiano** | [Deutsch](README.de.md) | [Français](README.fr.md) | [Português](README.pt.md) | [中文](README.zh.md) | [日本語](README.ja.md) | [हिन्दी](README.hi.md) | [العربية](README.ar.md)

Un'app desktop per installare giochi via USB su una Nintendo Switch con [DBI](https://github.com/rashevskyv/dbi). Scegli una cartella con file `.nsp` / `.nsz` / `.xci`, collega la Switch e installa.

È una riscrittura in Go di [lunixoid/dbibackend](https://github.com/lunixoid/dbibackend) (uno script Python) con interfaccia grafica, build pronte per macOS e Windows, e senza bisogno di installare Python o libusb.

![DBI Backend](docs/screenshot.png)

## Funzionalità

- **Interfaccia grafica**: selezione della cartella (Finder, Esplora file o trascinamento), elenco dei file trovati con le dimensioni, stato della connessione, barra di avanzamento con velocità di trasferimento e un registro.
- **Nessun riavvio**: quando DBI ha finito, l'app attende di nuovo la Switch. L'ultima cartella viene ricordata e il server si avvia automaticamente all'apertura.
- **11 lingue**: inglese, russo, spagnolo, italiano, tedesco, francese, portoghese, cinese, giapponese, hindi e arabo. La lingua segue quella del sistema e si può cambiare dalla finestra.
- **Driver per Windows integrato**: se l'app rileva la Switch senza driver, propone di installarlo. Questo sostituisce il passaggio manuale con Zadig (vedi sotto).
- **Modalità a riga di comando** (`-cli`) che si comporta come lo script originale, per l'automazione.
- **Più sicura dell'originale**: la Switch può richiedere solo file dalla cartella scelta.

## Download

Le build pronte sono nella pagina [Releases](../../releases).

| Sistema | File | Note |
|---|---|---|
| macOS 12+ (Apple Silicon e Intel) | `DBI Backend.app` | Non firmata con un certificato sviluppatore Apple. Al primo avvio: clic destro → **Apri**, oppure esegui `xattr -dr com.apple.quarantine "DBI Backend.app"`. |
| Windows 10/11 (x64 e ARM64) | `DBI Backend.exe` | Un unico file, nient'altro da installare. Il driver USB si installa dall'app (vedi sotto). |
| Linux | compilare dai sorgenti | Vedi [Compilazione](#compilazione). |

## Utilizzo

1. Avvia l'app e scegli la cartella con i tuoi giochi. Sono incluse anche le sottocartelle.
2. Sulla Switch, apri DBI e scegli **Install title from USB**.
3. Collega la Switch al computer con un cavo USB. Lo stato diventa **Connesso**.
4. Scegli i file in DBI e installali. Avanzamento e velocità sono mostrati in fondo alla finestra.

## Windows: driver USB

Su Windows l'app può comunicare con la Switch solo tramite il driver WinUSB. Prima bisognava installarlo a mano con [Zadig](https://zadig.akeo.ie/).

Ora ci pensa l'app. Quando rileva la Switch senza driver, mostra una richiesta: fai clic su **Installa** e conferma la richiesta di amministratore di Windows. Puoi anche usare in qualsiasi momento il pulsante **Installa driver USB** nella finestra. La Switch deve essere collegata e DBI deve essere in modalità **Install title from USB**.

Come funziona: l'app assegna il driver WinUSB incluso in Windows e firmato da Microsoft (come **Aggiorna driver → Scegli da un elenco di driver disponibili nel computer → WinUsb Device** in Gestione dispositivi). Non aggiunge alcun certificato al sistema e funziona con Smart App Control attivo.

## Linux

Per accedere alla Switch senza root, installa la regola udev:

```bash
sudo cp data/linux/99-dbibackend.rules /etc/udev/rules.d/
sudo udevadm control --reload-rules
```

## Velocità di trasferimento

La velocità dipende soprattutto dalla Switch, non dal computer:

- Su **USB 2.0** il limite è di circa **40 MB/s**. Con file `.nsp` / `.xci` non compressi, l'app tiene il bus occupato per circa l'80% del tempo e raggiunge in media circa 32 MB/s.
- **I file `.nsz`** si installano circa tre volte più lentamente: la Switch decomprime ogni blocco da sola.
- DBI richiede i dati in blocchi fino a 1 MB e chiede il successivo solo dopo aver elaborato il precedente. Il computer non può velocizzare questo processo.

Alla fine di ogni sessione il registro mostra una riga `Session stats`:

| Campo | Significato |
|---|---|
| `usb_MB/s` | Velocità durante l'invio dei dati |
| `avg_MB/s` | Velocità media dalla prima all'ultima richiesta |
| `time_data` / `time_handshake` / `waiting_for_switch` | Quota di tempo spesa per l'invio dei dati, per lo scambio del protocollo e in attesa della Switch |
| `active` / `idle` | Tempo di installazione e tempo di inattività prima e dopo |

Se `waiting_for_switch` è alto, il collo di bottiglia è la Switch (velocità di scrittura della microSD, decompressione NSZ). Con **Debug** attivo, il registro mostra ogni richiesta di DBI con dimensione e velocità.

## Modalità a riga di comando

```bash
dbibackend -cli ~/Switch          # wait for the Switch, serve one session, exit
dbibackend -cli -debug ~/Switch   # the same with a detailed log
dbibackend ~/Switch               # open the window with this folder
dbibackend -install-driver        # Windows: install the WinUSB driver for the connected Switch
```

Su macOS il server può avviarsi automaticamente quando si collega la Switch: modifica il percorso in `data/darwin/com.dbibackend.usb.agent.plist` e copia il file in `~/Library/LaunchAgents/`.

## Compilazione

Servono Go 1.25+ e un compilatore C (cgo è richiesto da libusb e Fyne).

```bash
# macOS
brew install libusb pkg-config
# Debian/Ubuntu (plus the Fyne dependencies: https://docs.fyne.io/started/)
sudo apt install libusb-1.0-0-dev pkg-config

make build        # ./dbibackend for the current system
make test         # tests
make release      # dist/DBI Backend.app and dist/DBI Backend.exe
```

`make release` funziona su macOS. Compila libusb dai sorgenti e la collega staticamente, quindi il risultato non richiede nulla di installato. La build per Windows richiede anche `brew install mingw-w64` e lo strumento `fyne` (`go install fyne.io/tools/cmd/fyne@latest`).

## Struttura del progetto

| Pacchetto | Scopo |
|---|---|
| `dbi` | Protocollo DBI0 (LIST / FILE_RANGE / EXIT), scansione della cartella, statistiche di sessione |
| `usbconn` | Apertura della Switch tramite libusb (gousb), trasferimenti bulk |
| `gui` | Interfaccia Fyne |
| `i18n` | Traduzioni (`i18n/locales/*.json`) |
| `winusb` | Installazione di WinUSB su Windows (SetupAPI) |
| `cmd/winusb-helper` | Helper nativo ARM64 per Windows per l'installazione del driver |

Per aggiungere o correggere una traduzione, modifica `i18n/locales/<code>.json`. `go test ./i18n/` verifica che ogni lingua abbia tutte le chiavi.

## Differenze rispetto all'originale

- La Switch può richiedere solo file dalla cartella scelta. L'originale apriva qualsiasi percorso inviato dal dispositivo.
- Il filtro delle estensioni è stato corretto: l'originale cercava `nsz` senza il punto.
- Se file in sottocartelle diverse hanno lo stesso nome, viene usato il primo trovato (DBI vede solo i nomi dei file).

## Ringraziamenti e licenze

- **Questo progetto**: [MIT](LICENSE).
- [lunixoid/dbibackend](https://github.com/lunixoid/dbibackend) (MIT): l'implementazione originale del protocollo.
- [DBI](https://github.com/rashevskyv/dbi) di duckbill: l'installer sulla Switch.
- [libusb](https://libusb.info/) (LGPL-2.1) è collegata staticamente nelle build. Il codice sorgente di questo progetto è aperto, quindi puoi ricompilarlo con la tua versione di libusb.
- [Fyne](https://fyne.io/) (BSD-3-Clause), [gousb](https://github.com/google/gousb) (Apache-2.0), [zenity](https://github.com/ncruces/zenity) (MIT).

Questo progetto non è affiliato a Nintendo. Installa solo giochi di cui possiedi una copia.
