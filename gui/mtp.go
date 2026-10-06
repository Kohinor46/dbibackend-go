package gui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/ncruces/zenity"

	"github.com/Kohinor46/dbibackend-go/dbi"
	"github.com/Kohinor46/dbibackend-go/i18n"
	"github.com/Kohinor46/dbibackend-go/mtp"
	"github.com/Kohinor46/dbibackend-go/usbconn"
)

// mtpClient is what the tab needs from the device: *mtp.Client over USB in
// the app, an in-memory fake in tests.
type mtpClient interface {
	Info() *mtp.DeviceInfo
	Storages(ctx context.Context) ([]mtp.Storage, error)
	List(ctx context.Context, storage, parent uint32) ([]mtp.Object, error)
	Download(ctx context.Context, o mtp.Object, w io.Writer) error
	Upload(ctx context.Context, storage, parent uint32, name string, size uint64, r io.Reader) (uint32, error)
	MakeFolder(ctx context.Context, storage, parent uint32, name string) (uint32, error)
	Delete(ctx context.Context, handle uint32) error
}

// connectMTP waits for the Switch in DBI's "MTP responder" mode and opens
// an MTP session; disconnect ends it and releases the device. A signal on
// grab ("Free the device") makes it take the device from the programs that
// hold it (see grabDevice).
var connectMTP = func(ctx context.Context, log *slog.Logger, onWait func(error), grab <-chan struct{}) (client mtpClient, disconnect func(), err error) {
	open := func() (*usbconn.Device, error) { return usbconn.OpenMTP(dbi.SwitchVID, dbi.SwitchPID) }
	var dev *usbconn.Device
	for dev == nil {
		if dev, err = open(); err == nil {
			break
		}
		onWait(err)
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(time.Second):
		case <-grab:
			dev = grabDevice(log, open) // on failure the next round reports the error
		}
	}
	c := mtp.NewClient(dev)
	if err := c.Open(ctx); err != nil {
		dev.Close()
		return nil, nil, err
	}
	return c, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		c.Close(ctx)
		dev.Close()
	}, nil
}

// grabDevice takes a busy device from the programs holding it. Stopping
// macOS's camera service once is not enough: the system starts it again
// at once and it grabs the device back. So for a few seconds the service is
// stopped every time it reappears while open is retried in between;
// launchd delays each restart longer (exponential throttling), so a window
// opens, and once the interface is claimed the restarted service can't
// take it.
func grabDevice(log *slog.Logger, open func() (*usbconn.Device, error)) *usbconn.Device {
	log.Info("Freeing the device", "owner", deviceOwner())
	start := time.Now()
	attempts, stopped := 0, 0
	var lastErr error
	for time.Since(start) < 3*time.Second {
		attempts++
		dev, err := open()
		if err == nil {
			log.Info("Device freed", "attempts", attempts, "stopped", stopped, "took", time.Since(start).Round(time.Millisecond))
			return dev
		}
		if lastErr = err; !errors.Is(err, usbconn.ErrBusy) {
			break
		}
		names, err := releaseDevice()
		if err != nil {
			log.Warn("Could not stop a program holding the device", "err", err)
		}
		stopped += len(names)
		time.Sleep(2 * time.Millisecond)
	}
	log.Warn("Could not free the device", "attempts", attempts, "stopped", stopped,
		"owner", deviceOwner(), "running", strings.Join(runningHolders(), ", "), "err", lastErr)
	return nil
}

// mtpSupported: the "Files (MTP)" tab is for macOS and Linux, which have no
// built-in MTP support. On Windows the system MTP driver owns the device and
// Explorer already shows the Switch, so the tab is hidden there.
var mtpSupported = runtime.GOOS != "windows"

// runOnUI runs f on the UI goroutine (fyne.Do); tests replace it to run
// UI updates on the test goroutine, which plays the UI thread there.
var runOnUI = fyne.Do

type folder struct {
	handle uint32
	name   string
}

