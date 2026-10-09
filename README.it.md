# DBI Backend (Go)

[English](README.md) | [Русский](README.ru.md) | [Español](README.es.md) | **Italiano** | [Deutsch](README.de.md) | [Français](README.fr.md) | [Português](README.pt.md) | [中文](README.zh.md) | [日本語](README.ja.md) | [हिन्दी](README.hi.md) | [العربية](README.ar.md)

Un'app desktop per installare giochi via USB su una Nintendo Switch con [DBI](https://github.com/rashevskyv/dbi). Scegli una cartella con file `.nsp` / `.nsz` / `.xci`, collega la Switch e installa.

È una riscrittura in Go di [lunixoid/dbibackend](https://github.com/lunixoid/dbibackend) (uno script Python) con interfaccia grafica, build pronte per macOS e Windows, e senza bisogno di installare Python o libusb.

![DBI Backend](docs/screenshot.png)

## Funzionalità

- **Interfaccia grafica**: selezione della cartella (Finder, Esplora file o trascinamento), elenco dei file trovati con le dimensioni, stato della connessione, barra di avanzamento con velocità di trasferimento e un pannello del registro condiviso da tutte le schede. Il pannello è compresso sull'ultima riga; espandilo quando ti serve, copia l'intero registro o salvalo in un file.
- **Nessun riavvio**: quando DBI ha finito, l'app attende di nuovo la Switch. L'ultima cartella viene ricordata; l'installazione parte quando fai clic su **Avvia**.
- **11 lingue**: inglese, russo, spagnolo, italiano, tedesco, francese, portoghese, cinese, giapponese, hindi e arabo. La lingua segue quella del sistema e si può cambiare dalla finestra.
- **Driver per Windows integrato**: se l'app rileva la Switch senza driver, propone di installarlo. Questo sostituisce il passaggio manuale con Zadig (vedi sotto).
- **File via MTP su macOS e Linux**: sfoglia le memorie della Switch, carica e scarica file e intere cartelle, elimina file e fai il backup dei salvataggi dei giochi, senza Android File Transfer (vedi sotto).
- **File via FTP su tutti i sistemi**: trova la Switch nella rete e connettiti al server FTP di DBI via Wi-Fi, senza cavo, per gestire la scheda SD o installare giochi (vedi sotto).
- **I caricamenti interrotti riprendono** da dove si erano fermati, anche dopo la riconnessione.
- **Notifiche** alla fine di un'installazione o di un trasferimento lungo e controllo di nuove versioni all'avvio.
- **Impostazioni**: aspetto chiaro o scuro e sei stili di vista dei file per le schede MTP e FTP.
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

1. Avvia l'app, scegli la cartella con i tuoi giochi (sono incluse anche le sottocartelle) e fai clic su **Avvia**.
2. Sulla Switch, apri DBI e scegli **Install title from USB**.
3. Collega la Switch al computer con un cavo USB. Lo stato diventa **Connesso**.
4. Scegli i file in DBI e installali. Avanzamento e velocità sono mostrati in fondo alla finestra.

## Windows: driver USB

Su Windows l'app può comunicare con la Switch solo tramite il driver WinUSB. Prima bisognava installarlo a mano con [Zadig](https://zadig.akeo.ie/).

Ora ci pensa l'app. Quando rileva la Switch senza driver, mostra una richiesta: fai clic su **Installa** e conferma la richiesta di amministratore di Windows. Puoi anche usare in qualsiasi momento il pulsante **Installa driver USB** nella finestra. La Switch deve essere collegata e DBI deve essere in modalità **Install title from USB**.

Come funziona: l'app assegna il driver WinUSB incluso in Windows e firmato da Microsoft (come **Aggiorna driver → Scegli da un elenco di driver disponibili nel computer → WinUsb Device** in Gestione dispositivi). Non aggiunge alcun certificato al sistema e funziona con Smart App Control attivo.

## File via MTP (macOS e Linux)

macOS non supporta MTP in modo nativo, quindi la Switch non compare nel Finder quando DBI avvia il suo MTP responder, e Android File Transfer, la soluzione abituale, non viene più aggiornato. La scheda **File (MTP)** lo sostituisce:

1. Sulla Switch, apri DBI e scegli **Run MTP responder**, poi collega il cavo.
2. Nell'app, apri la scheda **File (MTP)** e fai clic su **Connetti**.
3. Scegli una memoria (scheda SD, NAND, destinazioni di installazione, salvataggi e così via), apri le cartelle e carica file e cartelle con **Carica file…** e **Carica cartella…** o trascinandoli nella finestra. Puoi anche scaricare ed eliminare file e creare cartelle.

Per installare un gioco, caricalo in una memoria che abbia «install» nel nome: DBI lo installa man mano che arriva. Sono supportati file più grandi di 4 GB.

**Quando carichi una cartella**, il suo contenuto viene unito a quello della cartella con lo stesso nome sulla Switch (maiuscole e minuscole non contano): i file con lo stesso nome vengono sostituiti e tutto il resto sulla Switch rimane com'è. I file di servizio come `.DS_Store` vengono saltati. Nelle memorie di installazione vengono inviati solo i file, senza le cartelle.

**Se un caricamento si interrompe** (il cavo si è staccato, la connessione è caduta), compare una barra **Continua**, anche dopo la riconnessione. Invia il resto: i file già arrivati per intero vengono saltati e quello che si stava inviando viene inviato di nuovo.

**Quando scarichi una cartella** (con il pulsante di download o con un clic destro), viene copiata con tutto il suo contenuto.

**Backup dei salvataggi**: quando DBI mostra la sua memoria «Saves», questo pulsante copia tutti i salvataggi dei giochi in una nuova cartella «DBI saves <data>» sul computer.

![File (MTP)](docs/screenshot-mtp.png)

**«La Switch è in uso da parte di un altro programma».** Su macOS il servizio fotocamere di sistema (`ptpcamerad`) si appropria dei dispositivi MTP appena vengono collegati. Fai clic su **Libera dispositivo**: l'app arresta il servizio e prende la Switch; macOS riavvia il servizio da solo quando serve. Anche Android File Transfer tiene occupato il dispositivo, quindi chiudilo prima.

Su Windows la scheda è nascosta: Esplora file mostra già la Switch in modalità MTP.

## File via FTP (tutti i sistemi)

DBI può avviare un server FTP via Wi-Fi. La scheda **FTP** si connette a questo server e offre lo stesso file manager della scheda MTP, compreso il caricamento di cartelle:

1. Sulla Switch, apri DBI e scegli **Run FTP server**. Sullo schermo compare l'indirizzo della Switch.
2. Nell'app, apri la scheda **FTP**, fai clic su **Cerca** per trovare la Switch nella rete (oppure inserisci il suo indirizzo), scegli la modalità e fai clic su **Connetti**:
   - **Scheda SD (porta 5000)**: sfoglia la scheda SD, carica file e cartelle, scarica ed elimina file, crea cartelle;
   - **Installazione (porta 6000)**: carica i giochi per installarli.

Il computer e la Switch devono essere sulla stessa rete. Il server di DBI non richiede una password; se ne hai impostata una, inseriscila in **Nome utente e password**. Via Wi-Fi i trasferimenti sono di solito più lenti che via USB. Gli indirizzi a cui ti sei connesso restano nel menu del campo dell'indirizzo.

Il server FTP di DBI non riesce a salvare nomi con lettere non latine (cirillico, lettere accentate e così via): li rifiuta o li salva in modo che non risultino visibili. Prima di un caricamento del genere, l'app chiede se rinominare questi file e cartelle con lettere latine («Паспорт.pdf» → «Pasport.pdf») o saltarli.

![FTP](docs/screenshot-ftp.png)

## Impostazioni

Il pulsante ⚙ accanto alla scelta della lingua apre le impostazioni:

- **Generali**: notifiche quando termina un'installazione o un trasferimento durato più di 10 secondi (**Prova** ne invia una subito) e controllo di nuove versioni all'avvio. Se ce n'è una, accanto a ⚙ compare un pulsante con il suo numero.
- **Aspetto**: come il sistema, chiaro o scuro.
- **Vista dei file** per le schede MTP e FTP: classica, icone colorate, etichette di formato, tabella, due righe o riquadri. Un'anteprima mostra ogni stile.

In ogni stile, un clic destro su un file o una cartella apre un menu con le relative azioni.

![Impostazioni](docs/screenshot-settings.png)

## Linux

Per accedere alla Switch senza root, installa la regola udev (vale sia per la modalità di installazione via USB sia per quella MTP):

```bash
sudo cp data/linux/99-dbibackend.rules /etc/udev/rules.d/
sudo udevadm control --reload-rules
```

Se la scheda **File (MTP)** segnala che la Switch è in uso da parte di un altro programma, è possibile che l'ambiente desktop l'abbia montata da solo (gvfs): smontala dal file manager.

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

Se `waiting_for_switch` è alto, il collo di bottiglia è la Switch (velocità di scrittura della microSD, decompressione NSZ). Con **Debug** attivo (nel pannello del registro), il registro mostra ogni richiesta di DBI con dimensione e velocità.

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
| `usbconn` | Apertura della Switch tramite libusb (gousb) in modalità installazione via USB e MTP, trasferimenti bulk |
| `mtp` | Client MTP (PTP su USB): memorie, cartelle, caricamento e download, file più grandi di 4 GB |
| `gui` | Interfaccia Fyne, client FTP |
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
- [Fyne](https://fyne.io/) (BSD-3-Clause), [gousb](https://github.com/google/gousb) (Apache-2.0), [zenity](https://github.com/ncruces/zenity) (MIT), [jlaffaye/ftp](https://github.com/jlaffaye/ftp) (ISC).

Questo progetto non è affiliato a Nintendo. Installa solo giochi di cui possiedi una copia.
