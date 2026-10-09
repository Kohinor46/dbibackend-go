package gui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/ncruces/zenity"

	"github.com/Kohinor46/dbibackend-go/i18n"
)

// runOnUI runs f on the UI goroutine (fyne.Do); tests replace it to run
// UI updates on the test goroutine, which plays the UI thread there.
var runOnUI = fyne.Do

// browser is the file manager shared by the MTP and FTP tabs: roots,
// folders, upload (merging folders, see uploader), download, delete and new
// folder, with progress and cancel. The tab owns the connection and hands
// the browser a remoteFS. All fields are used on the UI goroutine; remote
// operations run in the background one at a time (see run).
type browser struct {
	u        *ui
	titleKey string           // i18n key of the tab title, for dialogs
	broken   func(error) bool // the connection can't continue after this error
	onBroken func()           // disconnects; called on the UI goroutine

	fs     remoteFS
	roots  []rRoot
	root   int
	path   []rEntry // folders below the root
	items  []rEntry
	busy   bool
	cancel context.CancelFunc // cancels the running operation

	// Widgets, rebuilt by build().
	rootSel     *widget.Select
	rootRow     *fyne.Container
	pathLabel   *widget.Label
	view        fileView
	viewBox     *fyne.Container // holds view; restyle swaps it
	emptyLabel  *widget.Label
	toolbar     []*widget.Button
	progress    *widget.ProgressBar
	progressLbl *widget.Label
	progressBox *fyne.Container
	progressTxt string
	content     *fyne.Container // re-laid out when progressBox shows/hides
}

func newBrowser(u *ui, titleKey string, broken func(error) bool, onBroken func()) *browser {
	return &browser{u: u, titleKey: titleKey, broken: broken, onBroken: onBroken}
}

// build returns the browser below header, the tab's own connection rows.
func (b *browser) build(header ...fyne.CanvasObject) fyne.CanvasObject {
	T := i18n.T
	b.rootSel = widget.NewSelect(nil, func(string) {
		if i := b.rootSel.SelectedIndex(); i >= 0 && i != b.root {
			b.root, b.path = i, nil
			b.refresh()
		}
	})
	b.rootRow = container.NewBorder(nil, nil, widget.NewLabel(T("mtp.storage")), nil, b.rootSel)
	b.pathLabel = widget.NewLabel("")
	b.pathLabel.Truncation = fyne.TextTruncateEllipsis

	up := widget.NewButtonWithIcon("", theme.NavigateBackIcon(), b.up)
	refresh := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), b.refresh)
	upload := widget.NewButtonWithIcon(T("mtp.upload"), theme.UploadIcon(), b.pickUpload)
	uploadFolder := widget.NewButtonWithIcon(T("mtp.upload_folder"), theme.FolderOpenIcon(), b.pickUploadFolder)
	newFolder := widget.NewButtonWithIcon(T("mtp.new_folder"), theme.FolderNewIcon(), b.askNewFolder)
	b.toolbar = []*widget.Button{up, refresh, upload, uploadFolder, newFolder}

	b.view = newFileView(b.u.fileStyle, b.host())
	b.viewBox = container.NewStack(b.view.obj)
	b.emptyLabel = widget.NewLabel(T("mtp.empty"))
	b.emptyLabel.Alignment = fyne.TextAlignCenter

	b.progress = widget.NewProgressBar()
	b.progress.TextFormatter = func() string { return b.progressTxt }
	b.progressLbl = widget.NewLabel("")
	b.progressLbl.Truncation = fyne.TextTruncateEllipsis
	cancel := widget.NewButtonWithIcon(T("cancel"), theme.CancelIcon(), func() {
		if b.cancel != nil {
			b.cancel()
		}
	})
	b.progressBox = container.NewVBox(widget.NewSeparator(), container.NewBorder(nil, nil, nil, cancel, b.progressLbl), b.progress)
	b.progressBox.Hide()

	rows := append(header, b.rootRow,
		container.NewBorder(nil, nil, container.NewHBox(up, refresh), container.NewHBox(newFolder, uploadFolder, upload), b.pathLabel),
		widget.NewSeparator(),
	)
	b.content = container.NewBorder(container.NewVBox(rows...), b.progressBox, nil, nil,
		container.NewStack(b.viewBox, container.NewCenter(b.emptyLabel)))
	b.render()
	return b.content
}

// host connects a file view to the browser.
func (b *browser) host() viewHost {
	return viewHost{
		items:    func() []rEntry { return b.items },
		busy:     func() bool { return b.busy },
		enter:    b.enter,
		download: b.pickDownload,
		delete:   b.askDelete,
	}
}

// restyle redraws the entries in the style chosen in Settings.
func (b *browser) restyle() {
	if b.viewBox == nil {
		return // not built yet
	}
	b.view = newFileView(b.u.fileStyle, b.host())
	b.viewBox.Objects = []fyne.CanvasObject{b.view.obj}
	b.viewBox.Refresh()
}

// enter opens a folder of the current one.
func (b *browser) enter(e rEntry) {
	if e.Dir && !b.busy {
		b.path = append(b.path, e)
		b.refresh()
	}
}

