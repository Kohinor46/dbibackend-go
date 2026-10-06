# DBI Backend (Go)

[English](README.md) | [Русский](README.ru.md) | [Español](README.es.md) | [Italiano](README.it.md) | [Deutsch](README.de.md) | [Français](README.fr.md) | [Português](README.pt.md) | [中文](README.zh.md) | [日本語](README.ja.md) | **हिन्दी** | [العربية](README.ar.md)

यह एक डेस्कटॉप ऐप है, जिससे आप USB के ज़रिए [DBI](https://github.com/rashevskyv/dbi) चलाने वाले Nintendo Switch पर गेम इंस्टॉल कर सकते हैं। `.nsp` / `.nsz` / `.xci` फ़ाइलों वाला फ़ोल्डर चुनें, Switch कनेक्ट करें और इंस्टॉल करें।

यह [lunixoid/dbibackend](https://github.com/lunixoid/dbibackend) (एक Python स्क्रिप्ट) का Go में दोबारा लिखा गया संस्करण है। इसमें ग्राफ़िकल इंटरफ़ेस है, macOS और Windows के लिए तैयार बिल्ड मिलते हैं, और Python या libusb इंस्टॉल करने की ज़रूरत नहीं पड़ती।

![DBI Backend](docs/screenshot.png)

## सुविधाएँ

- **ग्राफ़िकल इंटरफ़ेस**: फ़ोल्डर चुनने का विकल्प (Finder, Explorer या ड्रैग एंड ड्रॉप), मिली हुई फ़ाइलों की सूची उनके साइज़ के साथ, कनेक्शन की स्थिति, ट्रांसफ़र स्पीड के साथ प्रोग्रेस बार, और लॉग।
- **रीस्टार्ट की ज़रूरत नहीं**: DBI का काम पूरा होने के बाद ऐप फिर से Switch का इंतज़ार करता है। आख़िरी फ़ोल्डर याद रहता है, और ऐप खुलते ही सर्वर अपने-आप शुरू हो जाता है।
- **11 भाषाएँ**: अंग्रेज़ी, रूसी, स्पैनिश, इतालवी, जर्मन, फ़्रेंच, पुर्तगाली, चीनी, जापानी, हिन्दी और अरबी। भाषा सिस्टम के हिसाब से चुनी जाती है और विंडो में बदली भी जा सकती है।
- **Windows ड्राइवर साथ में**: जब ऐप को Switch दिखता है लेकिन उसका ड्राइवर नहीं होता, तो ऐप ड्राइवर इंस्टॉल करने का विकल्प देता है। इससे Zadig वाला मैन्युअल स्टेप ख़त्म हो जाता है (नीचे देखें)।
- **कमांड-लाइन मोड** (`-cli`), जो मूल स्क्रिप्ट की तरह काम करता है — ऑटोमेशन के लिए।
- **मूल से ज़्यादा सुरक्षित**: Switch सिर्फ़ चुने गए फ़ोल्डर की फ़ाइलें ही माँग सकता है।

## डाउनलोड

तैयार बिल्ड [Releases](../../releases) पेज पर उपलब्ध हैं।

| सिस्टम | फ़ाइल | नोट्स |
|---|---|---|
| macOS 12+ (Apple Silicon और Intel) | `DBI Backend.app` | Apple डेवलपर सर्टिफ़िकेट से साइन नहीं किया गया है। पहली बार खोलते समय: राइट-क्लिक करके **खोलें** (Open) चुनें, या `xattr -dr com.apple.quarantine "DBI Backend.app"` चलाएँ। |
| Windows 10/11 (x64 और ARM64) | `DBI Backend.exe` | सिर्फ़ एक फ़ाइल, और कुछ इंस्टॉल करने की ज़रूरत नहीं। USB ड्राइवर ऐप से ही इंस्टॉल होता है (नीचे देखें)। |
| Linux | सोर्स से बिल्ड करें | [बिल्ड करना](#बिल्ड-करना) देखें। |

## इस्तेमाल का तरीका

1. ऐप शुरू करें और अपने गेम वाला फ़ोल्डर चुनें। सब-फ़ोल्डर भी शामिल होते हैं।
2. Switch पर DBI खोलें और **Install title from USB** चुनें।
3. Switch को USB केबल से कंप्यूटर से कनेक्ट करें। स्थिति बदलकर **कनेक्ट हो गया** हो जाएगी।
4. DBI में फ़ाइलें चुनें और उन्हें इंस्टॉल करें। प्रोग्रेस और स्पीड विंडो में नीचे दिखती है।

## Windows: USB ड्राइवर

Windows पर ऐप Switch से सिर्फ़ WinUSB ड्राइवर के ज़रिए ही बात कर सकता है। पहले इसे [Zadig](https://zadig.akeo.ie/) से हाथ से इंस्टॉल करना पड़ता था।

अब ऐप यह काम ख़ुद करता है। जब उसे बिना ड्राइवर वाला Switch दिखता है, तो एक संदेश आता है: **इंस्टॉल करें** पर क्लिक करें और Windows के एडमिनिस्ट्रेटर अनुमति वाले अनुरोध की पुष्टि करें। आप विंडो में **USB ड्राइवर इंस्टॉल करें** बटन का इस्तेमाल भी कभी भी कर सकते हैं। इसके लिए Switch कनेक्ट होना चाहिए और DBI **Install title from USB** मोड में होना चाहिए।

यह कैसे काम करता है: ऐप वही WinUSB ड्राइवर सेट करता है जो Windows के साथ आता है और Microsoft द्वारा साइन किया गया है (यह Device Manager में **Update driver → Let me pick → WinUsb Device** करने के बराबर है)। यह सिस्टम में कोई सर्टिफ़िकेट नहीं जोड़ता, और Smart App Control चालू होने पर भी काम करता है।

## Linux

udev नियम इंस्टॉल करके बिना root के Switch तक पहुँच दें:

```bash
sudo cp data/linux/99-dbibackend.rules /etc/udev/rules.d/
sudo udevadm control --reload-rules
```

## ट्रांसफ़र स्पीड

स्पीड मुख्य रूप से Switch की वजह से सीमित होती है, कंप्यूटर की वजह से नहीं:

- **USB 2.0** पर अधिकतम स्पीड लगभग **40 MB/s** है। बिना कंप्रेशन वाली `.nsp` / `.xci` फ़ाइलों पर ऐप लगभग 80% समय बस को व्यस्त रखता है और औसतन लगभग 32 MB/s तक पहुँचता है।
- **`.nsz` फ़ाइलें** लगभग तीन गुना धीमी इंस्टॉल होती हैं: Switch हर ब्लॉक को ख़ुद डीकंप्रेस करता है।
- DBI डेटा को अधिकतम 1 MB के टुकड़ों में माँगता है, और अगला टुकड़ा तभी माँगता है जब पिछला प्रोसेस हो चुका हो। कंप्यूटर इसे तेज़ नहीं कर सकता।

हर सेशन के अंत में लॉग में `Session stats` लाइन दिखती है:

| फ़ील्ड | मतलब |
|---|---|
| `usb_MB/s` | डेटा भेजे जाते समय की स्पीड |
| `avg_MB/s` | पहले अनुरोध से आख़िरी अनुरोध तक की औसत स्पीड |
| `time_data` / `time_handshake` / `waiting_for_switch` | डेटा भेजने, प्रोटोकॉल के आदान-प्रदान और Switch के इंतज़ार में लगे समय का हिस्सा |
| `active` / `idle` | इंस्टॉल में लगा समय, और उससे पहले व बाद का खाली (idle) समय |

अगर `waiting_for_switch` ज़्यादा है, तो रुकावट Switch में है (microSD की राइट स्पीड, NSZ डीकंप्रेशन)। **डीबग** चालू करने पर लॉग में DBI का हर अनुरोध उसके साइज़ और स्पीड के साथ दिखता है।

## कमांड-लाइन मोड

```bash
dbibackend -cli ~/Switch          # wait for the Switch, serve one session, exit
dbibackend -cli -debug ~/Switch   # the same with a detailed log
dbibackend ~/Switch               # open the window with this folder
dbibackend -install-driver        # Windows: install the WinUSB driver for the connected Switch
```

macOS पर Switch कनेक्ट होते ही सर्वर अपने-आप शुरू हो सकता है: `data/darwin/com.dbibackend.usb.agent.plist` में पाथ बदलें और फ़ाइल को `~/Library/LaunchAgents/` में कॉपी करें।

## बिल्ड करना

आपको Go 1.25+ और एक C कंपाइलर चाहिए (libusb और Fyne के लिए cgo ज़रूरी है)।

```bash
# macOS
brew install libusb pkg-config
# Debian/Ubuntu (plus the Fyne dependencies: https://docs.fyne.io/started/)
sudo apt install libusb-1.0-0-dev pkg-config

make build        # ./dbibackend for the current system
make test         # tests
make release      # dist/DBI Backend.app and dist/DBI Backend.exe
```

`make release` macOS पर चलता है। यह libusb को सोर्स से बिल्ड करके स्टैटिक रूप से लिंक करता है, इसलिए तैयार ऐप के लिए कुछ भी इंस्टॉल करने की ज़रूरत नहीं पड़ती। Windows बिल्ड के लिए `brew install mingw-w64` और `fyne` टूल (`go install fyne.io/tools/cmd/fyne@latest`) भी चाहिए।

## प्रोजेक्ट की संरचना

| पैकेज | काम |
|---|---|
| `dbi` | DBI0 प्रोटोकॉल (LIST / FILE_RANGE / EXIT), फ़ोल्डर स्कैन, सेशन के आँकड़े |
| `usbconn` | libusb (gousb) के ज़रिए Switch को खोलना, bulk ट्रांसफ़र |
| `gui` | Fyne इंटरफ़ेस |
| `i18n` | अनुवाद (`i18n/locales/*.json`) |
| `winusb` | Windows पर WinUSB इंस्टॉल करना (SetupAPI) |
| `cmd/winusb-helper` | ड्राइवर इंस्टॉल करने के लिए Windows ARM64 का नेटिव हेल्पर |

कोई अनुवाद जोड़ने या सुधारने के लिए `i18n/locales/<code>.json` एडिट करें। `go test ./i18n/` जाँचता है कि हर भाषा में सभी keys मौजूद हैं।

## मूल संस्करण से अंतर

- Switch सिर्फ़ चुने गए फ़ोल्डर की फ़ाइलें माँग सकता है। मूल संस्करण डिवाइस से आया कोई भी पाथ खोल देता था।
- एक्सटेंशन फ़िल्टर ठीक किया गया है: मूल संस्करण `nsz` को बिना डॉट के मिलाता था।
- अगर अलग-अलग सब-फ़ोल्डरों में एक ही नाम की फ़ाइलें हों, तो सबसे पहले मिली फ़ाइल इस्तेमाल होती है (DBI को सिर्फ़ फ़ाइलों के नाम दिखते हैं)।

## आभार और लाइसेंस

- **यह प्रोजेक्ट**: [MIT](LICENSE)।
- [lunixoid/dbibackend](https://github.com/lunixoid/dbibackend) (MIT): प्रोटोकॉल का मूल इम्प्लीमेंटेशन।
- duckbill का [DBI](https://github.com/rashevskyv/dbi): Switch पर चलने वाला इंस्टॉलर।
- [libusb](https://libusb.info/) (LGPL-2.1) बिल्ड में स्टैटिक रूप से लिंक की गई है। इस प्रोजेक्ट का सोर्स कोड खुला है, इसलिए आप इसे libusb के अपने संस्करण के साथ दोबारा बिल्ड कर सकते हैं।
- [Fyne](https://fyne.io/) (BSD-3-Clause), [gousb](https://github.com/google/gousb) (Apache-2.0), [zenity](https://github.com/ncruces/zenity) (MIT)।

इस प्रोजेक्ट का Nintendo से कोई संबंध नहीं है। सिर्फ़ वही गेम इंस्टॉल करें जो आपके अपने हैं।
