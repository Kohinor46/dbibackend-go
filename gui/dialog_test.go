package gui

import (
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/Kohinor46/dbibackend-go/i18n"
)

// Regression: the built-in folder dialog used to panic (Resize before Show).
func TestChooseDir(t *testing.T) {
	u := newUI(test.NewApp(), false)
	u.build()
	u.dirEntry.SetText(t.TempDir())
	u.chooseDirFyne()
	u.dirEntry.SetText("")
	u.chooseDirFyne()
}

// Switching language rebuilds the window and keeps folder, titles and state.
func TestLanguageRebuild(t *testing.T) {
	defer i18n.Set("en")
	i18n.Set("en")
	u := newUI(test.NewApp(), false)
	u.build()
	dir := t.TempDir()
	u.setDir(dir)
	u.setStatus("status.connected", 0)
	if u.startBtn.Text != "Start" {
		t.Fatalf("start = %q", u.startBtn.Text)
	}
	i18n.Set("ru")
	u.build()
	if u.dirEntry.Text != dir {
		t.Errorf("dir lost: %q", u.dirEntry.Text)
	}
	if u.startBtn.Text != "Старт" || u.status.Text != "Подключено" {
		t.Errorf("not translated: %q %q", u.startBtn.Text, u.status.Text)
	}
	if u.countLabel.Text != "0 шт., 0 Б" {
		t.Errorf("count = %q", u.countLabel.Text)
	}
}

// A typed path that no longer exists must not break the picker: it opens in
// the nearest existing parent.
func TestStartDir(t *testing.T) {
	base := t.TempDir()
	for in, want := range map[string]string{
		base:                                   base,
		"  " + base + "  ":                     base,
		filepath.Join(base, "renamed", "game"): base,
		"":                                     "",
		"relative/does/not/exist":              "",
	} {
		if got := startDir(in); got != want {
			t.Errorf("startDir(%q) = %q, want %q", in, got, want)
		}
	}
}
