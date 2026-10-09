package gui

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/Kohinor46/dbibackend-go/i18n"
	"github.com/Kohinor46/dbibackend-go/mtp"
)

// writeTree creates files (path → content; a trailing "/" makes a folder).
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for p, content := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if strings.HasSuffix(p, "/") {
			os.MkdirAll(full, 0o755)
			continue
		}
		os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// child finds an object by exact name in a device folder.
func (f *fakeClient) child(parent uint32, name string) *fakeEntry {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, e := range f.objects {
		if e.obj.Info.Parent == parent && e.obj.Name() == name {
			return e
		}
	}
	return nil
}

func (f *fakeClient) names(parent uint32) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var n []string
	for _, e := range f.objects {
		if e.obj.Info.Parent == parent {
			n = append(n, e.obj.Name())
		}
	}
	sort.Strings(n)
	return n
}

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestUploadMergesFolders(t *testing.T) {
	f := &fakeClient{objects: map[uint32]*fakeEntry{}, next: 1}
	mods := f.add(mtp.ParentRoot, "mods", true, nil) // different case on the device
	f.add(mods, "a.txt", false, []byte("old a"))
	f.add(mods, "keep.txt", false, []byte("keep"))
	x := f.add(mods, "x", true, nil)
	f.add(x, "inside", false, []byte("i"))
	f.add(mods, "clash", false, []byte("a file"))

	src := t.TempDir()
	writeTree(t, src, map[string]string{
		"Mods/a.txt":       "new a",
		"Mods/sub/b.txt":   "b",
		"Mods/.DS_Store":   "junk",
		"Mods/._a.txt":     "junk",
		"Mods/x":           "file vs folder",
		"Mods/clash/c.txt": "folder vs file",
		"Mods/empty/":      "",
	})
	os.Symlink("/etc/hosts", filepath.Join(src, "Mods", "link"))

	items, total := collectLocal([]string{filepath.Join(src, "Mods")})
	if total != int64(len("new a")+len("b")+len("file vs folder")+len("folder vs file")) {
		t.Errorf("total = %d (junk or symlink counted?)", total)
	}
	u := newUploader(context.Background(), mtpFS{f}, mtpID(sdCard, mtp.ParentRoot), false, quietLog())
	if err := u.run(items); err != nil {
		t.Fatal(err)
	}

	if got := f.names(mtp.ParentRoot); len(got) != 1 || got[0] != "mods" {
		t.Errorf("root = %v: the folder must merge into the existing one, not be duplicated", got)
	}
	if got := f.names(mods); strings.Join(got, ",") != "a.txt,clash,empty,keep.txt,sub,x" {
		t.Errorf("mods = %v", got)
	}
	if a := f.child(mods, "a.txt"); a == nil || string(a.data) != "new a" {
		t.Error("a.txt was not replaced with the new content")
	}
	if k := f.child(mods, "keep.txt"); k == nil || string(k.data) != "keep" {
		t.Error("a file that isn't in the upload was touched")
	}
	if e := f.child(mods, "x"); e == nil || !e.obj.IsFolder() || len(f.names(x)) != 1 {
		t.Error("the existing folder x was replaced or emptied")
	}
	if e := f.child(mods, "clash"); e == nil || e.obj.IsFolder() || string(e.data) != "a file" {
		t.Error("the existing file clash was replaced")
	}
	sub := f.child(mods, "sub")
	if sub == nil || f.child(sub.obj.Handle, "b.txt") == nil {
		t.Error("new subfolder with b.txt missing")
	}
	want := uploadStats{uploaded: 2, replaced: 1, skipped: 3} // x file, clash folder, clash/c.txt
	if u.stats != want {
		t.Errorf("stats = %+v, want %+v", u.stats, want)
	}
}

func TestUploadFlatIntoInstallStorage(t *testing.T) {
	f := &fakeClient{objects: map[uint32]*fakeEntry{}, next: 1}
	src := t.TempDir()
	writeTree(t, src, map[string]string{
		"Games/a.nsp":         "a",
		"Games/Updates/a.nsp": "a update",
		"Games/Updates/b.nsp": "b",
		"Games/.DS_Store":     "junk",
	})
	items, _ := collectLocal([]string{filepath.Join(src, "Games")})
	u := newUploader(context.Background(), mtpFS{f}, mtpID(sdCard, mtp.ParentRoot), true, quietLog())
	if err := u.run(items); err != nil {
		t.Fatal(err)
	}
	if got := f.names(mtp.ParentRoot); strings.Join(got, ",") != "a.nsp,b.nsp" {
		t.Errorf("install root = %v: no folders, files only", got)
	}
	if u.stats.uploaded != 3 || u.stats.replaced != 1 {
		t.Errorf("stats = %+v", u.stats)
	}
}

// Dropping a folder on the tab uploads it with the merge rules and reports the result.
func TestMTPTabUploadFolder(t *testing.T) {
	defer i18n.Set("en")
	i18n.Set("en")
	defer func(orig func(func())) { runOnUI = orig }(runOnUI)
	runOnUI = func(fn func()) { uiQueue <- fn }
	fake := newFakeClient() // root: Games/ (with a.xci), b.nsp
	defer func(orig func(context.Context, *slog.Logger, func(error), <-chan struct{}) (mtpClient, func(), error)) {
		connectMTP = orig
	}(connectMTP)
	connectMTP = func(context.Context, *slog.Logger, func(error), <-chan struct{}) (mtpClient, func(), error) {
		return fake, func() {}, nil
	}
	u := newUI(test.NewApp(), false)
	u.build()
	m := u.mtp
	m.connect()
	waitFor(t, "connect", func() bool { return m.client != nil && !m.b.busy })

	src := t.TempDir()
	writeTree(t, src, map[string]string{"Games/new.nsp": "new", "Games/a.xci": "replaced"})
	m.b.upload([]string{filepath.Join(src, "Games")})
	waitFor(t, "upload", func() bool { return !m.b.busy })

	games := fake.child(mtp.ParentRoot, "Games")
	if got := fake.names(games.obj.Handle); strings.Join(got, ",") != "a.xci,new.nsp" {
		t.Errorf("Games = %v", got)
	}
	if got := fake.names(mtp.ParentRoot); strings.Join(got, ",") != "Games,b.nsp" {
		t.Errorf("root = %v", got)
	}
}
