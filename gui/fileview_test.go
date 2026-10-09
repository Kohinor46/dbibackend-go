package gui

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Kohinor46/dbibackend-go/i18n"
)

// walk calls fn for every object in the tree under root.
func walk(root fyne.CanvasObject, fn func(fyne.CanvasObject)) {
	fn(root)
	if c, ok := root.(*fyne.Container); ok {
		for _, child := range c.Objects {
			walk(child, fn)
		}
	}
	if w, ok := root.(fyne.Widget); ok {
		for _, child := range test.WidgetRenderer(w).Objects() {
			walk(child, fn)
		}
	}
}

// Every style shows the entries; tapping a folder opens it and a right
// click opens the entry's menu.
func TestFileStyles(t *testing.T) {
	defer i18n.Set("en")
	i18n.Set("ru")
	a := test.NewApp()
	defer a.Quit()
	for s := range numFileStyles {
		var entered []string
		h := viewHost{
			items:    func() []rEntry { return previewEntries },
			busy:     func() bool { return false },
			enter:    func(e rEntry) { entered = append(entered, e.Name) },
			download: func(rEntry) {},
			delete:   func(rEntry) {},
		}
		v := newFileView(s, h)
		w := a.NewWindow("")
		w.SetContent(v.obj)
		w.Resize(fyne.NewSize(700, 500))
		v.refresh()

		var boxes []*tapBox
		var texts []string
		walk(v.obj, func(o fyne.CanvasObject) {
			switch o := o.(type) {
			case *tapBox:
				boxes = append(boxes, o)
			case *widget.Label:
				texts = append(texts, o.Text)
			}
		})
		if len(boxes) < len(previewEntries) {
			t.Errorf("%s: %d rows for %d entries", s.name(), len(boxes), len(previewEntries))
			continue
		}
		found := false
		for _, tx := range texts {
			found = found || tx == previewEntries[2].Name
		}
		if !found {
			t.Errorf("%s: game name not shown", s.name())
		}

		test.Tap(boxes[0])
		if len(entered) != 1 || entered[0] != "atmosphere" {
			t.Errorf("%s: tapping a folder entered %v", s.name(), entered)
		}
		test.TapSecondary(boxes[2])
		if w.Canvas().Overlays().Top() == nil {
			t.Errorf("%s: no menu on right click", s.name())
		}
		w.Close()
	}
}

// The style is remembered, applied to the open browsers, and a bad stored
// value falls back to the default.
func TestFileStyleSetting(t *testing.T) {
	defer i18n.Set("en")
	i18n.Set("en")
	a := test.NewApp()
	u := newUI(a, false)
	u.build()
	if u.fileStyle != defaultFileStyle {
		t.Fatalf("default style = %v", u.fileStyle)
	}
	before := u.ftp.b.view.obj
	u.setFileStyle(styleTable)
	if a.Preferences().Int(prefFileStyle) != int(styleTable) || u.ftp.b.view.obj == before || u.ftp.b.viewBox.Objects[0] != u.ftp.b.view.obj {
		t.Error("style not applied to the FTP browser")
	}
	if newUI(a, false).fileStyle != styleTable {
		t.Error("style not remembered")
	}
	a.Preferences().SetInt(prefFileStyle, 42)
	if loadFileStyle(a.Preferences()) != defaultFileStyle {
		t.Error("bad stored style not ignored")
	}
	u.showSettings() // builds without panicking
}

// The appearance forces light or dark, is remembered and applied at start.
func TestAppearance(t *testing.T) {
	a := test.NewApp()
	u := newUI(a, false)
	u.build()
	bg := func() color.Color { return a.Settings().Theme().Color(theme.ColorNameBackground, theme.VariantDark) }
	light := theme.DefaultTheme().Color(theme.ColorNameBackground, theme.VariantLight)
	dark := theme.DefaultTheme().Color(theme.ColorNameBackground, theme.VariantDark)

	u.setAppearance(appearanceLight)
	if bg() != light {
		t.Error("light appearance not applied")
	}
	u.setAppearance(appearanceDark)
	if bg() != dark {
		t.Error("dark appearance not applied")
	}
	a.Settings().SetTheme(theme.DefaultTheme())
	if newUI(a, false).appearance != appearanceDark || bg() != dark {
		t.Error("appearance not restored at start")
	}
	u.setAppearance(appearanceSystem)
	if a.Settings().Theme() != theme.DefaultTheme() {
		t.Error("system appearance should use the default theme")
	}
}
