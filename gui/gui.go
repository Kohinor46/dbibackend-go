// Package gui is the Fyne desktop interface for the DBI backend.
package gui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	fyneLang "fyne.io/fyne/v2/lang"
	"github.com/ncruces/zenity"

	"github.com/Kohinor46/dbibackend-go/dbi"
	"github.com/Kohinor46/dbibackend-go/i18n"
	"github.com/Kohinor46/dbibackend-go/usbconn"
	"github.com/Kohinor46/dbibackend-go/winusb"
)

const (
	prefDir  = "titlesDir"
	prefLang = "language"
)

type ui struct {
	app      fyne.App
	win      fyne.Window
	log      *slog.Logger
	logLevel *slog.LevelVar
	logView  *logView
	mtp      *mtpTab
	ftp      *ftpTab
	tabs     *container.AppTabs
	drops    []func([]fyne.URI) // per tab: what dropping files does; nil = pick the folder

	// Rebuilt by build() when the language changes.
	dirEntry   *widget.Entry
	browseBtn  *widget.Button
	startBtn   *widget.Button
	driverBtn  *widget.Button // Windows only
	status     *widget.Label
	list       *widget.List
	countLabel *widget.Label
	curLabel   *widget.Label
	progress   *widget.ProgressBar

	// Accessed only on the UI goroutine.
	titles        []dbi.Title
	progressText  string
	active        string
	statusKey     string
	statusImp     widget.Importance
	cancel        context.CancelFunc
	serverDone    chan struct{} // closed when the last serveLoop has released the device
	running       bool
	driverOffered bool
	installing    bool
	fileStyle     fileStyle  // how the MTP and FTP browsers draw entries
	appearance    appearance // light, dark or as the system
	logOpen       bool       // the shared log panel is expanded
	logToggle     *widget.Button

	updateTag, updatePage string // a newer release, once found
	sendNotification      func(*fyne.Notification)
}

// Run opens the main window and blocks until it is closed.
// dir preselects the titles directory (may be empty); debug enables verbose logs.
func Run(dir string, debug bool) {
	a := app.NewWithID("com.dbibackend.gui")
	u := newUI(a, debug)

	lang := a.Preferences().String(prefLang)
	if lang == "" {
		lang = i18n.Match(fyneLang.SystemLocale().String())
	}
	i18n.Set(lang)
	u.build()

	if dir == "" {
		dir = a.Preferences().String(prefDir)
	}
	if dir != "" {
		u.setDir(dir) // installing starts only when the user presses Start
	}

	u.win.Resize(fyne.NewSize(760, 640))
	u.checkUpdate()
	u.win.ShowAndRun()
	if u.cancel != nil {
		u.cancel()
	}
}

func newUI(a fyne.App, debug bool) *ui {
	u := &ui{app: a, win: a.NewWindow("DBI Backend"), logLevel: new(slog.LevelVar), statusKey: "status.stopped"}
	if debug {
		u.logLevel.Set(slog.LevelDebug)
	}
	u.sendNotification = a.SendNotification
	u.logView = newLogView()
	u.log = slog.New(&lineHandler{w: io.MultiWriter(os.Stderr, u.logView), level: u.logLevel})
	u.fileStyle = loadFileStyle(a.Preferences())
	if u.appearance = loadAppearance(a.Preferences()); u.appearance != appearanceSystem {
		a.Settings().SetTheme(u.appearance.theme())
	}
	u.mtp = newMTPTab(u)
	u.ftp = newFTPTab(u)
	return u
}

