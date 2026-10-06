# DBI Backend (Go)

<div dir="rtl">

[English](README.md) | [Русский](README.ru.md) | [Español](README.es.md) | [Italiano](README.it.md) | [Deutsch](README.de.md) | [Français](README.fr.md) | [Português](README.pt.md) | [中文](README.zh.md) | [日本語](README.ja.md) | [हिन्दी](README.hi.md) | **العربية**

تطبيق سطح مكتب لتثبيت الألعاب عبر USB على جهاز Nintendo Switch الذي يعمل عليه [DBI](https://github.com/rashevskyv/dbi). اختر مجلدًا يحتوي على ملفات `.nsp` / `.nsz` / `.xci`، ثم وصّل Switch وابدأ التثبيت.

وهو إعادة كتابة بلغة Go لمشروع [lunixoid/dbibackend](https://github.com/lunixoid/dbibackend) (سكربت Python)، مع واجهة رسومية وإصدارات جاهزة لنظامَي macOS وWindows، ودون الحاجة إلى تثبيت Python أو libusb.

![DBI Backend](docs/screenshot.png)

## الميزات

- **واجهة رسومية**: اختيار المجلد (عبر Finder أو مستكشف الملفات أو بالسحب والإفلات)، وقائمة بالملفات التي عُثر عليها مع أحجامها، وحالة الاتصال، وشريط تقدّم يعرض سرعة النقل، وسجلّ.
- **دون إعادة تشغيل**: بعد أن ينتهي DBI، ينتظر التطبيق اتصال Switch من جديد. ويتذكّر آخر مجلد، ويبدأ الخادم تلقائيًا عند فتح التطبيق.
- **11 لغة**: الإنجليزية والروسية والإسبانية والإيطالية والألمانية والفرنسية والبرتغالية والصينية واليابانية والهندية والعربية. تتبع اللغة إعدادات النظام، ويمكن تغييرها من النافذة.
- **برنامج تشغيل Windows مدمج**: عندما يرى التطبيق جهاز Switch دون برنامج تشغيل، يعرض تثبيته. وهذا يغني عن خطوة Zadig اليدوية (انظر أدناه).
- **الملفات عبر MTP على macOS وLinux**: تصفّح وحدات تخزين Switch، وارفع الملفات ونزّلها واحذفها، دون الحاجة إلى Android File Transfer (انظر أدناه).
- **وضع سطر الأوامر** (`-cli`) يعمل مثل السكربت الأصلي، لأغراض الأتمتة.
- **أكثر أمانًا من الأصل**: لا يستطيع Switch طلب الملفات إلا من المجلد المختار.

## التنزيل

الإصدارات الجاهزة متوفرة في صفحة [Releases](../../releases).

| النظام | الملف | ملاحظات |
|---|---|---|
| macOS 12 أو أحدث (Apple Silicon وIntel) | `DBI Backend.app` | غير موقّع بشهادة مطوّر من Apple. عند التشغيل الأول: انقر بزر الماوس الأيمن واختر **فتح**، أو نفّذ الأمر `xattr -dr com.apple.quarantine "DBI Backend.app"`. |
| Windows 10/11 (x64 وARM64) | `DBI Backend.exe` | ملف واحد، ولا حاجة إلى تثبيت أي شيء آخر. يُثبَّت برنامج تشغيل USB من داخل التطبيق (انظر أدناه). |
| Linux | البناء من المصدر | انظر [البناء من المصدر](#البناء-من-المصدر). |

## الاستخدام

1. شغّل التطبيق واختر المجلد الذي يحتوي على ألعابك. تُضمَّن المجلدات الفرعية أيضًا.
2. على Switch، افتح DBI واختر **Install title from USB**.
3. وصّل Switch بالحاسوب بكابل USB. ستتغير الحالة إلى **متصل**.
4. اختر الملفات في DBI وثبّتها. يظهر التقدّم والسرعة في أسفل النافذة.

## برنامج تشغيل USB في Windows

على Windows، لا يستطيع التطبيق التواصل مع Switch إلا عبر برنامج التشغيل WinUSB. وفي السابق كان عليك تثبيته يدويًا باستخدام [Zadig](https://zadig.akeo.ie/).

أما الآن فيتولى التطبيق ذلك بنفسه. عندما يرى جهاز Switch دون برنامج تشغيل، يعرض رسالة: انقر **تثبيت** ووافق على طلب أذونات المسؤول في Windows. ويمكنك أيضًا استخدام زر **تثبيت برنامج تشغيل USB** في النافذة في أي وقت. يجب أن يكون Switch موصولًا وأن يكون DBI في وضع **Install title from USB**.

آلية العمل: يعيّن التطبيق للجهاز برنامج التشغيل WinUSB المضمَّن في Windows والموقَّع من Microsoft، وهو ما يعادل في «إدارة الأجهزة» اختيار **تحديث برنامج التشغيل** ثم **السماح لي بالاختيار** ثم **WinUsb Device** (في النسخة الإنجليزية: `Update driver → Let me pick → WinUsb Device`). ولا يضيف أي شهادات إلى النظام، ويعمل حتى مع تفعيل Smart App Control.

## الملفات عبر MTP (macOS وLinux)

لا يدعم macOS بروتوكول MTP بشكل مدمج، لذا لا يظهر Switch في Finder عندما يشغّل DBI مستجيب MTP. كما أن Android File Transfer، وهو الحل المعتاد لذلك، لم يعد يُحدَّث. وتحل علامة التبويب **الملفات (MTP)** محله:

1. على Switch، افتح DBI واختر **Run MTP responder**، ثم وصّل الكابل.
2. في التطبيق، افتح علامة التبويب **الملفات (MTP)** وانقر **اتصال**.
3. اختر وحدة تخزين (بطاقة SD، وNAND، ووجهات التثبيت، وبيانات الحفظ وغيرها)، وافتح المجلدات، وارفع الملفات باستخدام **رفع ملفات…** أو بسحبها إلى النافذة. ويمكنك أيضًا تنزيل الملفات وحذفها وإنشاء مجلدات.

لتثبيت لعبة، ارفعها إلى وحدة تخزين يحتوي اسمها على «install»، إذ يثبّتها DBI أثناء وصولها. والملفات التي يتجاوز حجمها 4 GB مدعومة.

![الملفات (MTP)](docs/screenshot-mtp.png)

**«Switch قيد الاستخدام من قِبل برنامج آخر.»** على macOS، تستحوذ خدمة الكاميرا في النظام (`ptpcamerad`) على أجهزة MTP فور توصيلها. انقر **تحرير الجهاز**: يوقف التطبيق الخدمة ويتولى الاتصال بجهاز Switch، ثم يعيد macOS تشغيل الخدمة تلقائيًا عند الحاجة إليها. ويحتجز Android File Transfer الجهاز كذلك، لذا أغلقه أولًا.

على Windows تكون علامة التبويب مخفية، لأن مستكشف الملفات يعرض Switch في وضع MTP أصلًا.

## Linux

اسمح بالوصول إلى Switch دون صلاحيات root بتثبيت قاعدة udev (وهي تشمل وضعَي التثبيت عبر USB وMTP كليهما):

```bash
sudo cp data/linux/99-dbibackend.rules /etc/udev/rules.d/
sudo udevadm control --reload-rules
```

إذا أفادت علامة التبويب **الملفات (MTP)** بأن Switch قيد الاستخدام من قِبل برنامج آخر، فربما تكون بيئة سطح المكتب قد ركّبته (mount) تلقائيًا عبر gvfs: ألغِ تركيبه من مدير الملفات.

## سرعة النقل

تتحدد السرعة أساسًا بجهاز Switch، لا بالحاسوب:

- عبر **USB 2.0** يبلغ الحد الأقصى نحو **40 MB/s**. ومع ملفات `.nsp` / `.xci` غير المضغوطة، يُبقي التطبيق الناقل مشغولًا نحو 80% من الوقت، ويصل متوسط السرعة إلى نحو 32 MB/s.
- **ملفات `.nsz`** تُثبَّت أبطأ بنحو ثلاث مرات، لأن Switch يفك ضغط كل كتلة بنفسه.
- يطلب DBI البيانات على أجزاء لا يتجاوز كل منها 1 MB، ولا يطلب الجزء التالي إلا بعد معالجة الجزء السابق. ولا يستطيع الحاسوب تسريع ذلك.

في نهاية كل جلسة يعرض السجل سطر `Session stats`:

| الحقل | المعنى |
|---|---|
| `usb_MB/s` | السرعة أثناء إرسال البيانات |
| `avg_MB/s` | متوسط السرعة من الطلب الأول حتى الأخير |
| `time_data` / `time_handshake` / `waiting_for_switch` | نسبة الوقت المستغرق في إرسال البيانات، وفي تبادل البروتوكول، وفي انتظار Switch |
| `active` / `idle` | مدة التثبيت، ووقت الخمول قبله وبعده |

إذا كانت قيمة `waiting_for_switch` مرتفعة، فعنق الزجاجة هو Switch (سرعة الكتابة على بطاقة microSD، وفك ضغط NSZ). وعند تفعيل **تصحيح الأخطاء**، يعرض السجل كل طلب من DBI مع حجمه وسرعته.

## وضع سطر الأوامر

```bash
dbibackend -cli ~/Switch          # wait for the Switch, serve one session, exit
dbibackend -cli -debug ~/Switch   # the same with a detailed log
dbibackend ~/Switch               # open the window with this folder
dbibackend -install-driver        # Windows: install the WinUSB driver for the connected Switch
```

على macOS، يمكن أن يبدأ الخادم تلقائيًا عند توصيل Switch: عدّل المسار في `data/darwin/com.dbibackend.usb.agent.plist` وانسخه إلى `~/Library/LaunchAgents/`.

## البناء من المصدر

تحتاج إلى Go 1.25 أو أحدث ومترجم C (إذ تتطلب libusb وFyne وجود cgo).

```bash
# macOS
brew install libusb pkg-config
# Debian/Ubuntu (plus the Fyne dependencies: https://docs.fyne.io/started/)
sudo apt install libusb-1.0-0-dev pkg-config

make build        # ./dbibackend for the current system
make test         # tests
make release      # dist/DBI Backend.app and dist/DBI Backend.exe
```

يُشغَّل `make release` على macOS. فهو يبني libusb من المصدر ويربطها ربطًا ثابتًا، لذا لا تحتاج النتيجة إلى تثبيت أي شيء. ويتطلب بناء نسخة Windows أيضًا `brew install mingw-w64` وأداة `fyne` (`go install fyne.io/tools/cmd/fyne@latest`).

## بنية المشروع

| الحزمة | الغرض |
|---|---|
| `dbi` | بروتوكول DBI0 (LIST / FILE_RANGE / EXIT)، وفحص المجلد، وإحصاءات الجلسة |
| `usbconn` | فتح Switch عبر libusb (gousb) في وضعَي التثبيت عبر USB وMTP، والنقل المجمّع (bulk) |
| `mtp` | عميل MTP (PTP over USB): وحدات التخزين، والمجلدات، والرفع والتنزيل، والملفات التي يتجاوز حجمها 4 GB |
| `gui` | واجهة Fyne |
| `i18n` | الترجمات (`i18n/locales/*.json`) |
| `winusb` | تثبيت WinUSB على Windows (SetupAPI) |
| `cmd/winusb-helper` | أداة مساعدة أصلية لنظام Windows على ARM64 لتثبيت برنامج التشغيل |

لإضافة ترجمة أو تصحيحها، عدّل الملف `i18n/locales/<code>.json`. ويتحقق `go test ./i18n/` من وجود جميع المفاتيح في كل لغة.

## الاختلافات عن الأصل

- لا يستطيع Switch طلب الملفات إلا من المجلد المختار، بينما كان الأصل يفتح أي مسار يرسله الجهاز.
- تم إصلاح مرشّح الامتدادات: كان الأصل يطابق `nsz` دون النقطة.
- إذا وُجدت ملفات بالاسم نفسه في مجلدات فرعية مختلفة، يُستخدم أول ملف يُعثر عليه (لأن DBI لا يرى سوى أسماء الملفات).

## الشكر والتراخيص

- **هذا المشروع**: [MIT](LICENSE).
- [lunixoid/dbibackend](https://github.com/lunixoid/dbibackend) (MIT): التنفيذ الأصلي للبروتوكول.
- [DBI](https://github.com/rashevskyv/dbi) من تطوير duckbill: أداة التثبيت على Switch.
- [libusb](https://libusb.info/) (LGPL-2.1) مربوطة ربطًا ثابتًا في الإصدارات. الشيفرة المصدرية لهذا المشروع مفتوحة، لذا يمكنك إعادة بنائه بإصدارك الخاص من libusb.
- [Fyne](https://fyne.io/) (BSD-3-Clause)، و[gousb](https://github.com/google/gousb) (Apache-2.0)، و[zenity](https://github.com/ncruces/zenity) (MIT).

هذا المشروع غير تابع لشركة Nintendo. لا تثبّت إلا الألعاب التي تملكها.

</div>