// open shows a newly connected remote.
func (b *browser) open(fs remoteFS, roots []rRoot) {
	b.fs, b.roots, b.root, b.path, b.items = fs, roots, 0, nil, nil
	b.refresh()
}

// close forgets the remote (the tab has disconnected).
func (b *browser) close() {
	if b.cancel != nil {
		b.cancel()
	}
	b.fs, b.roots, b.path, b.items, b.busy, b.cancel = nil, nil, nil, nil, false, nil
	if b.content != nil {
		b.showProgress(false)
		b.render()
	}
}

// showProgress shows or hides the progress area; the Border layout only
// makes room for it after a refresh.
func (b *browser) showProgress(show bool) {
	if show {
		b.progressBox.Show()
	} else {
		b.progressBox.Hide()
	}
	b.content.Refresh()
}

// render shows the current state in the widgets.
func (b *browser) render() {
	connected := b.fs != nil
	options := make([]string, len(b.roots))
	for i, r := range b.roots {
		options[i] = r.Name
	}
	b.rootSel.SetOptions(options)
	if b.root < len(options) {
		b.rootSel.SetSelectedIndex(b.root)
	} else {
		b.rootSel.ClearSelected()
	}
	if len(b.roots) > 1 {
		b.rootRow.Show()
	} else {
		b.rootRow.Hide() // nothing to pick before connecting or with a single root (FTP)
	}

	names := make([]string, len(b.path))
	for i, f := range b.path {
		names[i] = f.Name
	}
	b.pathLabel.SetText("/" + strings.Join(names, "/"))

	for i, btn := range b.toolbar {
		if !connected || b.busy || (i == 0 && len(b.path) == 0) {
			btn.Disable()
		} else {
			btn.Enable()
		}
	}
	if connected && !b.busy {
		b.rootSel.Enable()
	} else {
		b.rootSel.Disable()
	}
	if connected && len(b.items) == 0 && !b.busy {
		b.emptyLabel.Show()
	} else {
		b.emptyLabel.Hide()
	}
	b.view.refresh()
}

// current returns the folder being shown.
func (b *browser) current() string {
	if len(b.path) > 0 {
		return b.path[len(b.path)-1].ID
	}
	return b.roots[b.root].ID
}

func (b *browser) up() {
	if len(b.path) > 0 {
		b.path = b.path[:len(b.path)-1]
		b.refresh()
	}
}

func (b *browser) refresh() {
	if b.fs == nil || b.root >= len(b.roots) {
		b.items = nil
		b.render()
		return
	}
	dir := b.current()
	var items []rEntry
	b.run("", 0, func(ctx context.Context, fs remoteFS, _ func(int64)) error {
		var err error
		items, err = fs.List(ctx, dir)
		sort.Slice(items, func(i, j int) bool {
			if items[i].Dir != items[j].Dir {
				return items[i].Dir
			}
			return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
		})
		return err
	}, func() { b.items = items })
}

// run executes op in the background with the browser locked. label and
// total enable the progress bar (op reports bytes done through its
// callback); then runs on the UI goroutine after a successful op. op must
// use the remoteFS it is given, never b's fields, which belong to the UI
// goroutine.
func (b *browser) run(label string, total int64, op func(ctx context.Context, fs remoteFS, progress func(int64)) error, then func()) {
	if b.busy || b.fs == nil {
		return
	}
	// The op gets the remote as of now: a disconnect may clear b.fs meanwhile.
	fs := b.fs
	ctx, cancel := context.WithCancel(context.Background())
	b.busy, b.cancel = true, cancel
	if label != "" {
		b.progressLbl.SetText(label)
		b.progressTxt = ""
		b.progress.SetValue(0)
		b.showProgress(true)
	}
	b.render()

	start := time.Now()
	var last time.Time
	progress := func(done int64) {
		if now := time.Now(); now.Sub(last) >= 200*time.Millisecond || done == total {
			last = now
			speed := float64(done) / now.Sub(start).Seconds()
			runOnUI(func() {
				if total > 0 {
					b.progressTxt = i18n.T("transfer.progress", float64(done)*100/float64(total), humanBytes(done), humanBytes(total), humanBytes(int64(speed)))
					b.progress.SetValue(float64(done) / float64(total))
				}
			})
		}
	}
	go func() {
		err := op(ctx, fs, progress)
		runOnUI(func() {
			if b.fs != fs {
				return // disconnected meanwhile; close() already reset the state
			}
			b.busy, b.cancel = false, nil
			b.showProgress(false)
			switch {
			case err == nil:
				if then != nil {
					then()
				}
				b.render()
			case b.broken(err) || errors.Is(err, context.Canceled):
				// The transfer was cut midway: the connection can't continue.
				b.u.log.Warn("Transfer interrupted", "err", err)
				b.onBroken()
				if !errors.Is(err, context.Canceled) {
					dialog.ShowError(errors.New(i18n.T("mtp.lost")), b.u.win)
				}
			default:
				b.u.log.Error("Operation failed", "err", err)
				b.render()
				dialog.ShowError(errors.New(i18n.T("mtp.failed", err)), b.u.win)
			}
		})
	}()
}

