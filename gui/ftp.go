package gui

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Kohinor46/dbibackend-go/i18n"
)

// DBI's FTP server: SD card files on port 5000, installing on port 6000.
const (
	ftpPortSD      = 5000
	ftpPortInstall = 6000

	prefFTPHost    = "ftpHost"
	prefFTPInstall = "ftpInstall"
	prefFTPUser    = "ftpUser"
)

// openFTP connects to an FTP server; tests replace it.
var openFTP = func(ctx context.Context, addr, user, pass string, log *slog.Logger) (remoteFS, func(), error) {
	f, err := dialFTP(ctx, addr, user, pass, log)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
}

// ftpTab is the "FTP" tab: a connection to DBI's FTP server over the
// network, around a file browser. All fields are used on the UI goroutine.
type ftpTab struct {
	u *ui
	b *browser

	fs         remoteFS
	closeFS    func()
	install    bool // install port (6000) instead of SD card (5000)
	connecting context.CancelFunc
	statusID   string
	statusAr   []any
	statusIm   widget.Importance

	// Widgets, rebuilt by build().
	host       *widget.Entry
	mode       *widget.RadioGroup
	user       *widget.Entry
	pass       *widget.Entry
	connectBtn *widget.Button
	status     *widget.Label
}

func newFTPTab(u *ui) *ftpTab {
	t := &ftpTab{u: u, statusID: "mtp.disconnected", install: u.app.Preferences().Bool(prefFTPInstall)}
	t.b = newBrowser(u, "tab.ftp", func(err error) bool { return errors.Is(err, errFTPLost) }, t.disconnect)
	return t
}

func (t *ftpTab) build() fyne.CanvasObject {
	T := i18n.T
	prefs := t.u.app.Preferences()
	host := prefs.String(prefFTPHost)
	if t.host != nil {
		host = t.host.Text // keep what was typed across a language change
	}
	t.host = widget.NewEntry()
	t.host.SetPlaceHolder(T("ftp.host_placeholder"))
	t.host.SetText(host)
	t.host.OnSubmitted = func(string) { t.toggleConnect() }

	t.mode = widget.NewRadioGroup([]string{T("ftp.mode_sd"), T("ftp.mode_install")}, nil)
	t.mode.Horizontal = true
	t.mode.Required = true
	t.setInstallMode(t.install)
	t.mode.OnChanged = func(label string) {
		install := label == T("ftp.mode_install")
		if install == t.install {
			return
		}
		t.install = install
		prefs.SetBool(prefFTPInstall, install)
		if t.fs != nil { // reconnect to the other port
			t.disconnect()
			t.connect()
		}
	}

	t.user = widget.NewEntry()
	t.user.SetPlaceHolder("anonymous")
	t.user.SetText(prefs.String(prefFTPUser))
	t.pass = widget.NewPasswordEntry()
	login := widget.NewAccordion(widget.NewAccordionItem(T("ftp.login"), widget.NewForm(
		widget.NewFormItem(T("ftp.user"), t.user),
		widget.NewFormItem(T("ftp.password"), t.pass),
	)))

	t.connectBtn = widget.NewButtonWithIcon("", theme.LoginIcon(), t.toggleConnect)
	t.status = widget.NewLabel("")
	t.status.TextStyle.Bold = true
	t.status.Truncation = fyne.TextTruncateEllipsis
	hint := widget.NewLabel(T("ftp.hint"))
	hint.Wrapping = fyne.TextWrapWord

	content := t.b.build(
		container.NewBorder(nil, nil, widget.NewLabel(T("ftp.host")), t.connectBtn, t.host),
		t.mode,
		hint,
		login,
		container.NewBorder(nil, nil, widget.NewLabel(T("status.label")), nil, t.status),
	)
	t.render()
	return content
}