// mtpTab is the "Files (MTP)" tab. All fields are used on the UI goroutine;
// device operations run in the background one at a time (see run).
type mtpTab struct {
	u *ui

	client   mtpClient
	closeDev func()
	grab     chan struct{} // tells the waiting connect loop to retry right away
	storages []mtp.Storage
	storage  int
	path     []folder // folders below the storage root
	items    []mtp.Object
	busy     bool
	cancel   context.CancelFunc // cancels the running operation or the wait
	statusID string
	statusAr []any
	statusIm widget.Importance

	// Widgets, rebuilt by build().
	connectBtn  *widget.Button
	releaseBtn  *widget.Button
	status      *widget.Label
	storageSel  *widget.Select
	pathLabel   *widget.Label
	list        *widget.List
	emptyLabel  *widget.Label
	toolbar     []*widget.Button
	progress    *widget.ProgressBar
	progressLbl *widget.Label
	progressBox *fyne.Container
	progressTxt string
	root        *fyne.Container // the tab content, re-laid out when progressBox shows/hides
}

func newMTPTab(u *ui) *mtpTab {
	return &mtpTab{u: u, statusID: "mtp.disconnected"}
}

func (m *mtpTab) build() fyne.CanvasObject {
	T := i18n.T
	m.connectBtn = widget.NewButtonWithIcon("", theme.LoginIcon(), m.toggleConnect)
	m.releaseBtn = widget.NewButtonWithIcon(T("mtp.release"), theme.MediaStopIcon(), m.release)
	m.releaseBtn.Hide()
	m.status = widget.NewLabel("")
	m.status.TextStyle.Bold = true
	m.status.Truncation = fyne.TextTruncateEllipsis

	m.storageSel = widget.NewSelect(nil, func(string) {
		if i := m.storageSel.SelectedIndex(); i >= 0 && i != m.storage {
			m.storage, m.path = i, nil
			m.refresh()
		}
	})
	m.pathLabel = widget.NewLabel("")
	m.pathLabel.Truncation = fyne.TextTruncateEllipsis

	up := widget.NewButtonWithIcon("", theme.NavigateBackIcon(), m.up)
	refresh := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), m.refresh)
	upload := widget.NewButtonWithIcon(T("mtp.upload"), theme.UploadIcon(), m.pickUpload)
	newFolder := widget.NewButtonWithIcon(T("mtp.new_folder"), theme.FolderNewIcon(), m.askNewFolder)
	m.toolbar = []*widget.Button{up, refresh, upload, newFolder}

	m.list = widget.NewList(
		func() int { return len(m.items) },
		func() fyne.CanvasObject {
			name := widget.NewLabel("")
			name.Truncation = fyne.TextTruncateEllipsis
			size := widget.NewLabel(humanBytes(1023 << 30))
			size.Alignment = fyne.TextAlignTrailing
			dl := widget.NewButtonWithIcon("", theme.DownloadIcon(), nil)
			del := widget.NewButtonWithIcon("", theme.DeleteIcon(), nil)
			return container.NewBorder(nil, nil, widget.NewIcon(theme.FileIcon()), container.NewHBox(size, dl, del), name)
		},
		func(id widget.ListItemID, o fyne.CanvasObject) {
			if id >= len(m.items) {
				return
			}
			it := m.items[id]
			c := o.(*fyne.Container)
			name, icon := c.Objects[0].(*widget.Label), c.Objects[1].(*widget.Icon)
			right := c.Objects[2].(*fyne.Container)
			size, dl, del := right.Objects[0].(*widget.Label), right.Objects[1].(*widget.Button), right.Objects[2].(*widget.Button)
			name.SetText(it.Name())
			del.OnTapped = func() { m.askDelete(it) }
			if it.IsFolder() {
				icon.SetResource(theme.FolderIcon())
				size.SetText("")
				dl.Hide()
			} else {
				icon.SetResource(theme.FileIcon())
				size.SetText(humanBytes(int64(it.Size)))
				dl.OnTapped = func() { m.pickDownload(it) }
				dl.Show()
			}
			if m.busy {
				dl.Disable()
				del.Disable()
			} else {
				dl.Enable()
				del.Enable()
			}
		},
	)
	m.list.OnSelected = func(id widget.ListItemID) {
		m.list.UnselectAll()
		if id < len(m.items) && m.items[id].IsFolder() && !m.busy {
			m.path = append(m.path, folder{m.items[id].Handle, m.items[id].Name()})
			m.refresh()
		}
	}
	m.emptyLabel = widget.NewLabel(T("mtp.empty"))
	m.emptyLabel.Alignment = fyne.TextAlignCenter

	m.progress = widget.NewProgressBar()
	m.progress.TextFormatter = func() string { return m.progressTxt }
	m.progressLbl = widget.NewLabel("")
	m.progressLbl.Truncation = fyne.TextTruncateEllipsis
	cancel := widget.NewButtonWithIcon(T("cancel"), theme.CancelIcon(), func() {
		if m.cancel != nil {
			m.cancel()
		}
	})
	m.progressBox = container.NewVBox(widget.NewSeparator(), container.NewBorder(nil, nil, nil, cancel, m.progressLbl), m.progress)
	m.progressBox.Hide()

	top := container.NewVBox(
		container.NewBorder(nil, nil, widget.NewLabel(T("status.label")), container.NewHBox(m.releaseBtn, m.connectBtn), m.status),
		container.NewBorder(nil, nil, widget.NewLabel(T("mtp.storage")), nil, m.storageSel),
		container.NewBorder(nil, nil, container.NewHBox(up, refresh), container.NewHBox(newFolder, upload), m.pathLabel),
		widget.NewSeparator(),
	)
	m.root = container.NewBorder(top, m.progressBox, nil, nil, container.NewStack(m.list, container.NewCenter(m.emptyLabel)))
	m.render()
	return m.root
}

