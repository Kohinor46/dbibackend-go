package gui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

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

// mtpTab is the "Files (MTP)" tab: the USB connection to DBI's MTP
// responder, around a file browser. All fields are used on the UI goroutine.
type mtpTab struct {
	u *ui
	b *browser

	client   mtpClient
	closeDev func()
	grab     chan struct{}      // tells the waiting connect loop to retry right away
	waiting  context.CancelFunc // cancels the wait for the device
	statusID string
	statusAr []any
	statusIm widget.Importance

	// Widgets, rebuilt by build().
	connectBtn *widget.Button
	releaseBtn *widget.Button
	status     *widget.Label
}

func newMTPTab(u *ui) *mtpTab {
	m := &mtpTab{u: u, statusID: "mtp.disconnected"}
	m.b = newBrowser(u, "tab.mtp", func(err error) bool { return errors.Is(err, mtp.ErrBroken) }, m.disconnect)
	return m
}

func (m *mtpTab) build() fyne.CanvasObject {
	T := i18n.T
	m.connectBtn = widget.NewButtonWithIcon("", theme.LoginIcon(), m.toggleConnect)
	m.releaseBtn = widget.NewButtonWithIcon(T("mtp.release"), theme.MediaStopIcon(), m.release)
	m.releaseBtn.Hide()
	m.status = widget.NewLabel("")
	m.status.TextStyle.Bold = true
	m.status.Truncation = fyne.TextTruncateEllipsis
	content := m.b.build(container.NewBorder(nil, nil, widget.NewLabel(T("status.label")), container.NewHBox(m.releaseBtn, m.connectBtn), m.status))
	m.render()
	return content
}

// render shows the connection state; the browser renders itself.
func (m *mtpTab) render() {
	T := i18n.T
	switch {
	case m.client != nil:
		m.connectBtn.SetText(T("mtp.disconnect"))
		m.connectBtn.SetIcon(theme.LogoutIcon())
		m.connectBtn.Importance = widget.MediumImportance
	case m.waiting != nil:
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
	case m.waiting != nil:
		m.waiting()
	default:
		m.connect()
	}
}

func (m *mtpTab) connect() {
	ctx, cancel := context.WithCancel(context.Background())
	m.waiting = cancel
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
			m.waiting = nil
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
			m.client, m.closeDev = client, closeDev
			m.setStatus("mtp.connected", widget.SuccessImportance, strings.TrimSpace(info.Manufacturer+" "+info.Model))
			m.b.open(mtpFS{client}, mtpRoots(storages))
		})
	}()
}

func (m *mtpTab) disconnect() {
	if m.waiting != nil {
		m.waiting()
		m.waiting = nil
	}
	if m.closeDev != nil {
		go m.closeDev()
	}
	m.client, m.closeDev = nil, nil
	m.b.close()
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