func (b *browser) pickUpload() {
	go func() {
		files, err := zenity.SelectFileMultiple(zenity.Title(i18n.T("mtp.upload")))
		if err != nil {
			if !errors.Is(err, zenity.ErrCanceled) {
				b.u.log.Warn("File picker failed", "err", err)
			}
			return
		}
		runOnUI(func() { b.upload(files) })
	}()
}

func (b *browser) pickUploadFolder() {
	go func() {
		dir, err := zenity.SelectFile(zenity.Directory(), zenity.Title(i18n.T("mtp.upload_folder")))
		if err != nil {
			if !errors.Is(err, zenity.ErrCanceled) {
				b.u.log.Warn("Folder picker failed", "err", err)
			}
			return
		}
		runOnUI(func() { b.upload([]string{dir}) })
	}()
}

// upload sends local files and folders into the current folder, merging
// folders that already exist (see uploader). Into flat roots (DBI's install
// targets) the files of selected folders are sent without the folders.
func (b *browser) upload(paths []string) {
	if b.fs == nil || b.busy {
		return
	}
	items, total := collectLocal(paths)
	folders := 0
	for _, it := range items {
		if it.dir {
			folders++
		}
	}
	if len(items) == 0 {
		return
	}
	dir, flat := b.current(), b.roots[b.root].Flat
	var stats uploadStats
	b.run(i18n.T("mtp.uploading", path.Base(items[0].rel)), total, func(ctx context.Context, fs remoteFS, progress func(int64)) error {
		u := newUploader(ctx, fs, dir, flat, b.u.log)
		u.progress = progress
		u.onFile = func(name string, n, total int) {
			text := i18n.T("mtp.uploading", fmt.Sprintf("%s  (%d/%d)", name, n, total))
			runOnUI(func() { b.progressLbl.SetText(text) })
		}
		err := u.run(items)
		stats = u.stats
		return err
	}, func() {
		b.refresh()
		if folders > 0 || stats.replaced > 0 || stats.skipped > 0 {
			dialog.ShowInformation(i18n.T(b.titleKey), i18n.T("mtp.upload_done", stats.uploaded, stats.replaced, stats.skipped), b.u.win)
		}
	})
}

func (b *browser) pickDownload(e rEntry) {
	go func() {
		dest, err := zenity.SelectFileSave(zenity.Title(i18n.T("mtp.download")), zenity.Filename(e.Name), zenity.ConfirmOverwrite())
		if err != nil {
			if !errors.Is(err, zenity.ErrCanceled) {
				b.u.log.Warn("File picker failed", "err", err)
			}
			return
		}
		runOnUI(func() { b.download(e, dest) })
	}()
}

func (b *browser) download(e rEntry, dest string) {
	b.run(i18n.T("mtp.downloading", e.Name), e.Size, func(ctx context.Context, fs remoteFS, progress func(int64)) error {
		f, err := os.Create(dest)
		if err != nil {
			return err
		}
		err = fs.Download(ctx, e, &countingWriter{w: f, report: progress})
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			os.Remove(dest) // don't leave a truncated file behind
			return err
		}
		b.u.log.Info("Downloaded", "file", e.Name, "to", dest)
		return nil
	}, nil)
}

func (b *browser) askNewFolder() {
	entry := widget.NewEntry()
	d := dialog.NewCustomConfirm(i18n.T("mtp.new_folder"), i18n.T("mtp.create"), i18n.T("cancel"),
		container.NewBorder(nil, nil, widget.NewLabel(i18n.T("mtp.folder_name")), nil, entry),
		func(ok bool) {
			if ok {
				b.makeFolder(entry.Text)
			}
		}, b.u.win)
	d.Resize(fyne.NewSize(420, 0))
	d.Show()
	b.u.win.Canvas().Focus(entry)
}

func (b *browser) askDelete(e rEntry) {
	dialog.NewCustomConfirm(i18n.T("mtp.delete"), i18n.T("mtp.delete"), i18n.T("cancel"),
		widget.NewLabel(i18n.T("mtp.delete_confirm", e.Name)),
		func(ok bool) {
			if ok {
				b.delete(e)
			}
		}, b.u.win).Show()
}

func (b *browser) makeFolder(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	dir := b.current()
	b.run("", 0, func(ctx context.Context, fs remoteFS, _ func(int64)) error {
		_, err := fs.MakeDir(ctx, dir, name)
		return err
	}, b.refresh)
}

func (b *browser) delete(e rEntry) {
	b.run("", 0, func(ctx context.Context, fs remoteFS, _ func(int64)) error {
		return fs.Delete(ctx, e)
	}, b.refresh)
}

// dropped uploads files and folders dropped on the window while the tab is shown.
func (b *browser) dropped(uris []fyne.URI) {
	paths := make([]string, 0, len(uris))
	for _, u := range uris {
		paths = append(paths, u.Path())
	}
	b.upload(paths)
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
