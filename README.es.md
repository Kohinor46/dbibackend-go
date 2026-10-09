# DBI Backend (Go)

[English](README.md) | [Русский](README.ru.md) | **Español** | [Italiano](README.it.md) | [Deutsch](README.de.md) | [Français](README.fr.md) | [Português](README.pt.md) | [中文](README.zh.md) | [日本語](README.ja.md) | [हिन्दी](README.hi.md) | [العربية](README.ar.md)

Una aplicación de escritorio para instalar juegos por USB en una Nintendo Switch con [DBI](https://github.com/rashevskyv/dbi). Elige una carpeta con archivos `.nsp` / `.nsz` / `.xci`, conecta la Switch e instala.

Es una reescritura en Go de [lunixoid/dbibackend](https://github.com/lunixoid/dbibackend) (un script de Python) con interfaz gráfica, compilaciones listas para macOS y Windows, y sin necesidad de instalar Python ni libusb.

![DBI Backend](docs/screenshot.png)

## Características

- **Interfaz gráfica**: selector de carpeta (Finder, Explorador o arrastrar y soltar), lista de archivos encontrados con su tamaño, estado de la conexión, barra de progreso con la velocidad de transferencia y un panel de registro común a todas las pestañas. El panel aparece contraído y muestra solo la última línea; despliégalo cuando lo necesites.
- **Sin reinicios**: cuando DBI termina, la aplicación vuelve a esperar a la Switch. Recuerda la última carpeta; la instalación empieza cuando haces clic en **Iniciar**.
- **11 idiomas**: inglés, ruso, español, italiano, alemán, francés, portugués, chino, japonés, hindi y árabe. El idioma sigue al del sistema y se puede cambiar desde la ventana.
- **Controlador de Windows integrado**: si la aplicación detecta la Switch sin controlador, te ofrece instalarlo. Esto sustituye el paso manual con Zadig (ver más abajo).
- **Archivos por MTP en macOS y Linux**: explora los almacenamientos de la Switch, sube archivos y carpetas enteras, y descarga y elimina archivos sin Android File Transfer (ver más abajo).
- **Archivos por FTP en todos los sistemas**: conéctate al servidor FTP de DBI por Wi-Fi, sin cable, para gestionar la tarjeta SD o instalar juegos (ver más abajo).
- **Ajustes**: apariencia clara u oscura y seis estilos de vista de archivos para las pestañas MTP y FTP.
- **Modo de línea de comandos** (`-cli`), que se comporta como el script original, para automatizaciones.
- **Más segura que la original**: la Switch solo puede pedir archivos de la carpeta elegida.

## Descarga

Las compilaciones listas para usar están en la página de [Releases](../../releases).

| Sistema | Archivo | Notas |
|---|---|---|
| macOS 12+ (Apple Silicon e Intel) | `DBI Backend.app` | No está firmada con un certificado de desarrollador de Apple. La primera vez: clic derecho → **Abrir**, o ejecuta `xattr -dr com.apple.quarantine "DBI Backend.app"`. |
| Windows 10/11 (x64 y ARM64) | `DBI Backend.exe` | Un único archivo, no hay que instalar nada más. El controlador USB se instala desde la aplicación (ver más abajo). |
| Linux | compilar desde el código fuente | Consulta [Compilación](#compilación). |

## Uso

1. Abre la aplicación, elige la carpeta con tus juegos (se incluyen las subcarpetas) y haz clic en **Iniciar**.
2. En la Switch, abre DBI y elige **Install title from USB**.
3. Conecta la Switch al ordenador con un cable USB. El estado cambia a **Conectado**.
4. Elige los archivos en DBI e instálalos. El progreso y la velocidad se muestran en la parte inferior de la ventana.

## Windows: controlador USB

En Windows, la aplicación solo puede comunicarse con la Switch a través del controlador WinUSB. Antes había que instalarlo a mano con [Zadig](https://zadig.akeo.ie/).

Ahora lo hace la propia aplicación. Cuando detecta la Switch sin controlador, muestra un aviso: haz clic en **Instalar** y acepta la solicitud de administrador de Windows. También puedes usar el botón **Instalar controlador USB** de la ventana en cualquier momento. La Switch debe estar conectada y DBI debe estar en el modo **Install title from USB**.

Cómo funciona: la aplicación asigna el controlador WinUSB que viene con Windows y está firmado por Microsoft (lo mismo que **Actualizar controlador → Elegir en una lista de controladores disponibles en el equipo → WinUsb Device** en el Administrador de dispositivos). No añade ningún certificado al sistema y funciona con el Control inteligente de aplicaciones (Smart App Control) activado.

## Archivos por MTP (macOS y Linux)

macOS no tiene soporte integrado para MTP, así que la Switch no aparece en Finder cuando DBI ejecuta su respondedor MTP, y Android File Transfer, la solución habitual, ya no se actualiza. La pestaña **Archivos (MTP)** lo sustituye:

1. En la Switch, abre DBI y elige **Run MTP responder**; después conecta el cable.
2. En la aplicación, abre la pestaña **Archivos (MTP)** y haz clic en **Conectar**.
3. Elige un almacenamiento (tarjeta SD, NAND, destinos de instalación, partidas guardadas, etc.), abre carpetas y sube archivos y carpetas con **Subir archivos…** y **Subir carpeta…** o arrastrándolos a la ventana. También puedes descargar y eliminar archivos y crear carpetas.

Para instalar un juego, súbelo a un almacenamiento que tenga «install» en su nombre: DBI lo instala a medida que llega. Se admiten archivos de más de 4 GB.

**Al subir una carpeta**, se fusiona con la carpeta del mismo nombre en la Switch (sin distinguir mayúsculas de minúsculas): los archivos con el mismo nombre se reemplazan y todo lo demás que haya en la Switch se conserva. Los archivos de servicio, como `.DS_Store`, se omiten. A los almacenamientos de instalación solo se envían los archivos, sin las carpetas.

![Archivos (MTP)](docs/screenshot-mtp.png)

**«La Switch está en uso por otro programa».** En macOS, el servicio de cámaras del sistema (`ptpcamerad`) se apropia de los dispositivos MTP en cuanto se conectan. Haz clic en **Liberar dispositivo**: la aplicación detiene el servicio y toma la Switch; macOS vuelve a iniciar el servicio por sí solo cuando lo necesita. Android File Transfer también retiene el dispositivo, así que ciérralo antes.

En Windows la pestaña está oculta: el Explorador de archivos ya muestra la Switch en modo MTP.

## Archivos por FTP (todos los sistemas)

DBI puede ejecutar un servidor FTP por Wi-Fi. La pestaña **FTP** se conecta a él y ofrece el mismo gestor de archivos que la pestaña MTP, incluida la subida de carpetas:

1. En la Switch, abre DBI y elige **Run FTP server**. En la pantalla aparece la dirección de la Switch.
2. En la aplicación, abre la pestaña **FTP**, introduce la dirección, elige el modo y haz clic en **Conectar**:
   - **Tarjeta SD (puerto 5000)**: explora la tarjeta SD, sube archivos y carpetas, descarga y elimina archivos y crea carpetas;
   - **Instalación (puerto 6000)**: sube juegos para instalarlos.

El ordenador y la Switch deben estar en la misma red. El servidor de DBI no pide contraseña; si has configurado una, introdúcela en **Usuario y contraseña**. Por Wi-Fi, las transferencias suelen ser más lentas que por USB.

![FTP](docs/screenshot-ftp.png)

## Ajustes

El botón ⚙ junto al selector de idioma abre los ajustes:

- **Apariencia**: como el sistema, clara u oscura.
- **Vista de archivos** para las pestañas MTP y FTP: clásica, iconos de colores, etiquetas de formato, tabla, dos líneas o mosaico. Una vista previa muestra cada estilo.

En todos los estilos, al hacer clic derecho en un archivo o una carpeta se abre un menú con sus acciones.

![Ajustes](docs/screenshot-settings.png)

## Linux

Para acceder a la Switch sin root, instala la regla de udev (sirve tanto para el modo de instalación por USB como para el modo MTP):

```bash
sudo cp data/linux/99-dbibackend.rules /etc/udev/rules.d/
sudo udevadm control --reload-rules
```

Si la pestaña **Archivos (MTP)** indica que la Switch está en uso por otro programa, puede que el escritorio la haya montado por su cuenta (gvfs): desmóntala en el gestor de archivos.

## Velocidad de transferencia

La velocidad la limita sobre todo la Switch, no el ordenador:

- Por **USB 2.0** el máximo ronda los **40 MB/s**. Con archivos `.nsp` / `.xci` sin comprimir, la aplicación mantiene el bus ocupado cerca del 80 % del tiempo y alcanza unos 32 MB/s de media.
- **Los archivos `.nsz`** se instalan unas tres veces más despacio: la Switch descomprime cada bloque por sí misma.
- DBI pide los datos en fragmentos de hasta 1 MB y solo solicita el siguiente cuando ha procesado el anterior. El ordenador no puede acelerar esto.

Al final de cada sesión, el registro muestra una línea `Session stats`:

| Campo | Significado |
|---|---|
| `usb_MB/s` | Velocidad mientras se envían datos |
| `avg_MB/s` | Velocidad media desde la primera solicitud hasta la última |
| `time_data` / `time_handshake` / `waiting_for_switch` | Proporción del tiempo dedicada a enviar datos, al intercambio del protocolo y a esperar a la Switch |
| `active` / `idle` | Tiempo de instalación y tiempo inactivo antes y después |

Si `waiting_for_switch` es alto, el cuello de botella es la Switch (velocidad de escritura de la microSD, descompresión de NSZ). Con **Depuración** activada (en el panel de registro), el registro muestra cada solicitud de DBI con su tamaño y velocidad.

## Modo de línea de comandos

```bash
dbibackend -cli ~/Switch          # wait for the Switch, serve one session, exit
dbibackend -cli -debug ~/Switch   # the same with a detailed log
dbibackend ~/Switch               # open the window with this folder
dbibackend -install-driver        # Windows: install the WinUSB driver for the connected Switch
```

En macOS, el servidor puede iniciarse automáticamente al conectar la Switch: edita la ruta en `data/darwin/com.dbibackend.usb.agent.plist` y cópialo a `~/Library/LaunchAgents/`.

## Compilación

Necesitas Go 1.25+ y un compilador de C (libusb y Fyne requieren cgo).

```bash
# macOS
brew install libusb pkg-config
# Debian/Ubuntu (plus the Fyne dependencies: https://docs.fyne.io/started/)
sudo apt install libusb-1.0-0-dev pkg-config

make build        # ./dbibackend for the current system
make test         # tests
make release      # dist/DBI Backend.app and dist/DBI Backend.exe
```

`make release` se ejecuta en macOS. Compila libusb desde el código fuente y la enlaza estáticamente, así que el resultado no necesita nada instalado. La compilación para Windows requiere además `brew install mingw-w64` y la herramienta `fyne` (`go install fyne.io/tools/cmd/fyne@latest`).

## Estructura del proyecto

| Paquete | Función |
|---|---|
| `dbi` | Protocolo DBI0 (LIST / FILE_RANGE / EXIT), escaneo de carpetas, estadísticas de sesión |
| `usbconn` | Apertura de la Switch mediante libusb (gousb) en los modos de instalación por USB y MTP, transferencias bulk |
| `mtp` | Cliente MTP (PTP sobre USB): almacenamientos, carpetas, subida y descarga, archivos de más de 4 GB |
| `gui` | Interfaz con Fyne, cliente FTP |
| `i18n` | Traducciones (`i18n/locales/*.json`) |
| `winusb` | Instalación de WinUSB en Windows (SetupAPI) |
| `cmd/winusb-helper` | Ayudante nativo para Windows ARM64 que instala el controlador |

Para añadir o corregir una traducción, edita `i18n/locales/<code>.json`. `go test ./i18n/` comprueba que todos los idiomas tengan todas las claves.

## Diferencias con el original

- La Switch solo puede pedir archivos de la carpeta elegida. El original abría cualquier ruta que enviara el dispositivo.
- Se ha corregido el filtro de extensiones: el original buscaba `nsz` sin el punto.
- Si hay archivos con el mismo nombre en distintas subcarpetas, se usa el primero que se encuentra (DBI solo ve los nombres de archivo).

## Agradecimientos y licencias

- **Este proyecto**: [MIT](LICENSE).
- [lunixoid/dbibackend](https://github.com/lunixoid/dbibackend) (MIT): la implementación original del protocolo.
- [DBI](https://github.com/rashevskyv/dbi) de duckbill: el instalador de la Switch.
- [libusb](https://libusb.info/) (LGPL-2.1) se enlaza estáticamente en las compilaciones. El código fuente de este proyecto es abierto, así que puedes recompilarlo con tu propia versión de libusb.
- [Fyne](https://fyne.io/) (BSD-3-Clause), [gousb](https://github.com/google/gousb) (Apache-2.0), [zenity](https://github.com/ncruces/zenity) (MIT), [jlaffaye/ftp](https://github.com/jlaffaye/ftp) (ISC).

Este proyecto no está afiliado a Nintendo. Instala solo juegos que poseas.