// showProgress shows or hides the progress area; the Border layout only
// makes room for it after a refresh.
func (m *mtpTab) showProgress(show bool) {
	if show {
		m.progressBox.Show()
	} else {
		m.progressBox.Hide()
	}
	m.root.Refresh()
}

// render shows the current state in the widgets.
func (m *mtpTab) render() {
	T := i18n.T
	connected := m.client != nil
	switch {
	case connected:
		m.connectBtn.SetText(T("mtp.disconnect"))
		m.connectBtn.SetIcon(theme.LogoutIcon())
		m.connectBtn.Importance = widget.MediumImportance
	case m.cancel != nil: // waiting for the device
		m.connectBtn.SetText(T("cancel"))
		m.connectBtn.SetIcon(theme.CancelIcon())
		m.connectBtn.Importance = widget.MediumImportance
	default:
		m.connectBtn.SetText(T("mtp.connect"))
		m.connectBtn.SetIcon(theme.LoginIcon())
		m.connectBtn.Importance = widget.HighImportance
	}
	m.connectBtn.Refresh()
	m.status.SetText(T(m.statusID, m.statusAr...))
	m.status.Importance = m.statusIm
	m.status.Refresh()

	options := make([]string, len(m.storages))
	for i, s := range m.storages {
		options[i] = storageName(s)
	}
	m.storageSel.SetOptions(options)
	if m.storage < len(options) {
		m.storageSel.SetSelectedIndex(m.storage)
	} else {
		m.storageSel.ClearSelected()
	}

	names := make([]string, len(m.path))
	for i, f := range m.path {
		names[i] = f.name
	}
	m.pathLabel.SetText("/" + strings.Join(names, "/"))

	for i, b := range m.toolbar {
		if !connected || m.busy || (i == 0 && len(m.path) == 0) {
			b.Disable()
		} else {
			b.Enable()
		}
	}
	if connected && !m.busy {
		m.storageSel.Enable()
	} else {
		m.storageSel.Disable()
	}
	if connected && len(m.items) == 0 && !m.busy {
		m.emptyLabel.Show()
	} else {
		m.emptyLabel.Hide()
	}
	m.list.Refresh()
}

