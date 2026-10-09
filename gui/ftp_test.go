package gui

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"
	ftpserver "github.com/fclairamb/ftpserverlib"
	"github.com/spf13/afero"

	"github.com/Kohinor46/dbibackend-go/i18n"
)

// testFTPDriver serves an in-memory filesystem; empty user means anonymous.
type testFTPDriver struct {
	fs         afero.Fs
	user, pass string
}

func (d *testFTPDriver) GetSettings() (*ftpserver.Settings, error) {
	return &ftpserver.Settings{ListenAddr: "127.0.0.1:0"}, nil
}
func (d *testFTPDriver) ClientConnected(ftpserver.ClientContext) (string, error) { return "test", nil }
func (d *testFTPDriver) ClientDisconnected(ftpserver.ClientContext)              {}
func (d *testFTPDriver) GetTLSConfig() (*tls.Config, error)                      { return nil, errors.New("no TLS") }
func (d *testFTPDriver) AuthUser(_ ftpserver.ClientContext, user, pass string) (ftpserver.ClientDriver, error) {
	if d.user != "" && (user != d.user || pass != d.pass) {
		return nil, errors.New("bad credentials")
	}
	return d.fs, nil
}

// startFTP runs an FTP server on a free local port until the test ends.
func startFTP(t *testing.T, d *testFTPDriver) string {
	t.Helper()
	if d.fs == nil {
		d.fs = afero.NewMemMapFs()
	}
	srv := ftpserver.NewFtpServer(d)
	if err := srv.Listen(); err != nil {
		t.Fatal(err)
	}
	go srv.Serve()
	t.Cleanup(func() { srv.Stop() })
	return srv.Addr()
}

func readFile(t *testing.T, fs afero.Fs, name string) string {
	t.Helper()
	b, err := afero.ReadFile(fs, name)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return string(b)
}

func TestFTPFS(t *testing.T) {
	d := &testFTPDriver{}
	addr := startFTP(t, d)
	ctx := context.Background()
	f, err := dialFTP(ctx, addr, "", "", quietLog())
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	dir, err := f.MakeDir(ctx, "/", "Игры")
	if err != nil || dir != "/Игры" {
		t.Fatalf("mkdir = %q, %v", dir, err)
	}
	payload := bytes.Repeat([]byte("dbi"), 200_000)
	if _, err := f.Upload(ctx, dir, "game.nsp", int64(len(payload)), bytes.NewReader(payload)); err != nil {
		t.Fatal(err)
	}
	list, err := f.List(ctx, dir)
	if err != nil || len(list) != 1 || list[0].Name != "game.nsp" || list[0].Size != int64(len(payload)) || list[0].ID != "/Игры/game.nsp" {
		t.Fatalf("list = %+v, %v", list, err)
	}
	var got bytes.Buffer
	if err := f.Download(ctx, list[0], &got); err != nil || !bytes.Equal(got.Bytes(), payload) {
		t.Fatalf("download: %v (%d bytes)", err, got.Len())
	}

	// A refusal from the server keeps the connection usable.
	err = f.Delete(ctx, rEntry{ID: "/missing.nsp"})
	if err == nil || errors.Is(err, errFTPLost) {
		t.Fatalf("delete missing = %v, want a server refusal", err)
	}
	// Folders are removed with their contents.
	if err := f.Delete(ctx, rEntry{ID: dir, Dir: true}); err != nil {
		t.Fatal(err)
	}
	if root, _ := f.List(ctx, "/"); len(root) != 0 {
		t.Errorf("root after delete = %+v", root)
	}

	// Cancelling an operation closes the connection.
	cctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := f.List(cctx, "/"); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled list = %v", err)
	}

	// A dead connection (Wi-Fi gone) is reported as lost.
	g, err := dialFTP(ctx, addr, "", "", quietLog())
	if err != nil {
		t.Fatal(err)
	}
	g.ctrl.Close()
	if _, err := g.List(ctx, "/"); !errors.Is(err, errFTPLost) {
		t.Errorf("after connection loss = %v, want errFTPLost", err)
	}
}

