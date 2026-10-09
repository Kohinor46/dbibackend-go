# DBI Backend (Go)

[English](README.md) | [Русский](README.ru.md) | [Español](README.es.md) | [Italiano](README.it.md) | [Deutsch](README.de.md) | [Français](README.fr.md) | **Português** | [中文](README.zh.md) | [日本語](README.ja.md) | [हिन्दी](README.hi.md) | [العربية](README.ar.md)

Um aplicativo para desktop que instala jogos via USB em um Nintendo Switch com o [DBI](https://github.com/rashevskyv/dbi). Escolha uma pasta com arquivos `.nsp` / `.nsz` / `.xci`, conecte o Switch e instale.

É uma reescrita em Go do [lunixoid/dbibackend](https://github.com/lunixoid/dbibackend) (um script em Python), com interface gráfica, builds prontos para macOS e Windows e sem precisar instalar Python nem libusb.

![DBI Backend](docs/screenshot.png)

## Recursos

- **Interface gráfica**: seleção de pasta (Finder, Explorador de Arquivos ou arrastar e soltar), lista dos arquivos encontrados com seus tamanhos, status da conexão, barra de progresso com velocidade de transferência e um painel de registro compartilhado por todas as abas. O painel fica recolhido, mostrando só a última linha; expanda-o quando precisar.
- **Sem reinícios**: quando o DBI termina, o aplicativo volta a aguardar o Switch. A última pasta fica salva, e a instalação começa quando você clica em **Iniciar**.
- **11 idiomas**: inglês, russo, espanhol, italiano, alemão, francês, português, chinês, japonês, hindi e árabe. O idioma acompanha o do sistema e pode ser alterado na janela.
- **Driver do Windows integrado**: quando o aplicativo detecta o Switch sem driver, ele oferece a instalação. Isso substitui a etapa manual com o Zadig (veja abaixo).
- **Arquivos via MTP no macOS e no Linux**: navegue pelos armazenamentos do Switch, envie arquivos e pastas inteiras, baixe e exclua arquivos, sem o Android File Transfer (veja abaixo).
- **Arquivos via FTP em qualquer sistema**: conecte-se ao servidor FTP do DBI pelo Wi-Fi, sem cabo, para gerenciar o cartão SD ou instalar jogos (veja abaixo).
- **Configurações**: aparência clara ou escura e seis estilos de exibição de arquivos nas abas MTP e FTP.
- **Modo de linha de comando** (`-cli`), que funciona como o script original, para automação.
- **Mais seguro que o original**: o Switch só pode solicitar arquivos da pasta escolhida.

## Download

Os builds prontos estão na página [Releases](../../releases).

| Sistema | Arquivo | Observações |
|---|---|---|
| macOS 12+ (Apple Silicon e Intel) | `DBI Backend.app` | Não é assinado com um certificado de desenvolvedor da Apple. Na primeira execução: clique com o botão direito → **Abrir** ou execute `xattr -dr com.apple.quarantine "DBI Backend.app"`. |
| Windows 10/11 (x64 e ARM64) | `DBI Backend.exe` | Um único arquivo, nada mais para instalar. O driver USB é instalado pelo próprio aplicativo (veja abaixo). |
| Linux | compilar a partir do código-fonte | Veja [Compilação](#compilação). |

## Como usar

1. Abra o aplicativo, escolha a pasta com seus jogos (as subpastas também são incluídas) e clique em **Iniciar**.
2. No Switch, abra o DBI e escolha **Install title from USB**.
3. Conecte o Switch ao computador com um cabo USB. O status muda para **Conectado**.
4. Escolha os arquivos no DBI e instale-os. O progresso e a velocidade aparecem na parte inferior da janela.

## Windows: driver USB

No Windows, o aplicativo só consegue se comunicar com o Switch pelo driver WinUSB. Antes era preciso instalá-lo manualmente com o [Zadig](https://zadig.akeo.ie/).

Agora o próprio aplicativo faz isso. Quando detecta o Switch sem driver, ele exibe um aviso: clique em **Instalar** e confirme a solicitação de administrador do Windows. Você também pode usar o botão **Instalar driver USB** na janela a qualquer momento. O Switch precisa estar conectado e o DBI, no modo **Install title from USB**.

Como funciona: o aplicativo atribui o driver WinUSB que acompanha o Windows e é assinado pela Microsoft (o mesmo que **Atualizar driver → Permitir que eu escolha… → WinUsb Device** no Gerenciador de Dispositivos). Ele não adiciona nenhum certificado ao sistema e funciona com o Controle Inteligente de Aplicativos ativado.

## Arquivos via MTP (macOS e Linux)

O macOS não tem suporte nativo a MTP, então o Switch não aparece no Finder quando o DBI está no modo MTP, e o Android File Transfer, a alternativa de costume, não recebe mais atualizações. A aba **Arquivos (MTP)** o substitui:

1. No Switch, abra o DBI e escolha **Run MTP responder**; depois, conecte o cabo.
2. No aplicativo, abra a aba **Arquivos (MTP)** e clique em **Conectar**.
3. Escolha um armazenamento (cartão SD, NAND, destinos de instalação, saves etc.), abra as pastas e envie arquivos e pastas com **Enviar arquivos…** e **Enviar pasta…** ou arrastando-os para a janela. Você também pode baixar e excluir arquivos e criar pastas.

Para instalar um jogo, envie-o para um armazenamento com “install” no nome: o DBI o instala à medida que ele chega. Arquivos com mais de 4 GB são suportados.

**Uma pasta enviada é mesclada** à pasta de mesmo nome no Switch (maiúsculas e minúsculas não fazem diferença): arquivos com o mesmo nome são substituídos, e todo o resto no Switch é mantido. Arquivos de sistema como `.DS_Store` são ignorados. Para os armazenamentos de instalação, só os arquivos são enviados, sem as pastas.

![Arquivos (MTP)](docs/screenshot-mtp.png)

**“O Switch está sendo usado por outro programa.”** No macOS, o serviço de câmeras do sistema (`ptpcamerad`) se apropria dos dispositivos MTP assim que eles são conectados. Clique em **Liberar dispositivo**: o aplicativo interrompe o serviço e assume o Switch; o macOS volta a iniciar o serviço sozinho quando necessário. O Android File Transfer também prende o dispositivo, então feche-o antes.

No Windows, a aba fica oculta: o Explorador de Arquivos já mostra o Switch no modo MTP.

## Arquivos via FTP (todos os sistemas)

O DBI pode executar um servidor FTP pelo Wi-Fi. A aba **FTP** se conecta a ele e oferece o mesmo gerenciador de arquivos da aba MTP, incluindo o envio de pastas:

1. No Switch, abra o DBI e escolha **Run FTP server**. A tela mostra o endereço do Switch.
2. No aplicativo, abra a aba **FTP**, digite o endereço, escolha o modo e clique em **Conectar**:
   - **Cartão SD (porta 5000)**: navegue pelo cartão SD, envie arquivos e pastas, baixe e exclua arquivos, crie pastas;
   - **Instalação (porta 6000)**: envie jogos para instalá-los.

O computador e o Switch precisam estar na mesma rede. O servidor do DBI permite o acesso sem senha; se você definiu uma, informe-a em **Usuário e senha**. Pelo Wi-Fi, as transferências costumam ser mais lentas do que pelo USB.

![FTP](docs/screenshot-ftp.png)

## Configurações

O botão ⚙ ao lado da seleção de idioma abre as configurações:

- **Aparência**: igual ao sistema, clara ou escura.
- **Exibição de arquivos** nas abas MTP e FTP: clássica, ícones coloridos, etiquetas de formato, tabela, duas linhas ou blocos. Uma prévia mostra cada estilo.

Em qualquer estilo, clicar com o botão direito em um arquivo ou pasta abre um menu com as ações disponíveis.

![Configurações](docs/screenshot-settings.png)

## Linux

Permita o acesso ao Switch sem root instalando a regra do udev (ela cobre os dois modos: instalação via USB e MTP):

```bash
sudo cp data/linux/99-dbibackend.rules /etc/udev/rules.d/
sudo udevadm control --reload-rules
```

Se a aba **Arquivos (MTP)** informar que o Switch está sendo usado por outro programa, talvez o ambiente de desktop o tenha montado automaticamente (gvfs): desmonte-o no gerenciador de arquivos.

## Velocidade de transferência

A velocidade é limitada principalmente pelo Switch, não pelo computador:

- Em **USB 2.0**, o limite é de cerca de **40 MB/s**. Com arquivos `.nsp` / `.xci` sem compressão, o aplicativo mantém o barramento ocupado cerca de 80% do tempo e atinge, em média, uns 32 MB/s.
- **Arquivos `.nsz`** são instalados cerca de três vezes mais devagar: o próprio Switch descompacta cada bloco.
- O DBI solicita os dados em partes de até 1 MB e só pede a próxima depois de processar a anterior. O computador não tem como acelerar isso.

Ao final de cada sessão, o registro mostra uma linha `Session stats`:

| Campo | Significado |
|---|---|
| `usb_MB/s` | Velocidade enquanto os dados estão sendo enviados |
| `avg_MB/s` | Velocidade média da primeira à última solicitação |
| `time_data` / `time_handshake` / `waiting_for_switch` | Parcela do tempo gasta enviando dados, na troca do protocolo e aguardando o Switch |
| `active` / `idle` | Tempo de instalação e tempo ocioso antes e depois dela |

Se `waiting_for_switch` estiver alto, o gargalo é o Switch (velocidade de gravação do microSD, descompactação de NSZ). Com **Depuração** ativada (no painel de registro), o registro mostra cada solicitação do DBI com o tamanho e a velocidade.

## Modo de linha de comando

```bash
dbibackend -cli ~/Switch          # wait for the Switch, serve one session, exit
dbibackend -cli -debug ~/Switch   # the same with a detailed log
dbibackend ~/Switch               # open the window with this folder
dbibackend -install-driver        # Windows: install the WinUSB driver for the connected Switch
```

No macOS, o servidor pode iniciar automaticamente quando o Switch é conectado: edite o caminho em `data/darwin/com.dbibackend.usb.agent.plist` e copie o arquivo para `~/Library/LaunchAgents/`.

## Compilação

Você precisa do Go 1.25+ e de um compilador C (o cgo é exigido pela libusb e pelo Fyne).

```bash
# macOS
brew install libusb pkg-config
# Debian/Ubuntu (plus the Fyne dependencies: https://docs.fyne.io/started/)
sudo apt install libusb-1.0-0-dev pkg-config

make build        # ./dbibackend for the current system
make test         # tests
make release      # dist/DBI Backend.app and dist/DBI Backend.exe
```

`make release` roda no macOS. Ele compila a libusb a partir do código-fonte e a vincula estaticamente, então o resultado não depende de nada instalado. O build para Windows também precisa de `brew install mingw-w64` e da ferramenta `fyne` (`go install fyne.io/tools/cmd/fyne@latest`).

## Estrutura do projeto

| Pacote | Finalidade |
|---|---|
| `dbi` | Protocolo DBI0 (LIST / FILE_RANGE / EXIT), varredura da pasta, estatísticas da sessão |
| `usbconn` | Abertura do Switch via libusb (gousb) nos modos de instalação via USB e MTP, transferências bulk |
| `mtp` | Cliente MTP (PTP sobre USB): armazenamentos, pastas, envio e download, arquivos com mais de 4 GB |
| `gui` | Interface Fyne, cliente FTP |
| `i18n` | Traduções (`i18n/locales/*.json`) |
| `winusb` | Instalação do WinUSB no Windows (SetupAPI) |
| `cmd/winusb-helper` | Auxiliar nativo para Windows ARM64 que instala o driver |

Para adicionar ou corrigir uma tradução, edite `i18n/locales/<code>.json`. `go test ./i18n/` verifica se todos os idiomas têm todas as chaves.

## Diferenças em relação ao original

- O Switch só pode solicitar arquivos da pasta escolhida. O original abria qualquer caminho enviado pelo dispositivo.
- O filtro de extensões foi corrigido: o original reconhecia `nsz` sem o ponto.
- Se arquivos em subpastas diferentes tiverem o mesmo nome, é usado o primeiro encontrado (o DBI só vê os nomes dos arquivos).

## Agradecimentos e licenças

- **Este projeto**: [MIT](LICENSE).
- [lunixoid/dbibackend](https://github.com/lunixoid/dbibackend) (MIT): a implementação original do protocolo.
- [DBI](https://github.com/rashevskyv/dbi), de duckbill: o instalador no Switch.
- A [libusb](https://libusb.info/) (LGPL-2.1) é vinculada estaticamente aos builds. O código-fonte deste projeto é aberto, então você pode recompilá-lo com sua própria versão da libusb.
- [Fyne](https://fyne.io/) (BSD-3-Clause), [gousb](https://github.com/google/gousb) (Apache-2.0), [zenity](https://github.com/ncruces/zenity) (MIT), [jlaffaye/ftp](https://github.com/jlaffaye/ftp) (ISC).

Este projeto não tem nenhuma afiliação com a Nintendo. Instale apenas jogos que você possui.