// build creates the window content in the current language. It is called
// again on a language change, so it restores the state kept in u.
func (u *ui) build() {
	T := i18n.T
	dir := ""
	if u.dirEntry != nil {
		dir = u.dirEntry.Text
	}

	// Folder row.
	u.dirEntry = widget.NewEntry()
	u.dirEntry.SetPlaceHolder(T("folder.placeholder"))
	u.dirEntry.SetText(dir)
	u.dirEntry.OnSubmitted = func(s string) { u.setDir(s) }
	u.browseBtn = widget.NewButtonWithIcon(T("folder.browse"), theme.FolderOpenIcon(), u.chooseDir)
	folderRow := container.NewBorder(nil, nil, widget.NewLabel(T("folder.label")), u.browseBtn, u.dirEntry)

	// Status row.
	u.status = widget.NewLabel("")
	u.status.TextStyle.Bold = true
	u.status.Truncation = fyne.TextTruncateEllipsis
	u.startBtn = widget.NewButtonWithIcon("", nil, u.toggle)
	u.refreshStartBtn()
	right := container.NewHBox(u.startBtn)
	if winusb.Supported {
		u.driverBtn = widget.NewButtonWithIcon(T("driver.button"), theme.SettingsIcon(), u.installDriver)
		if u.installing {
			u.driverBtn.Disable()
		}
		right.Objects = append([]fyne.CanvasObject{u.driverBtn}, right.Objects...)
	}
	statusRow := container.NewBorder(nil, nil, widget.NewLabel(T("status.label")), right, u.status)
	u.setStatus(u.statusKey, u.statusImp)

	// Language picker.
	names := make([]string, len(i18n.Languages))
	for i, l := range i18n.Languages {
		names[i] = l.Name
	}
	langSel := widget.NewSelect(names, nil)
	for _, l := range i18n.Languages {
		if l.Code == i18n.Current() {
			langSel.SetSelected(l.Name)
		}
	}
	langSel.OnChanged = func(name string) {
		for _, l := range i18n.Languages {
			if l.Name == name && l.Code != i18n.Current() {
				i18n.Set(l.Code)
				u.app.Preferences().SetString(prefLang, l.Code)
				u.build()
			}
		}
	}

	// Titles list.
	u.list = widget.NewList(
		func() int { return len(u.titles) },
		func() fyne.CanvasObject {
			name := widget.NewLabel("")
			name.Truncation = fyne.TextTruncateEllipsis
			size := widget.NewLabel(humanBytes(1023 << 30)) // sets min width
			size.Alignment = fyne.TextAlignTrailing
			return container.NewBorder(nil, nil, widget.NewIcon(theme.FileIcon()), size, name)
		},
		func(id widget.ListItemID, o fyne.CanvasObject) {
			t := u.titles[id]
			c := o.(*fyne.Container)
			name, icon, size := c.Objects[0].(*widget.Label), c.Objects[1].(*widget.Icon), c.Objects[2].(*widget.Label)
			name.SetText(t.Name)
			size.SetText(humanBytes(t.Size))
			active := t.Name == u.active
			name.TextStyle.Bold = active
			if active {
				icon.SetResource(theme.DownloadIcon())
			} else {
				icon.SetResource(theme.FileIcon())
			}
			name.Refresh()
		},
	)
	u.list.OnSelected = func(widget.ListItemID) { u.list.UnselectAll() }
	u.countLabel = widget.NewLabel("")
	refresh := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), u.rescan)
	titlesHeader := container.NewHBox(widget.NewLabelWithStyle(T("files.title"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), u.countLabel, layout.NewSpacer(), refresh)
	titlesPane := container.NewBorder(titlesHeader, nil, nil, nil, u.list)

	// Transfer progress.
	u.curLabel = widget.NewLabel("")
	u.curLabel.Truncation = fyne.TextTruncateEllipsis
	u.progress = widget.NewProgressBar()
	u.progress.TextFormatter = func() string { return u.progressText }
	progressBox := container.NewVBox(widget.NewSeparator(), u.curLabel, u.progress)
	u.resetTransfer()

	top := container.NewVBox(folderRow, statusRow, widget.NewSeparator())
	install := container.NewBorder(top, progressBox, nil, nil, titlesPane)
	selected := 0
	if u.tabs != nil {
		selected = u.tabs.SelectedIndex()
	}
	u.tabs = container.NewAppTabs(container.NewTabItemWithIcon(T("tab.install"), theme.DownloadIcon(), install))
	u.drops = []func([]fyne.URI){nil}
	if mtpSupported {
		u.tabs.Append(container.NewTabItemWithIcon(T("tab.mtp"), theme.StorageIcon(), u.mtp.build()))
		u.drops = append(u.drops, u.mtp.b.dropped)
	}
	u.tabs.Append(container.NewTabItemWithIcon(T("tab.ftp"), theme.ComputerIcon(), u.ftp.build()))
	u.drops = append(u.drops, u.ftp.b.dropped)
	u.tabs.SelectIndex(selected)
	// Settings and the language picker sit on the tab bar's row, right-aligned.
	settings := widget.NewButtonWithIcon("", theme.SettingsIcon(), u.showSettings)
	corner := container.NewHBox(layout.NewSpacer(), settings, langSel)
	if u.updateTag != "" {
		// Short, so that it fits on the tab bar's row; the log says more.
		update := widget.NewButtonWithIcon(u.updateTag, theme.DownloadIcon(), u.openUpdate)
		update.Importance = widget.HighImportance
		corner.Objects = append([]fyne.CanvasObject{layout.NewSpacer(), update}, corner.Objects[1:]...)
	}
	u.withLog(container.NewStack(u.tabs, container.NewVBox(corner)))

	if isDir(dir) {
		u.setTitles(u.titles)
	} else if dir != "" {
		u.countLabel.SetText(T("folder.not_found"))
	}

	u.win.SetOnDropped(func(_ fyne.Position, uris []fyne.URI) {
		if drop := u.drops[u.tabs.SelectedIndex()]; drop != nil {
			drop(uris)
			return
		}
		for _, uri := range uris {
			if isDir(uri.Path()) {
				u.setDir(uri.Path())
				return
			}
		}
	})
}