func storageName(s mtp.Storage) string {
	name := s.Info.Description
	if name == "" {
		name = s.Info.VolumeLabel
	}
	if name == "" {
		name = fmt.Sprintf("0x%08X", s.ID)
	}
	if s.Info.MaxCapacity > 0 {
		name += "  (" + i18n.T("mtp.free", humanBytes(int64(s.Info.FreeSpace)), humanBytes(int64(s.Info.MaxCapacity))) + ")"
	}
	return name
}

func (m *mtpTab) setStatus(id string, imp widget.Importance, args ...any) {
	m.statusID, m.statusIm, m.statusAr = id, imp, args
	m.render()
}

func (m *mtpTab) toggleConnect() {
	switch {
	case m.client != nil:
		m.disconnect()
	case m.cancel != nil:
		m.cancel()
	default:
		m.connect()
	}
}

func (m *mtpTab) connect() {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	grab := make(chan struct{}, 1)
	m.grab = grab
	m.setStatus("mtp.waiting", widget.WarningImportance)
	go func() {
		wasBusy := false // the same error repeats every second: report changes only
		client, closeDev, err := connectMTP(ctx, m.u.log, func(err error) {
			busy := errors.Is(err, usbconn.ErrBusy)
			switch {
			case busy && !wasBusy:
				m.u.log.Warn("MTP device is busy", "err", err, "running", strings.Join(runningHolders(), ", "))
				runOnUI(func() {
					m.setStatus("mtp.busy", widget.DangerImportance)
					if canRelease {
						m.releaseBtn.Show()
					}
				})
			case !busy && wasBusy:
				runOnUI(func() {
					m.releaseBtn.Hide()
					m.setStatus("mtp.waiting", widget.WarningImportance)
				})
			case !busy && !errors.Is(err, usbconn.ErrNotFound):
				m.u.log.Warn("Cannot open MTP device", "err", err)
			}
			wasBusy = busy
		}, grab)
		var storages []mtp.Storage
		if err == nil {
			if storages, err = client.Storages(ctx); err != nil {
				closeDev()
			}
		}
		runOnUI(func() {
			m.cancel = nil
			m.releaseBtn.Hide()
			if err != nil {
				if !errors.Is(err, context.Canceled) {
					m.u.log.Error("MTP connect failed", "err", err)
					dialog.ShowError(errors.New(i18n.T("mtp.failed", err)), m.u.win)
				}
				m.setStatus("mtp.disconnected", widget.MediumImportance)
				return
			}
			info := client.Info()
			m.u.log.Info("MTP connected", "model", info.Model, "storages", len(storages),
				"propList", info.Supports(mtp.OpSendObjectPropList))
			m.client, m.closeDev, m.storages, m.storage, m.path, m.items = client, closeDev, storages, 0, nil, nil
			m.setStatus("mtp.connected", widget.SuccessImportance, strings.TrimSpace(info.Manufacturer+" "+info.Model))
			m.refresh()
		})
	}()
}

func (m *mtpTab) disconnect() {
	if m.cancel != nil {
		m.cancel()
	}
	if m.closeDev != nil {
		go m.closeDev()
	}
	m.client, m.closeDev, m.storages, m.path, m.items, m.busy = nil, nil, nil, nil, nil, false
	m.showProgress(false)
	m.setStatus("mtp.disconnected", widget.MediumImportance)
}

// release makes the waiting connect loop take the device from the programs
// holding it (see grabDevice).
func (m *mtpTab) release() {
	m.releaseBtn.Hide()
	select {
	case m.grab <- struct{}{}:
	default:
	}
}

func (m *mtpTab) current() (storage, parent uint32) {
	storage, parent = m.storages[m.storage].ID, mtp.ParentRoot
	if len(m.path) > 0 {
		parent = m.path[len(m.path)-1].handle
	}
	return storage, parent
}

