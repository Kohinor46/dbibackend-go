package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Kohinor46/dbibackend-go/i18n"
)

// appearance is the colour scheme chosen in Settings.
type appearance int

const (
	appearanceSystem appearance = iota // follow the system's light/dark mode
	appearanceLight
	appearanceDark

	numAppearances

	prefAppearance = "appearance"
)

func (a appearance) name() string {
	return i18n.T([...]string{"appearance.system", "appearance.light", "appearance.dark"}[a])
}

func loadAppearance(p fyne.Preferences) appearance {
	a := appearance(p.IntWithFallback(prefAppearance, int(appearanceSystem)))
	if a < 0 || a >= numAppearances {
		return appearanceSystem
	}
	return a
}

// fixedVariant is the default theme always in one variant (light or dark),
// whatever the system uses.
type fixedVariant struct {
	fyne.Theme
	variant fyne.ThemeVariant
}

func (t fixedVariant) Color(n fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	return t.Theme.Color(n, t.variant)
}

func (a appearance) theme() fyne.Theme {
	switch a {
	case appearanceLight:
		return fixedVariant{theme.DefaultTheme(), theme.VariantLight}
	case appearanceDark:
		return fixedVariant{theme.DefaultTheme(), theme.VariantDark}
	}
	return theme.DefaultTheme()
}

// previewEntries fill the style preview in Settings.
var previewEntries = []rEntry{
	{Name: "atmosphere", Dir: true},
	{Name: "switch", Dir: true},
	{Name: "Game [0100000000010000][v0].nsp", Size: 16_900 << 20},
	{Name: "Game [0100000000020000][v0].nsz", Size: 7_400 << 20},
	{Name: "Game [0100000000030000].xci", Size: 4_300 << 20},
	{Name: "hbmenu.nro", Size: 1_258_000},
}

// showSettings opens the Settings dialog: the file view style, with a
// live preview. A change applies at once and is remembered.
func (u *ui) showSettings() {
	T := i18n.T
	host := viewHost{
		items:    func() []rEntry { return previewEntries },
		busy:     func() bool { return false },
		enter:    func(rEntry) {},
		download: func(rEntry) {},
		delete:   func(rEntry) {},
	}
	preview := container.NewStack(newFileView(u.fileStyle, host).obj)
	redraw := func() {
		preview.Objects = []fyne.CanvasObject{newFileView(u.fileStyle, host).obj}
		preview.Refresh()
	}

	looks := make([]string, numAppearances)
	for a := range numAppearances {
		looks[a] = a.name()
	}
	look := widget.NewRadioGroup(looks, nil)
	look.Horizontal = true
	look.Required = true
	look.SetSelected(u.appearance.name())
	look.OnChanged = func(name string) {
		for a := range numAppearances {
			if a.name() == name && a != u.appearance {
				u.setAppearance(a)
				redraw()
			}
		}
	}

	names := make([]string, numFileStyles)
	for s := range numFileStyles {
		names[s] = s.name()
	}
	styles := widget.NewRadioGroup(names, nil)
	styles.Required = true
	styles.SetSelected(u.fileStyle.name())
	styles.OnChanged = func(name string) {
		for s := range numFileStyles {
			if s.name() == name && s != u.fileStyle {
				u.setFileStyle(s)
				redraw()
			}
		}
	}

	heading := func(key string) *widget.Label {
		return widget.NewLabelWithStyle(T(key), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	}
	top := container.NewVBox(heading("settings.appearance"), look, widget.NewSeparator(), heading("settings.file_style"))
	content := container.NewBorder(top, nil, container.NewVBox(styles), nil, preview)
	d := dialog.NewCustom(T("settings.title"), T("settings.close"), content, u.win)
	d.Resize(fyne.NewSize(720, 560))
	d.Show()
}

// setFileStyle switches the MTP and FTP browsers to style s.
func (u *ui) setFileStyle(s fileStyle) {
	u.fileStyle = s
	u.app.Preferences().SetInt(prefFileStyle, int(s))
	u.mtp.b.restyle()
	u.ftp.b.restyle()
}

// setAppearance switches the colour scheme. The file views are rebuilt:
// some of their texts take their colour when created.
func (u *ui) setAppearance(a appearance) {
	u.appearance = a
	u.app.Preferences().SetInt(prefAppearance, int(a))
	u.app.Settings().SetTheme(a.theme())
	u.mtp.b.restyle()
	u.ftp.b.restyle()
}
