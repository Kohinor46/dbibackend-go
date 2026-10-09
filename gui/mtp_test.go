package gui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"

	"github.com/Kohinor46/dbibackend-go/i18n"
	"github.com/Kohinor46/dbibackend-go/mtp"
	"github.com/Kohinor46/dbibackend-go/usbconn"
)

// fakeClient is an in-memory MTP device for the tab.
type fakeClient struct {
	mu      sync.Mutex
	objects map[uint32]*fakeEntry
	next    uint32
	failOp  string // operation that returns failErr
	failErr error
}

type fakeEntry struct {
	obj  mtp.Object
	data []byte
}

const (
	sdCard       uint32 = 0x00010001
	savesStorage uint32 = 0x00070001
)

func newFakeClient() *fakeClient {
	f := &fakeClient{objects: map[uint32]*fakeEntry{}, next: 1}
	games := f.add(mtp.ParentRoot, "Games", true, nil)
	f.add(mtp.ParentRoot, "b.nsp", false, []byte("bbb"))
	f.add(games, "a.xci", false, bytes.Repeat([]byte{1}, 5000))
	return f
}

func (f *fakeClient) add(parent uint32, name string, folder bool, data []byte) uint32 {
	return f.addTo(sdCard, parent, name, folder, data)
}

func (f *fakeClient) addTo(storage, parent uint32, name string, folder bool, data []byte) uint32 {
	h := f.next
	f.next++
	o := mtp.Object{Handle: h, Size: uint64(len(data)), Info: mtp.ObjectInfo{StorageID: storage, Filename: name, Parent: parent}}
	if folder {
		o.Info.Format = mtp.FormatAssociation
	}
	f.objects[h] = &fakeEntry{obj: o, data: data}
	return h
}

func (f *fakeClient) setFail(op string, err error) {
	f.mu.Lock()
	f.failOp, f.failErr = op, err
	f.mu.Unlock()
}

func (f *fakeClient) fail(op string) error {
	if f.failOp == op {
		return f.failErr
	}
	return nil
}

func (f *fakeClient) Info() *mtp.DeviceInfo {
	return &mtp.DeviceInfo{Manufacturer: "Nintendo", Model: "Switch (DBI)"}
}

func (f *fakeClient) Storages(context.Context) ([]mtp.Storage, error) {
	return []mtp.Storage{
		{ID: sdCard, Info: mtp.StorageInfo{Description: "SD Card", MaxCapacity: 256 << 30, FreeSpace: 100 << 30}},
		{ID: 0x00020001, Info: mtp.StorageInfo{Description: "SD Card install"}},
		{ID: savesStorage, Info: mtp.StorageInfo{Description: "7: Saves"}},
	}, nil
}

func (f *fakeClient) List(_ context.Context, storage, parent uint32) ([]mtp.Object, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("list"); err != nil {
		return nil, err
	}
	var out []mtp.Object
	for _, e := range f.objects {
		if e.obj.Info.StorageID == storage && e.obj.Info.Parent == parent {
			out = append(out, e.obj)
		}
	}
	return out, nil
}

func (f *fakeClient) Download(_ context.Context, o mtp.Object, w io.Writer) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("download"); err != nil {
		return err
	}
	_, err := w.Write(f.objects[o.Handle].data)
	return err
}

func (f *fakeClient) Upload(_ context.Context, storage, parent uint32, name string, size uint64, r io.Reader) (uint32, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return 0, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.fail("upload"); err != nil {
		return 0, err
	}
	if uint64(len(data)) != size {
		return 0, errors.New("size mismatch")
	}
	h := f.add(parent, name, false, data)
	f.objects[h].obj.Info.StorageID = storage
	return h, nil
}

func (f *fakeClient) MakeFolder(_ context.Context, storage, parent uint32, name string) (uint32, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.add(parent, name, true, nil), nil
}

func (f *fakeClient) Delete(_ context.Context, h uint32) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.objects[h] == nil {
		return &mtp.RespError{Op: mtp.OpDeleteObject, Code: mtp.RespInvalidObjectHndl}
	}
	delete(f.objects, h)
	return nil
}