// withLog shows main with the log panel under it, shared by all tabs. The
// panel starts collapsed to one line with the latest entry; its header
// button expands it.
func (u *ui) withLog(main fyne.CanvasObject) {
	T := i18n.T
	debug := widget.NewCheck(T("debug"), func(on bool) {
		if on {
			u.logLevel.Set(slog.LevelDebug)
		} else {
			u.logLevel.Set(slog.LevelInfo)
		}
	})
	debug.SetChecked(u.logLevel.Level() == slog.LevelDebug)
	clear := widget.NewButtonWithIcon("", theme.ContentClearIcon(), u.logView.Clear)
	var copyBtn *widget.Button
	copyBtn = widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
		u.app.Clipboard().SetContent(u.logView.Text())
		copyBtn.SetIcon(theme.ConfirmIcon()) // a short "copied"
		time.AfterFunc(1500*time.Millisecond, func() { runOnUI(func() { copyBtn.SetIcon(theme.ContentCopyIcon()) }) })
	})
	save := widget.NewButtonWithIcon("", theme.DocumentSaveIcon(), u.pickSaveLog)
	var render func()
	u.logToggle = widget.NewButtonWithIcon(T("log.title"), nil, func() {
		u.logOpen = !u.logOpen
		render()
	})
	u.logToggle.Importance = widget.LowImportance
	header := container.NewBorder(nil, nil, u.logToggle, container.NewHBox(debug, copyBtn, save, clear), u.logView.last)
	render = func() {
		if u.logOpen {
			u.logToggle.SetIcon(theme.MenuDropDownIcon())
			u.logView.last.Hide()
			split := container.NewVSplit(main, container.NewBorder(header, nil, nil, nil, u.logView.scroll))
			split.Offset = 0.65
			u.win.SetContent(container.NewPadded(split))
			u.logView.scroll.ScrollToBottom()
		} else {
			u.logToggle.SetIcon(theme.MenuDropUpIcon())
			u.logView.last.Show()
			u.win.SetContent(container.NewPadded(container.NewBorder(nil, container.NewVBox(widget.NewSeparator(), header), nil, nil, main)))
		}
	}
	render()
}

func (u *ui) refreshStartBtn() {
	if u.running {
		u.startBtn.SetText(i18n.T("stop"))
		u.startBtn.SetIcon(theme.MediaStopIcon())
		u.startBtn.Importance = widget.DangerImportance
	} else {
		u.startBtn.SetText(i18n.T("start"))
		u.startBtn.SetIcon(theme.MediaPlayIcon())
		u.startBtn.Importance = widget.HighImportance
	}
	u.startBtn.Refresh()
}

// chooseDirFyne is the built-in Fyne folder dialog, used when no native one is available.
func (u *ui) chooseDirFyne() {
	d := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
		if err == nil && uri != nil {
			u.setDir(uri.Path())
		}
	}, u.win)
	if cur := startDir(u.dirEntry.Text); cur != "" {
		if l, err := storage.ListerForURI(storage.NewFileURI(cur)); err == nil {
			d.SetLocation(l)
		}
	}
	d.Show() // the dialog's window only exists after Show, so Resize must come later
	d.Resize(fyne.NewSize(700, 500))
}

// setDir changes the titles folder; a running server is restarted on the new folder.
func (u *ui) setDir(dir string) {
	dir = strings.TrimSpace(dir)
	u.dirEntry.SetText(dir)
	if !isDir(dir) {
		u.setTitles(nil)
		u.countLabel.SetText(i18n.T("folder.not_found"))
		return
	}
	u.app.Preferences().SetString(prefDir, dir)
	u.rescan()
	if u.running {
		u.stop()
		u.start()
	}
}

func (u *ui) rescan() {
	dir := u.dirEntry.Text
	if !isDir(dir) {
		return
	}
	titles, err := dbi.ScanTitles(dir)
	if err != nil {
		u.log.Error("Scan failed", "err", err)
	}
	u.setTitles(titles)
}

