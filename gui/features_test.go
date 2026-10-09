package gui

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/spf13/afero"

	"github.com/Kohinor46/dbibackend-go/i18n"
	"github.com/Kohinor46/dbibackend-go/mtp"
)

func TestNewerVersion(t *testing.T) {
	for _, tc := range []struct {
		cur, latest string
		want        bool
	}{
		{"1.3.0", "v1.3.1", true},
		{"1.3.0", "v1.4.0", true},
		{"1.3.0", "2.0.0", true},
		{"1.3.0", "v1.3.0", false},
		{"1.3.0", "v1.2.9", false},
		{"1.10.0", "v1.9.0", false},
		{"1.3.0", "v1.4.0-beta", false}, // not a plain release tag
		{"1.3.0", "", false},
	} {
		if got := newerVersion(tc.cur, tc.latest); got != tc.want {
			t.Errorf("newerVersion(%q, %q) = %v", tc.cur, tc.latest, got)
		}
	}
}

// A newer release shows a button; the check can be turned off.
func TestUpdateCheck(t *testing.T) {
	defer i18n.Set("en")
	i18n.Set("en")
	defer func(orig func(func())) { runOnUI = orig }(runOnUI)
	runOnUI = func(fn func()) { uiQueue <- fn }
	defer func(orig func(context.Context) (string, string, error)) { latestRelease = orig }(latestRelease)
	calls := 0
	latestRelease = func(context.Context) (string, string, error) {
		calls++
		return "v99.0.0", "https://github.com/Kohinor46/dbibackend-go/releases/tag/v99.0.0", nil
	}
	a := test.NewApp()
	u := newUI(a, false)
	u.build()
	u.checkUpdate()
	waitFor(t, "update found", func() bool { return u.updateTag == "99.0.0" })
	found := false
	walk(u.win.Content(), func(o fyne.CanvasObject) {
		if btn, ok := o.(*widget.Button); ok && strings.Contains(btn.Text, "99.0.0") {
			found = true
		}
	})
	if !found {
		t.Error("no update button")
	}

	a.Preferences().SetBool(prefCheckUpdates, false)
	newUI(a, false).checkUpdate()
	if calls != 1 {
		t.Errorf("checked %d times with the check turned off", calls)
	}
}

// Notifications follow operations that took a while, if they are on.
func TestNotify(t *testing.T) {
	defer i18n.Set("en")
	i18n.Set("en")
	a := test.NewApp()
	u := newUI(a, false)
	var got []string
	u.sendNotification = func(n *fyne.Notification) { got = append(got, n.Content) }
	long := time.Now().Add(-notifyMin - time.Second)
	u.notifyAfter(time.Now(), "notify.install_done", 2) // quick: the user saw it
	u.notifyAfter(long, "notify.install_done", 3)
	a.Preferences().SetBool(prefNotify, false)
	u.notifyAfter(long, "notify.install_done", 4)
	if len(got) != 1 || got[0] != "Installation finished: 3 file(s)" {
		t.Errorf("notifications = %q", got)
	}
}

// The panel shows the last lines; Copy and Save get the whole log.
func TestLogKeepsAndSaves(t *testing.T) {
	u := newUI(test.NewApp(), false)
	u.build()
	for i := range maxLogLines + 100 {
		fmt.Fprintf(u.logView, "line %d\n", i)
	}
	u.logView.flush()
	if shown := strings.Count(u.logView.label.Text, "\n") + 1; shown != maxLogLines {
		t.Errorf("panel shows %d lines", shown)
	}
	dest := filepath.Join(t.TempDir(), "log.txt")
	u.saveLog(dest)
	b, err := os.ReadFile(dest)
	if err != nil || !strings.HasPrefix(string(b), "line 0\n") || !strings.Contains(string(b), fmt.Sprintf("line %d\n", maxLogLines+99)) {
		t.Errorf("saved log: %v, %d bytes", err, len(b))
	}
}