// uiQueue receives runOnUI calls; waitFor runs them on the test goroutine,
// which plays the UI thread.
var uiQueue = make(chan func(), 10000)

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		for drained := false; !drained; {
			select {
			case f := <-uiQueue:
				f()
			default:
				drained = true
			}
		}
		if cond() {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func names(items []rEntry) []string {
	var n []string
	for _, e := range items {
		n = append(n, e.Name)
	}
	return n
}

func TestMTPTab(t *testing.T) {
	defer i18n.Set("en")
	i18n.Set("en")
	fake := newFakeClient()
	var disconnected atomic.Bool
	defer func(orig func(context.Context, *slog.Logger, func(error), <-chan struct{}) (mtpClient, func(), error)) {
		connectMTP = orig
	}(connectMTP)
	defer func(orig func(func())) { runOnUI = orig }(runOnUI)
	runOnUI = func(f func()) { uiQueue <- f }
	connectMTP = func(context.Context, *slog.Logger, func(error), <-chan struct{}) (mtpClient, func(), error) {
		return fake, func() { disconnected.Store(true) }, nil
	}

	u := newUI(test.NewApp(), false)
	u.build()
	m := u.mtp
	m.connect()
	waitFor(t, "connect and list", func() bool { return m.client != nil && !m.b.busy && len(m.b.items) > 0 })

	if got := m.status.Text; got != "Connected: Nintendo Switch (DBI)" {
		t.Errorf("status = %q", got)
	}
	if got := names(m.b.items); len(got) != 2 || got[0] != "Games" || got[1] != "b.nsp" {
		t.Fatalf("root = %v (folders first)", got)
	}
	if len(m.b.rootSel.Options) != 3 {
		t.Errorf("storages = %v", m.b.rootSel.Options)
	}

	// Open the folder by clicking it.
	m.b.enter(m.b.items[0])
	waitFor(t, "folder listing", func() bool { return !m.b.busy })
	if len(m.b.path) != 1 || names(m.b.items)[0] != "a.xci" || m.b.pathLabel.Text != "/Games" {
		t.Fatalf("in folder: path=%v items=%v label=%q", m.b.path, names(m.b.items), m.b.pathLabel.Text)
	}

	// Upload a local file into it.
	dir := t.TempDir()
	src := filepath.Join(dir, "new.nsp")
	payload := bytes.Repeat([]byte("dbi"), 100_000)
	os.WriteFile(src, payload, 0o644)
	m.b.upload([]string{src, filepath.Join(dir, "missing")})
	waitFor(t, "upload", func() bool { return !m.b.busy })
	if got := names(m.b.items); len(got) != 2 || got[1] != "new.nsp" {
		t.Fatalf("after upload: %v", got)
	}

	// Download it back.
	dest := filepath.Join(dir, "copy.nsp")
	m.b.download(m.b.items[1], dest)
	waitFor(t, "download", func() bool { return !m.b.busy })
	if got, _ := os.ReadFile(dest); !bytes.Equal(got, payload) {
		t.Fatal("downloaded file differs")
	}

	// A failed download leaves no partial file and keeps the session.
	fake.setFail("download", &mtp.RespError{Op: mtp.OpGetObject, Code: mtp.RespGeneralError})
	bad := filepath.Join(dir, "bad.nsp")
	m.b.download(m.b.items[1], bad)
	waitFor(t, "failed download", func() bool { return !m.b.busy })
	if _, err := os.Stat(bad); err == nil || m.client == nil {
		t.Errorf("failed download: file left=%v connected=%v", err == nil, m.client != nil)
	}
	fake.setFail("", nil)

	// New folder, delete, go up.
	m.b.makeFolder("  Saves ")
	waitFor(t, "make folder", func() bool { return !m.b.busy })
	if got := names(m.b.items); got[0] != "Saves" {
		t.Fatalf("after make folder: %v", got)
	}
	m.b.delete(m.b.items[0])
	waitFor(t, "delete", func() bool { return !m.b.busy })
	if len(m.b.items) != 2 {
		t.Fatalf("after delete: %v", names(m.b.items))
	}
	m.b.up()
	waitFor(t, "up", func() bool { return !m.b.busy })
	if len(m.b.path) != 0 || len(m.b.items) != 2 {
		t.Fatalf("after up: %v", names(m.b.items))
	}

	// Switching language keeps the session and the listing.
	i18n.Set("ru")
	u.build()
	if m.client == nil || len(m.b.items) != 2 || m.status.Text != "Подключено: Nintendo Switch (DBI)" {
		t.Errorf("after rebuild: connected=%v items=%v status=%q", m.client != nil, names(m.b.items), m.status.Text)
	}

	// A broken transfer drops the connection.
	fake.setFail("list", mtp.ErrBroken)
	m.b.refresh()
	waitFor(t, "disconnect", func() bool { return m.client == nil })
	waitFor(t, "device released", func() bool { return disconnected.Load() })
}

// On Windows there is no MTP tab: install and FTP only.
func TestNoMTPTabWhenUnsupported(t *testing.T) {
	defer func(v bool) { mtpSupported = v }(mtpSupported)
	mtpSupported = false
	u := newUI(test.NewApp(), false)
	u.build()
	if n := len(u.tabs.Items); n != 2 || len(u.drops) != 2 {
		t.Fatalf("tabs = %d, drops = %d; want install + FTP", n, len(u.drops))
	}
	// Rebuilding (language change) keeps the selected tab and the folder.
	dir := t.TempDir()
	u.setDir(dir)
	u.tabs.SelectIndex(1)
	u.build()
	if u.tabs.SelectedIndex() != 1 || u.dirEntry.Text != dir {
		t.Errorf("after rebuild: tab=%d dir=%q", u.tabs.SelectedIndex(), u.dirEntry.Text)
	}
}

// A busy device shows the release button; pressing it signals the connect
// loop to grab the device.
func TestMTPBusyRelease(t *testing.T) {
	defer i18n.Set("en")
	i18n.Set("en")
	defer func(orig func(func())) { runOnUI = orig }(runOnUI)
	runOnUI = func(f func()) { uiQueue <- f }
	defer func(orig func(context.Context, *slog.Logger, func(error), <-chan struct{}) (mtpClient, func(), error)) {
		connectMTP = orig
	}(connectMTP)
	connectMTP = func(ctx context.Context, _ *slog.Logger, onWait func(error), grab <-chan struct{}) (mtpClient, func(), error) {
		onWait(fmt.Errorf("claim interface: %w", usbconn.ErrBusy))
		select {
		case <-grab:
			return newFakeClient(), func() {}, nil
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
	}

	u := newUI(test.NewApp(), false)
	u.build()
	m := u.mtp
	m.connect()
	waitFor(t, "busy status", func() bool { return m.statusID == "mtp.busy" })
	if canRelease != m.releaseBtn.Visible() {
		t.Errorf("release button visible = %v, want %v", m.releaseBtn.Visible(), canRelease)
	}
	m.release()
	waitFor(t, "connected after release", func() bool { return m.client != nil })
	if m.releaseBtn.Visible() {
		t.Error("release button still visible after connecting")
	}
	m.disconnect()
}

// grabDevice keeps stopping the holders and retrying until the device opens.
func TestGrabDevice(t *testing.T) {
	defer func(orig func() ([]string, error)) { releaseDevice = orig }(releaseDevice)
	stops := 0
	releaseDevice = func() ([]string, error) { stops++; return []string{"ptpcamerad"}, nil }
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	tries := 0
	dev := grabDevice(log, func() (*usbconn.Device, error) {
		if tries++; tries < 5 {
			return nil, fmt.Errorf("claim: %w", usbconn.ErrBusy)
		}
		return &usbconn.Device{}, nil
	})
	if dev == nil || tries != 5 || stops != 4 {
		t.Errorf("dev=%v tries=%d stops=%d", dev != nil, tries, stops)
	}

	// A different error ends the attempt at once.
	tries = 0
	dev = grabDevice(log, func() (*usbconn.Device, error) {
		tries++
		return nil, usbconn.ErrNotFound
	})
	if dev != nil || tries != 1 {
		t.Errorf("not found: dev=%v tries=%d", dev != nil, tries)
	}
}