func (u *ui) setTitles(titles []dbi.Title) {
	u.titles = titles
	var total int64
	for _, t := range titles {
		total += t.Size
	}
	u.countLabel.SetText(i18n.T("files.count", len(titles), humanBytes(total)))
	u.list.Refresh()
}

func (u *ui) toggle() {
	if u.running {
		u.stop()
	} else {
		u.start()
	}
}

func (u *ui) start() {
	dir := u.dirEntry.Text
	if !isDir(dir) {
		dialog.ShowError(errors.New(i18n.T("folder.required")), u.win)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	u.cancel, u.running = cancel, true
	u.refreshStartBtn()
	prev, done := u.serverDone, make(chan struct{})
	u.serverDone = done
	go func() {
		defer close(done)
		// A restarted server must not open (and reset) the Switch while the
		// previous session is still closing it.
		if prev != nil {
			<-prev
		}
		u.serveLoop(ctx, dir)
	}()
}

func (u *ui) stop() {
	if u.cancel != nil {
		u.cancel()
		u.cancel = nil
	}
	u.running = false
	u.refreshStartBtn()
	u.setStatus("status.stopped", widget.MediumImportance)
	u.resetTransfer()
}

// serveLoop runs on its own goroutine: wait for the Switch, serve one DBI session, repeat.
func (u *ui) serveLoop(ctx context.Context, dir string) {
	u.log.Info("Server started", "dir", dir)
	defer u.log.Info("Server stopped")
	for {
		u.do(ctx, func() { u.setStatus("status.waiting", widget.WarningImportance) })
		noDriver := false
		dev, err := usbconn.Wait(ctx, dbi.SwitchVID, dbi.SwitchPID, func(err error) {
			switch {
			case errors.Is(err, usbconn.ErrNoDriver):
				if !noDriver { // log and prompt once, not every second
					noDriver = true
					u.log.Warn("Cannot open Switch", "err", err)
					u.do(ctx, func() {
						u.setStatus("status.no_driver", widget.DangerImportance)
						u.offerDriver()
					})
				}
			case !errors.Is(err, usbconn.ErrNotFound):
				u.log.Warn("Cannot open Switch", "err", err)
			}
		})
		if err != nil {
			return // cancelled
		}
		u.log.Info("Switch connected")
		u.do(ctx, func() { u.setStatus("status.connected", widget.SuccessImportance) })

		tr := &tracker{}
		start := time.Now()
		srv := &dbi.Server{Dir: dir, Log: u.log, OnEvent: func(e dbi.Event) { u.onEvent(ctx, tr, e) }}
		err = srv.Serve(ctx, dev)
		dev.Close()
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			u.log.Error("Session ended with error", "err", err)
			u.do(ctx, func() { u.notifyAfter(start, "notify.install_failed", err) })
		} else {
			u.log.Info("Session finished")
			if n := len(tr.finished); n > 0 {
				u.do(ctx, func() { u.notifyAfter(start, "notify.install_done", n) })
			}
		}
		u.do(ctx, u.resetTransfer)

		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

// do runs fn on the UI goroutine. Server events may race with stop(),
// so fn is dropped if this server run was cancelled meanwhile.
func (u *ui) do(ctx context.Context, fn func()) {
	fyne.Do(func() {
		if ctx.Err() == nil {
			fn()
		}
	})
}

// tracker computes transfer speed; used only from the serve goroutine.
type tracker struct {
	last      time.Time
	bytes     int64
	speed     float64
	lastTitle string
	finished  map[string]bool // titles sent to the end
}

func (u *ui) onEvent(ctx context.Context, tr *tracker, e dbi.Event) {
	switch e := e.(type) {
	case dbi.ListEvent:
		u.do(ctx, func() { u.setTitles(e.Titles) })
	case dbi.ExitEvent:
		u.do(ctx, u.resetTransfer)
	case dbi.ProgressEvent:
		if e.Pos >= e.Title.Size {
			if tr.finished == nil {
				tr.finished = map[string]bool{}
			}
			tr.finished[e.Title.Name] = true
		}
		now := time.Now()
		if tr.last.IsZero() || e.Title.Name != tr.lastTitle {
			tr.last, tr.bytes, tr.lastTitle = now, 0, e.Title.Name
		}
		tr.bytes += int64(e.Bytes)
		el := now.Sub(tr.last)
		if el < 250*time.Millisecond && e.Pos < e.Title.Size {
			return // throttle UI updates
		}
		if el > 0 {
			cur := float64(tr.bytes) / el.Seconds()
			if tr.speed == 0 {
				tr.speed = cur
			} else {
				tr.speed = 0.7*tr.speed + 0.3*cur
			}
		}
		tr.last, tr.bytes = now, 0
		speed := tr.speed
		u.do(ctx, func() { u.showProgress(e.Title, e.Pos, speed) })
	}
}

func (u *ui) showProgress(t dbi.Title, pos int64, speed float64) {
	if u.active != t.Name {
		u.active = t.Name
		u.list.Refresh()
	}
	u.curLabel.SetText(i18n.T("transfer.sending", t.Name))
	frac := 0.0
	if t.Size > 0 {
		frac = float64(pos) / float64(t.Size)
	}
	u.progressText = i18n.T("transfer.progress", frac*100, humanBytes(pos), humanBytes(t.Size), humanBytes(int64(speed)))
	u.progress.SetValue(frac)
}

func (u *ui) resetTransfer() {
	u.active = ""
	u.list.Refresh()
	u.curLabel.SetText(i18n.T("transfer.none"))
	u.progressText = ""
	u.progress.SetValue(0)
}

func (u *ui) setStatus(key string, imp widget.Importance) {
	u.statusKey, u.statusImp = key, imp
	u.status.SetText(i18n.T(key))
	u.status.Importance = imp
	u.status.Refresh()
}

// pickSaveLog asks where to save the whole log and writes it there.
func (u *ui) pickSaveLog() {
	name := "dbibackend-log-" + time.Now().Format("2006-01-02-150405") + ".txt"
	go func() {
		dest, err := zenity.SelectFileSave(zenity.Title(i18n.T("log.save")), zenity.Filename(name), zenity.ConfirmOverwrite())
		if err != nil {
			if !errors.Is(err, zenity.ErrCanceled) {
				u.log.Warn("File picker failed", "err", err)
			}
			return
		}
		runOnUI(func() { u.saveLog(dest) })
	}()
}

func (u *ui) saveLog(dest string) {
	if err := os.WriteFile(dest, []byte(u.logView.Text()+"\n"), 0o644); err != nil {
		dialog.ShowError(err, u.win)
		return
	}
	u.log.Info("Log saved", "to", dest)
}

// logView is an io.Writer that keeps the log and shows its last lines.
type logView struct {
	mu      sync.Mutex
	lines   []string
	pending bool // a UI refresh is already queued
	label   *widget.Label
	scroll  *container.Scroll
	last    *widget.Label // the latest line, shown while the panel is collapsed
}

const (
	maxLogLines  = 500    // shown in the panel
	maxKeptLines = 20_000 // kept for Copy and Save
)

func newLogView() *logView {
	l := &logView{label: widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Monospace: true})}
	l.label.Selectable = true
	l.scroll = container.NewScroll(l.label)
	l.last = widget.NewLabel("")
	l.last.Truncation = fyne.TextTruncateEllipsis
	return l
}