func (t *ftpTab) render() {
	T := i18n.T
	switch {
	case t.fs != nil:
		t.connectBtn.SetText(T("mtp.disconnect"))
		t.connectBtn.SetIcon(theme.LogoutIcon())
		t.connectBtn.Importance = widget.MediumImportance
	case t.connecting != nil:
		t.connectBtn.SetText(T("cancel"))
		t.connectBtn.SetIcon(theme.CancelIcon())
		t.connectBtn.Importance = widget.MediumImportance
	default:
		t.connectBtn.SetText(T("mtp.connect"))
		t.connectBtn.SetIcon(theme.LoginIcon())
		t.connectBtn.Importance = widget.HighImportance
	}
	t.connectBtn.Refresh()
	editable := t.fs == nil && t.connecting == nil
	for _, e := range []*widget.Entry{t.host, t.user, t.pass} {
		if editable {
			e.Enable()
		} else {
			e.Disable()
		}
	}
	t.status.SetText(T(t.statusID, t.statusAr...))
	t.status.Importance = t.statusIm
	t.status.Refresh()
}

// installMode reports whether the install port (6000) is chosen.
func (t *ftpTab) installMode() bool { return t.install }

func (t *ftpTab) setInstallMode(install bool) {
	t.install = install
	if install {
		t.mode.SetSelected(i18n.T("ftp.mode_install"))
	} else {
		t.mode.SetSelected(i18n.T("ftp.mode_sd"))
	}
}

func (t *ftpTab) setStatus(id string, imp widget.Importance, args ...any) {
	t.statusID, t.statusIm, t.statusAr = id, imp, args
	t.render()
}

func (t *ftpTab) toggleConnect() {
	switch {
	case t.fs != nil:
		t.disconnect()
	case t.connecting != nil:
		t.connecting()
	default:
		t.connect()
	}
}

// address returns host:port; a port typed into the host field wins over the mode.
func (t *ftpTab) address() string {
	host := strings.TrimSpace(t.host.Text)
	if _, _, err := net.SplitHostPort(host); err == nil {
		return host
	}
	port := ftpPortSD
	if t.installMode() {
		port = ftpPortInstall
	}
	return net.JoinHostPort(strings.Trim(host, "[]"), strconv.Itoa(port))
}

func (t *ftpTab) connect() {
	if strings.TrimSpace(t.host.Text) == "" {
		dialog.ShowError(errors.New(i18n.T("ftp.host_required")), t.u.win)
		return
	}
	prefs := t.u.app.Preferences()
	prefs.SetString(prefFTPHost, strings.TrimSpace(t.host.Text))
	prefs.SetString(prefFTPUser, strings.TrimSpace(t.user.Text))

	addr, user, pass := t.address(), strings.TrimSpace(t.user.Text), t.pass.Text
	install := t.installMode()
	ctx, cancel := context.WithCancel(context.Background())
	t.connecting = cancel
	t.setStatus("ftp.connecting", widget.WarningImportance, addr)
	go func() {
		fs, closeFS, err := openFTP(ctx, addr, user, pass, t.u.log)
		runOnUI(func() {
			t.connecting = nil
			if err != nil {
				if !errors.Is(err, context.Canceled) {
					t.u.log.Error("FTP connect failed", "addr", addr, "err", err)
					dialog.ShowError(errors.New(i18n.T("mtp.failed", err)), t.u.win)
				}
				t.setStatus("mtp.disconnected", widget.MediumImportance)
				return
			}
			t.u.log.Info("FTP connected", "addr", addr, "install", install)
			t.fs, t.closeFS = fs, closeFS
			t.setStatus("mtp.connected", widget.SuccessImportance, addr)
			root := rRoot{ID: "/", Name: i18n.T("ftp.mode_sd")}
			if install {
				root = rRoot{ID: "/", Name: i18n.T("ftp.mode_install"), Flat: true}
			}
			t.b.open(fs, []rRoot{root})
		})
	}()
}

func (t *ftpTab) disconnect() {
	if t.connecting != nil {
		t.connecting()
		t.connecting = nil
	}
	if t.closeFS != nil {
		go t.closeFS()
	}
	t.fs, t.closeFS = nil, nil
	t.b.close()
	t.setStatus("mtp.disconnected", widget.MediumImportance)
}