func (m *mtpTab) up() {
	if len(m.path) > 0 {
		m.path = m.path[:len(m.path)-1]
		m.refresh()
	}
}

func (m *mtpTab) refresh() {
	if m.client == nil || m.storage >= len(m.storages) {
		m.items = nil
		m.render()
		return
	}
	storage, parent := m.current()
	var items []mtp.Object
	m.run("", 0, func(ctx context.Context, c mtpClient, _ func(int64)) error {
		var err error
		items, err = c.List(ctx, storage, parent)
		sort.Slice(items, func(i, j int) bool {
			if items[i].IsFolder() != items[j].IsFolder() {
				return items[i].IsFolder()
			}
			return strings.ToLower(items[i].Name()) < strings.ToLower(items[j].Name())
		})
		return err
	}, func() { m.items = items })
}

// run executes op in the background with the tab locked. label and total
// enable the progress bar (op reports bytes done through its callback);
// then runs on the UI goroutine after a successful op. op must use the
// client it is given, never m's fields, which belong to the UI goroutine.
func (m *mtpTab) run(label string, total int64, op func(ctx context.Context, c mtpClient, progress func(int64)) error, then func()) {
	if m.busy || m.client == nil {
		return
	}
	// The op gets the client as of now: Disconnect may clear m.client meanwhile.
	client := m.client
	ctx, cancel := context.WithCancel(context.Background())
	m.busy, m.cancel = true, cancel
	if label != "" {
		m.progressLbl.SetText(label)
		m.progressTxt = ""
		m.progress.SetValue(0)
		m.showProgress(true)
	}
	m.render()

	start := time.Now()
	var last time.Time
	progress := func(done int64) {
		if now := time.Now(); now.Sub(last) >= 200*time.Millisecond || done == total {
			last = now
			speed := float64(done) / now.Sub(start).Seconds()
			runOnUI(func() {
				if total > 0 {
					m.progressTxt = i18n.T("transfer.progress", float64(done)*100/float64(total), humanBytes(done), humanBytes(total), humanBytes(int64(speed)))
					m.progress.SetValue(float64(done) / float64(total))
				}
			})
		}
	}
	go func() {
		err := op(ctx, client, progress)
		runOnUI(func() {
			m.busy, m.cancel = false, nil
			m.showProgress(false)
			switch {
			case err == nil:
				if then != nil {
					then()
				}
				m.render()
			case errors.Is(err, mtp.ErrBroken) || errors.Is(err, context.Canceled):
				// The transfer was cut midway: the session can't continue.
				m.u.log.Warn("MTP operation interrupted", "err", err)
				m.disconnect()
				if !errors.Is(err, context.Canceled) {
					dialog.ShowError(errors.New(i18n.T("mtp.lost")), m.u.win)
				}
			default:
				m.u.log.Error("MTP operation failed", "err", err)
				m.render()
				dialog.ShowError(errors.New(i18n.T("mtp.failed", err)), m.u.win)
			}
		})
	}()
}

func (m *mtpTab) pickUpload() {
	go func() {
		files, err := zenity.SelectFileMultiple(zenity.Title(i18n.T("mtp.upload")))
		if err != nil {
			if !errors.Is(err, zenity.ErrCanceled) {
				m.u.log.Warn("File picker failed", "err", err)
			}
			return
		}
		runOnUI(func() { m.upload(files) })
	}()
}