func (l *logView) Write(p []byte) (int, error) {
	l.mu.Lock()
	l.lines = append(l.lines, strings.Split(strings.TrimRight(string(p), "\n"), "\n")...)
	if len(l.lines) > maxKeptLines {
		l.lines = l.lines[len(l.lines)-maxKeptLines:]
	}
	// Coalesce bursts of log lines into one re-render of the label.
	schedule := !l.pending
	l.pending = true
	l.mu.Unlock()
	if schedule {
		runOnUI(l.flush)
	}
	return len(p), nil
}

func (l *logView) flush() {
	l.mu.Lock()
	text := strings.Join(l.lines[max(0, len(l.lines)-maxLogLines):], "\n")
	last := ""
	if len(l.lines) > 0 {
		last = l.lines[len(l.lines)-1]
	}
	l.pending = false
	l.mu.Unlock()
	l.label.SetText(text)
	l.last.SetText(last)
	l.scroll.ScrollToBottom()
}

// Text returns the whole kept log.
func (l *logView) Text() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Join(l.lines, "\n")
}

func (l *logView) Clear() {
	l.mu.Lock()
	l.lines = nil
	l.mu.Unlock()
	l.label.SetText("")
	l.last.SetText("")
}

func isDir(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func humanBytes(n int64) string {
	units := []string{"unit.B", "unit.KB", "unit.MB", "unit.GB", "unit.TB"}
	if n < 1024 {
		return fmt.Sprintf("%d %s", n, i18n.T(units[0]))
	}
	v, exp := float64(n), 0
	for v >= 1024 && exp < len(units)-1 {
		v /= 1024
		exp++
	}
	return fmt.Sprintf("%.1f %s", v, i18n.T(units[exp]))
}