// Resuming keeps the files that arrived whole, sends again the one that was
// in flight, and replaces partial ones.
func TestUploaderResume(t *testing.T) {
	d := &testFTPDriver{fs: afero.NewMemMapFs()}
	afero.WriteFile(d.fs, "/Mods/a.txt", []byte("aaaa"), 0o644) // arrived
	afero.WriteFile(d.fs, "/Mods/b.txt", []byte("xxxx"), 0o644) // in flight: full size, wrong data
	afero.WriteFile(d.fs, "/Mods/c.txt", []byte("cc"), 0o644)   // partial
	addr := startFTP(t, d)
	ctx := context.Background()
	f, err := dialFTP(ctx, addr, "", "", quietLog())
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	src := t.TempDir()
	writeTree(t, src, map[string]string{"Mods/a.txt": "aaaa", "Mods/b.txt": "bbbb", "Mods/c.txt": "cccc", "Mods/d.txt": "dddd"})
	items, _ := collectLocal([]string{filepath.Join(src, "Mods")})
	u := newUploader(ctx, f, "/", false, quietLog())
	u.resume, u.redo = true, "Mods/b.txt"
	if err := u.run(items); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"a": "aaaa", "b": "bbbb", "c": "cccc", "d": "dddd"} {
		if got := readFile(t, d.fs, "/Mods/"+name+".txt"); got != want {
			t.Errorf("%s.txt = %q", name, got)
		}
	}
	if u.stats != (uploadStats{uploaded: 3, replaced: 2, present: 1}) {
		t.Errorf("stats = %+v", u.stats)
	}
}

// A broken-off upload offers Continue, also after reconnecting, into the
// same folder found by name.
func TestResumeAfterReconnect(t *testing.T) {
	defer i18n.Set("en")
	i18n.Set("en")
	defer func(orig func(func())) { runOnUI = orig }(runOnUI)
	runOnUI = func(fn func()) { uiQueue <- fn }
	d := &testFTPDriver{fs: afero.NewMemMapFs()}
	afero.WriteFile(d.fs, "/games/x.nsp", []byte("x"), 0o644)
	addr := startFTP(t, d)
	u := newUI(test.NewApp(), false)
	u.build()
	ft := u.ftp
	ft.host.SetText(addr)

	src := t.TempDir()
	writeTree(t, src, map[string]string{"new.nsp": "new"})
	ft.connect()
	waitFor(t, "connect", func() bool { return ft.fs != nil && !ft.b.busy })
	ft.b.pending = &pendingUpload{paths: []string{filepath.Join(src, "new.nsp")}, rootID: "/", dirs: []string{"Games"}, redo: "new.nsp"}
	ft.disconnect()
	if ft.b.resumeBar.Visible() {
		t.Error("Continue shown while disconnected")
	}
	ft.connect()
	waitFor(t, "reconnect", func() bool { return ft.fs != nil && !ft.b.busy })
	if !ft.b.resumeBar.Visible() || !strings.Contains(ft.b.resumeLbl.Text, "new.nsp") {
		t.Fatalf("resume bar visible=%v text=%q", ft.b.resumeBar.Visible(), ft.b.resumeLbl.Text)
	}
	ft.b.resumeUpload()
	waitFor(t, "resume", func() bool { return !ft.b.busy && ft.b.pending == nil })
	if got := readFile(t, d.fs, "/games/new.nsp"); got != "new" {
		t.Errorf("resumed into the wrong place: %q", got)
	}
	if ft.b.resumeBar.Visible() {
		t.Error("Continue still shown after success")
	}
	ft.disconnect()
}

func TestSafeName(t *testing.T) {
	for in, want := range map[string]string{
		"save.dat":    "save.dat",
		"a:b?c*.bin":  "a_b_c_.bin",
		"trailing. ":  "trailing__",
		"..":          "_",
		"":            "_",
		"Zelda – 1/2": "Zelda – 1_2",
	} {
		if got := safeName(in); got != want {
			t.Errorf("safeName(%q) = %q, want %q", in, got, want)
		}
	}
}

// A folder comes down with its subfolders (empty ones too).
func TestDownloadFolder(t *testing.T) {
	d := &testFTPDriver{fs: afero.NewMemMapFs()}
	afero.WriteFile(d.fs, "/saves/Zelda/slot1.dat", []byte("one"), 0o644)
	afero.WriteFile(d.fs, "/saves/Mario Kart/a:b.dat", bytes.Repeat([]byte{7}, 70_000), 0o644)
	d.fs.MkdirAll("/saves/Empty", 0o755)
	addr := startFTP(t, d)
	ctx := context.Background()
	f, err := dialFTP(ctx, addr, "", "", quietLog())
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	dest := filepath.Join(t.TempDir(), "backup")
	var last, total int64
	td := &treeDownload{ctx: ctx, fs: f, progress: func(d, t int64) { last, total = d, t }, onFile: func(string, int, int) {}}
	n, err := td.run("/saves", dest)
	if err != nil || n != 2 {
		t.Fatalf("files = %d, %v", n, err)
	}
	if b, _ := os.ReadFile(filepath.Join(dest, "Zelda", "slot1.dat")); string(b) != "one" {
		t.Errorf("slot1.dat = %q", b)
	}
	if st, err := os.Stat(filepath.Join(dest, "Mario Kart", "a_b.dat")); err != nil || st.Size() != 70_000 {
		t.Errorf("a_b.dat: %v", err)
	}
	if st, err := os.Stat(filepath.Join(dest, "Empty")); err != nil || !st.IsDir() {
		t.Error("empty folder not created")
	}
	if last != total || total != 70_003 {
		t.Errorf("progress %d of %d", last, total)
	}
}

