package gui

import (
	"context"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/spf13/afero"

	"github.com/Kohinor46/dbibackend-go/i18n"
)

func TestLatinName(t *testing.T) {
	for in, want := range map[string]string{
		"Паспорт Надя.pdf":     "Pasport Nadya.pdf",
		"ОМС Ромик.pdf":        "OMS Romik.pdf",
		"Щука и Ёжик.txt":      "Shchuka i Ezhik.txt",
		"Йод, объём":           "Yod, obem",
		"Їжак Ґанок":           "Yizhak Ganok",
		"café.txt":             "cafe.txt",
		"café.txt":            "cafe.txt", // decomposed, as macOS stores it
		"й̆":                   "y_",       // a stray accent after й can't be kept
		"日本.txt":               "__.txt",
		"a:b?.txt":             "a_b_.txt",
		"Game [0100] (v2).nsp": "Game [0100] (v2).nsp",
		"Конец. ":              "Konets__",
	} {
		if got := latinName(in); got != want {
			t.Errorf("latinName(%q) = %q, want %q", in, got, want)
		}
	}
	if !isLatinName("a b-c_(1) [x].txt") || isLatinName("тест") || isLatinName("café") || isLatinName("a:b") {
		t.Error("isLatinName")
	}
	if numbered("a.pdf", 2) != "a (2).pdf" || numbered("README", 3) != "README (3)" || numbered(".hidden", 2) != ".hidden (2)" {
		t.Error("numbered")
	}
}

// Renaming gives every item a Latin name, numbering clashes within the
// upload; skipping leaves the non-Latin ones out.
func TestUploaderNames(t *testing.T) {
	src := t.TempDir()
	writeTree(t, src, map[string]string{
		"Архив/Pasport.pdf": "latin",
		"Архив/Паспорт.pdf": "cyrillic",
		"Архив/Фото/1.jpg":  "photo",
		"Архив/ok.txt":      "ok",
	})
	items, _ := collectLocal([]string{filepath.Join(src, "Архив")})
	ctx := context.Background()

	for _, tc := range []struct {
		mode  nameMode
		files map[string]string
		stats uploadStats
	}{
		{namesLatin, map[string]string{
			"/Arkhiv/Pasport.pdf": "latin", "/Arkhiv/Pasport (2).pdf": "cyrillic",
			"/Arkhiv/Foto/1.jpg": "photo", "/Arkhiv/ok.txt": "ok",
		}, uploadStats{uploaded: 4}},
		{namesSkip, map[string]string{}, uploadStats{skipped: 6}}, // the folder holds them all
	} {
		d := &testFTPDriver{fs: afero.NewMemMapFs()}
		addr := startFTP(t, d)
		f, err := dialFTP(ctx, addr, "", "", quietLog())
		if err != nil {
			t.Fatal(err)
		}
		u := newUploader(ctx, f, "/", false, quietLog())
		u.names = tc.mode
		if err := u.run(items); err != nil {
			t.Fatal(err)
		}
		for name, want := range tc.files {
			if got := readFile(t, d.fs, name); got != want {
				t.Errorf("mode %d: %s = %q", tc.mode, name, got)
			}
		}
		if u.stats != tc.stats {
			t.Errorf("mode %d: stats = %+v", tc.mode, u.stats)
		}
		f.Close()
	}
}

// Over FTP the tab asks before uploading non-Latin names.
func TestFTPAsksAboutNames(t *testing.T) {
	defer i18n.Set("en")
	i18n.Set("en")
	defer func(orig func(func())) { runOnUI = orig }(runOnUI)
	runOnUI = func(fn func()) { uiQueue <- fn }
	d := &testFTPDriver{fs: afero.NewMemMapFs()}
	addr := startFTP(t, d)
	u := newUI(test.NewApp(), false)
	u.build()
	ft := u.ftp
	ft.host.SetText(addr)
	ft.connect()
	waitFor(t, "connect", func() bool { return ft.fs != nil && !ft.b.busy })

	src := t.TempDir()
	writeTree(t, src, map[string]string{"Надя.txt": "n", "plain.txt": "p"})
	ft.b.upload([]string{filepath.Join(src, "Надя.txt"), filepath.Join(src, "plain.txt")})
	top := u.win.Canvas().Overlays().Top()
	if top == nil || ft.b.busy {
		t.Fatal("no question before uploading")
	}
	var rename *widget.Button
	walk(top, func(o fyne.CanvasObject) {
		if b, ok := o.(*widget.Button); ok && b.Text == "Rename" {
			rename = b
		}
	})
	if rename == nil {
		t.Fatal("no Rename button")
	}
	test.Tap(rename)
	waitFor(t, "upload", func() bool { return !ft.b.busy && ft.b.pending == nil })
	if readFile(t, d.fs, "/Nadya.txt") != "n" || readFile(t, d.fs, "/plain.txt") != "p" {
		t.Error("files not uploaded as expected")
	}
	ft.disconnect()
}
