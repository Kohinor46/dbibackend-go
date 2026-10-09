package gui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/Kohinor46/dbibackend-go/i18n"
)

// contains reports whether o is in the tree under root.
func contains(root, o fyne.CanvasObject) bool {
	if root == o {
		return true
	}
	if c, ok := root.(*fyne.Container); ok {
		for _, child := range c.Objects {
			if contains(child, o) {
				return true
			}
		}
	}
	if w, ok := root.(fyne.Widget); ok {
		for _, child := range test.WidgetRenderer(w).Objects() {
			if contains(child, o) {
				return true
			}
		}
	}
	return false
}

// The log is shared by all tabs and starts collapsed to its latest line.
func TestLogPanel(t *testing.T) {
	defer i18n.Set("en")
	i18n.Set("en")
	u := newUI(test.NewApp(), false)
	u.build()
	content := func() fyne.CanvasObject { return u.win.Content() }
	if u.logOpen || contains(content(), u.logView.scroll) || !contains(content(), u.logView.last) {
		t.Fatal("log not collapsed at start")
	}
	u.logView.Write([]byte("first\nsecond\n"))
	u.logView.flush()
	if u.logView.last.Text != "second" {
		t.Errorf("collapsed line = %q", u.logView.last.Text)
	}

	test.Tap(u.logToggle)
	if !u.logOpen || !contains(content(), u.logView.scroll) || !contains(content(), u.tabs) {
		t.Fatal("log not expanded next to the tabs")
	}
	for i := range u.tabs.Items {
		u.tabs.SelectIndex(i) // every tab keeps the panel
		if !contains(content(), u.logView.scroll) {
			t.Errorf("tab %d hides the log", i)
		}
	}

	i18n.Set("ru") // a rebuild keeps the state
	u.build()
	if !u.logOpen || !contains(content(), u.logView.scroll) {
		t.Error("expanded log collapsed by a language change")
	}
	test.Tap(u.logToggle)
	if contains(content(), u.logView.scroll) {
		t.Error("log not collapsed again")
	}
}