// upload sends local files into the current folder, one after another.
func (m *mtpTab) upload(paths []string) {
	if m.client == nil || m.busy {
		return
	}
	type file struct {
		path string
		size int64
	}
	var files []file
	var total int64
	for _, p := range paths {
		if st, err := os.Stat(p); err == nil && st.Mode().IsRegular() {
			files = append(files, file{p, st.Size()})
			total += st.Size()
		}
	}
	if len(files) == 0 {
		return
	}
	storage, parent := m.current()
	label := i18n.T("mtp.uploading", filepath.Base(files[0].path))
	if len(files) > 1 {
		label = i18n.T("mtp.uploading", fmt.Sprintf("%d × …", len(files)))
	}
	m.run(label, total, func(ctx context.Context, c mtpClient, progress func(int64)) error {
		var done int64
		for _, f := range files {
			r, err := os.Open(f.path)
			if err != nil {
				return err
			}
			cr := &countingReader{r: r, done: done, report: progress}
			_, err = c.Upload(ctx, storage, parent, filepath.Base(f.path), uint64(f.size), cr)
			r.Close()
			if err != nil {
				return err
			}
			done += f.size
			m.u.log.Info("Uploaded", "file", filepath.Base(f.path), "size", f.size)
		}
		return nil
	}, m.refresh)
}

func (m *mtpTab) pickDownload(o mtp.Object) {
	go func() {
		dest, err := zenity.SelectFileSave(zenity.Title(i18n.T("mtp.download")), zenity.Filename(o.Name()), zenity.ConfirmOverwrite())
		if err != nil {
			if !errors.Is(err, zenity.ErrCanceled) {
				m.u.log.Warn("File picker failed", "err", err)
			}
			return
		}
		runOnUI(func() { m.download(o, dest) })
	}()
}

func (m *mtpTab) download(o mtp.Object, dest string) {
	m.run(i18n.T("mtp.downloading", o.Name()), int64(o.Size), func(ctx context.Context, c mtpClient, progress func(int64)) error {
		f, err := os.Create(dest)
		if err != nil {
			return err
		}
		err = c.Download(ctx, o, &countingWriter{w: f, report: progress})
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			os.Remove(dest) // don't leave a truncated file behind
			return err
		}
		m.u.log.Info("Downloaded", "file", o.Name(), "to", dest)
		return nil
	}, nil)
}

func (m *mtpTab) askNewFolder() {
	entry := widget.NewEntry()
	d := dialog.NewCustomConfirm(i18n.T("mtp.new_folder"), i18n.T("mtp.create"), i18n.T("cancel"),
		container.NewBorder(nil, nil, widget.NewLabel(i18n.T("mtp.folder_name")), nil, entry),
		func(ok bool) {
			if ok {
				m.makeFolder(entry.Text)
			}
		}, m.u.win)
	d.Resize(fyne.NewSize(420, 0))
	d.Show()
	m.u.win.Canvas().Focus(entry)
}

func (m *mtpTab) askDelete(o mtp.Object) {
	dialog.NewCustomConfirm(i18n.T("mtp.delete"), i18n.T("mtp.delete"), i18n.T("cancel"),
		widget.NewLabel(i18n.T("mtp.delete_confirm", o.Name())),
		func(ok bool) {
			if ok {
				m.delete(o)
			}
		}, m.u.win).Show()
}

func (m *mtpTab) makeFolder(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	storage, parent := m.current()
	m.run("", 0, func(ctx context.Context, c mtpClient, _ func(int64)) error {
		_, err := c.MakeFolder(ctx, storage, parent, name)
		return err
	}, m.refresh)
}

func (m *mtpTab) delete(o mtp.Object) {
	m.run("", 0, func(ctx context.Context, c mtpClient, _ func(int64)) error {
		return c.Delete(ctx, o.Handle)
	}, m.refresh)
}

// dropped handles files dropped on the window while this tab is shown.
func (m *mtpTab) dropped(uris []fyne.URI) {
	paths := make([]string, 0, len(uris))
	for _, u := range uris {
		paths = append(paths, u.Path())
	}
	m.upload(paths)
}

type countingReader struct {
	r      io.Reader
	done   int64
	report func(int64)
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.done += int64(n)
	c.report(c.done)
	return n, err
}

type countingWriter struct {
	w      io.Writer
	done   int64
	report func(int64)
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.done += int64(n)
	c.report(c.done)
	return n, err
}