// The merge rules work the same over FTP: existing folders are reused
// (case-insensitively), same-name files replaced, everything else kept.
func TestFTPUploadMergesFolders(t *testing.T) {
	d := &testFTPDriver{fs: afero.NewMemMapFs()}
	afero.WriteFile(d.fs, "/mods/a.txt", []byte("old a"), 0o644)
	afero.WriteFile(d.fs, "/mods/keep.txt", []byte("keep"), 0o644)
	addr := startFTP(t, d)
	ctx := context.Background()
	f, err := dialFTP(ctx, addr, "", "", quietLog())
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	src := t.TempDir()
	writeTree(t, src, map[string]string{"Mods/a.txt": "new a", "Mods/sub/b.txt": "b", "Mods/.DS_Store": "junk"})
	items, _ := collectLocal([]string{filepath.Join(src, "Mods")})
	u := newUploader(ctx, f, "/", false, quietLog())
	if err := u.run(items); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, d.fs, "/mods/a.txt"); got != "new a" {
		t.Errorf("a.txt = %q", got)
	}
	if got := readFile(t, d.fs, "/mods/keep.txt"); got != "keep" {
		t.Errorf("keep.txt = %q", got)
	}
	if got := readFile(t, d.fs, "/mods/sub/b.txt"); got != "b" {
		t.Errorf("sub/b.txt = %q", got)
	}
	if ok, _ := afero.Exists(d.fs, "/Mods"); ok {
		t.Error("a second folder /Mods was created instead of merging into /mods")
	}
	if ok, _ := afero.Exists(d.fs, "/mods/.DS_Store"); ok {
		t.Error(".DS_Store was uploaded")
	}
	if u.stats != (uploadStats{uploaded: 2, replaced: 1}) {
		t.Errorf("stats = %+v", u.stats)
	}
}

// The tab connects with a login, browses, uploads, and reports a bad password.
func TestFTPTab(t *testing.T) {
	defer i18n.Set("en")
	i18n.Set("en")
	defer func(orig func(func())) { runOnUI = orig }(runOnUI)
	runOnUI = func(fn func()) { uiQueue <- fn }

	d := &testFTPDriver{fs: afero.NewMemMapFs(), user: "switch", pass: "pwd"}
	afero.WriteFile(d.fs, "/Games/old.nsp", []byte("old"), 0o644)
	addr := startFTP(t, d)

	u := newUI(test.NewApp(), false)
	u.build()
	ft := u.ftp
	ft.host.SetText(addr) // host:port overrides the port of the mode
	ft.user.SetText("switch")
	ft.pass.SetText("wrong")
	ft.connect()
	waitFor(t, "failed login", func() bool { return ft.connecting == nil })
	if ft.fs != nil || ft.statusID != "mtp.disconnected" {
		t.Fatalf("bad password: connected=%v status=%s", ft.fs != nil, ft.statusID)
	}

	ft.pass.SetText("pwd")
	ft.connect()
	waitFor(t, "connect and list", func() bool { return ft.fs != nil && !ft.b.busy && len(ft.b.items) == 1 })
	if ft.status.Text != "Connected: "+addr || ft.b.rootRow.Visible() {
		t.Errorf("status=%q rootRow visible=%v", ft.status.Text, ft.b.rootRow.Visible())
	}
	if !ft.host.Disabled() {
		t.Error("address editable while connected")
	}

	ft.b.enter(ft.b.items[0]) // open Games
	waitFor(t, "open folder", func() bool { return !ft.b.busy && len(ft.b.path) == 1 })
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "new.nsp"), []byte("new"), 0o644)
	ft.b.upload([]string{filepath.Join(src, "new.nsp")})
	waitFor(t, "upload", func() bool { return !ft.b.busy })
	if got := strings.Join(names(ft.b.items), ","); got != "new.nsp,old.nsp" {
		t.Errorf("Games = %s", got)
	}
	if got := readFile(t, d.fs, "/Games/new.nsp"); got != "new" {
		t.Errorf("server file = %q", got)
	}

	ft.disconnect()
	if ft.fs != nil || ft.b.fs != nil || ft.host.Disabled() {
		t.Error("not fully disconnected")
	}
	if u.app.Preferences().String(prefFTPHost) != addr || u.app.Preferences().String(prefFTPUser) != "switch" {
		t.Error("host/user not remembered")
	}
}

// The mode survives a language change (the radio labels change).
func TestFTPModeKeptOnRebuild(t *testing.T) {
	defer i18n.Set("en")
	i18n.Set("en")
	u := newUI(test.NewApp(), false)
	u.build()
	u.ftp.setInstallMode(true)
	i18n.Set("ru")
	u.build()
	if !u.ftp.installMode() {
		t.Error("install mode lost after rebuild")
	}
}

func TestFTPAddress(t *testing.T) {
	u := newUI(test.NewApp(), false)
	u.build()
	ft := u.ftp
	for _, tc := range []struct {
		host    string
		install bool
		want    string
	}{
		{"192.168.1.20", false, "192.168.1.20:5000"},
		{" 192.168.1.20 ", true, "192.168.1.20:6000"},
		{"192.168.1.20:2121", true, "192.168.1.20:2121"},
		{"fe80::1", false, "[fe80::1]:5000"},
		{"[fe80::1]", false, "[fe80::1]:5000"},
	} {
		ft.host.SetText(tc.host)
		ft.setInstallMode(tc.install)
		if got := ft.address(); got != tc.want {
			t.Errorf("address(%q, install=%v) = %q, want %q", tc.host, tc.install, got, tc.want)
		}
	}
}