// The MTP tab offers a backup when DBI shows its Saves storage, and copies
// it all into the chosen folder.
func TestSavesBackup(t *testing.T) {
	defer i18n.Set("en")
	i18n.Set("en")
	defer func(orig func(func())) { runOnUI = orig }(runOnUI)
	runOnUI = func(fn func()) { uiQueue <- fn }
	fake := newFakeClient()
	game := fake.addTo(savesStorage, mtp.ParentRoot, "Zelda [0100]", true, nil)
	fake.addTo(savesStorage, game, "slot1.dat", false, []byte("save"))
	defer func(orig func(context.Context, *slog.Logger, func(error), <-chan struct{}) (mtpClient, func(), error)) {
		connectMTP = orig
	}(connectMTP)
	connectMTP = func(context.Context, *slog.Logger, func(error), <-chan struct{}) (mtpClient, func(), error) {
		return fake, func() {}, nil
	}
	u := newUI(test.NewApp(), false)
	u.build()
	m := u.mtp
	if m.backupBtn.Visible() {
		t.Error("backup offered before connecting")
	}
	m.connect()
	waitFor(t, "connect", func() bool { return m.client != nil && !m.b.busy })
	if !m.backupBtn.Visible() {
		t.Fatal("no backup button with a Saves storage")
	}
	dest := filepath.Join(t.TempDir(), "DBI saves")
	m.backup(dest)
	waitFor(t, "backup", func() bool { return !m.b.busy })
	if b, err := os.ReadFile(filepath.Join(dest, "Zelda [0100]", "slot1.dat")); err != nil || string(b) != "save" {
		t.Errorf("backup: %q, %v", b, err)
	}
	m.disconnect()
	if m.backupBtn.Visible() {
		t.Error("backup offered after disconnecting")
	}
}

// probeFTP finds hosts greeting like an FTP server and nothing else.
func TestProbeFTP(t *testing.T) {
	addr := fakeDBI(t, map[string]string{"/": ""})
	host, port, _ := strings.Cut(addr, ":")
	var p int
	fmt.Sscan(port, &p)
	if got := probeFTP(context.Background(), []string{host, "127.0.0.2"}, p); len(got) != 1 || got[0] != host {
		t.Errorf("found %v", got)
	}
	if got := probeFTP(context.Background(), []string{host}, 1); len(got) != 0 {
		t.Errorf("found %v on a closed port", got)
	}
}

func TestRememberHost(t *testing.T) {
	got := rememberHost([]string{"a", "b", "c", "d", "e"}, "c")
	if strings.Join(got, ",") != "c,a,b,d,e" {
		t.Errorf("got %v", got)
	}
	got = rememberHost(got, "f")
	if strings.Join(got, ",") != "f,c,a,b,d" {
		t.Errorf("got %v", got)
	}
}

// Search fills in a single Switch, asks among several, explains none.
func TestFTPSearch(t *testing.T) {
	defer i18n.Set("en")
	i18n.Set("en")
	defer func(orig func(func())) { runOnUI = orig }(runOnUI)
	runOnUI = func(fn func()) { uiQueue <- fn }
	defer func(orig func(context.Context) []string) { findSwitches = orig }(findSwitches)
	u := newUI(test.NewApp(), false)
	u.build()
	ft := u.ftp

	findSwitches = func(context.Context) []string { return []string{"192.168.1.20"} }
	ft.search()
	waitFor(t, "search", func() bool { return !ft.searching })
	if ft.host.Text != "192.168.1.20" {
		t.Errorf("host = %q", ft.host.Text)
	}
	for _, found := range [][]string{nil, {"192.168.1.20", "192.168.1.21"}} {
		findSwitches = func(context.Context) []string { return found }
		ft.search()
		waitFor(t, "search", func() bool { return !ft.searching })
		if u.win.Canvas().Overlays().Top() == nil {
			t.Errorf("no dialog for %v", found)
		}
		u.win.Canvas().Overlays().Remove(u.win.Canvas().Overlays().Top())
	}
}
