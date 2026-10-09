package gui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
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

	fs      remoteFS
	roots   []rRoot
	root    int
	path    []rEntry // folders below the root
	items   []rEntry
	busy    bool
	cancel  context.CancelFunc // cancels the running operation
	started time.Time          // when the running (or last) operation started

	// pending is an upload that broke off; it can be resumed, also after
	// reconnecting.
	pending *pendingUpload

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
	resumeLbl   *widget.Label
	resumeBar   *fyne.Container
	content     *fyne.Container // re-laid out when progressBox shows/hides
}

// pendingUpload is enough to run an upload again: what, and where (by
// folder names, since IDs can change after reconnecting).
type pendingUpload struct {
	paths  []string
	rootID string
	dirs   []string // folder names from the root to the destination
	flat   bool
	redo   string   // the file being sent when it broke
	names  nameMode // what to do with names the backend can't store
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

	b.resumeLbl = widget.NewLabel("")
	b.resumeLbl.Truncation = fyne.TextTruncateEllipsis
	resume := widget.NewButtonWithIcon(T("mtp.resume"), theme.MediaReplayIcon(), b.resumeUpload)
	resume.Importance = widget.HighImportance
	dismiss := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
		b.pending = nil
		b.render()
	})
	b.resumeBar = container.NewBorder(nil, nil, widget.NewIcon(theme.WarningIcon()), container.NewHBox(resume, dismiss), b.resumeLbl)

	rows := append(header, b.rootRow,
		container.NewBorder(nil, nil, container.NewHBox(up, refresh), container.NewHBox(newFolder, uploadFolder, upload), b.pathLabel),
		b.resumeBar,
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
	if b.pending != nil && connected && !b.busy && b.rootIndex(b.pending.rootID) >= 0 {
		b.resumeLbl.SetText(i18n.T("mtp.resume_hint", path.Base(b.pending.paths[0])))
		b.resumeBar.Show()
	} else {
		b.resumeBar.Hide()
	}
	if connected && len(b.items) == 0 && !b.busy {
		b.emptyLabel.Show()
	} else {
		b.emptyLabel.Hide()
	}
	b.view.refresh()
}

// rootIndex returns the index of the root with this ID, or -1.
func (b *browser) rootIndex(id string) int {
	for i, r := range b.roots {
		if r.ID == id {
			return i
		}
	}
	return -1
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
	b.run("", func(ctx context.Context, fs remoteFS, _ func(int64, int64)) error {
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

// run executes op in the background with the browser locked. label
// enables the progress bar: op reports bytes done (and the total) through
// its callback. then runs on the UI goroutine after a successful op. op
// must use the remoteFS it is given, never b's fields, which belong to the
// UI goroutine. A failed transfer (label set) is notified.
func (b *browser) run(label string, op func(ctx context.Context, fs remoteFS, progress func(done, total int64)) error, then func()) {
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
	b.started = start
	var last time.Time
	progress := func(done, total int64) {
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
					b.u.notifyAfter(start, "notify.lost")
					dialog.ShowError(errors.New(i18n.T("mtp.lost")), b.u.win)
				}
			default:
				b.u.log.Error("Operation failed", "err", err)
				b.render()
				if label != "" {
					b.u.notifyAfter(start, "mtp.failed", err)
				}
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
	if b.fs == nil || b.busy || len(paths) == 0 {
		return
	}
	dirs := make([]string, len(b.path))
	for i, f := range b.path {
		dirs[i] = f.Name
	}
	r := b.roots[b.root]
	job := &pendingUpload{paths: paths, rootID: r.ID, dirs: dirs, flat: r.Flat}
	if _, ok := b.fs.(latinOnly); !ok {
		b.startUpload(job, false)
		return
	}
	items, _ := collectLocal(paths)
	var bad []string
	for _, it := range items {
		if !isLatinName(path.Base(it.rel)) {
			bad = append(bad, it.rel)
		}
	}
	if len(bad) == 0 {
		b.startUpload(job, false)
		return
	}
	b.askNames(bad, func(mode nameMode) {
		job.names = mode
		b.startUpload(job, false)
	})
}

// askNames asks what to do with names DBI's FTP server can't store:
// rename them with Latin letters, or skip them.
func (b *browser) askNames(bad []string, then func(nameMode)) {
	T := i18n.T
	example := path.Base(bad[0])
	text := widget.NewLabel(T("ftp.names_question", len(bad), example, latinName(example)))
	text.Wrapping = fyne.TextWrapWord
	var d *dialog.CustomDialog
	choice := func(mode nameMode) func() {
		return func() {
			d.Hide()
			then(mode)
		}
	}
	rename := widget.NewButton(T("ftp.names_rename"), choice(namesLatin))
	rename.Importance = widget.HighImportance
	d = dialog.NewCustomWithoutButtons(T("ftp.names_title"), text, b.u.win)
	d.SetButtons([]fyne.CanvasObject{
		widget.NewButton(T("cancel"), func() { d.Hide() }),
		widget.NewButton(T("ftp.names_skip"), choice(namesSkip)),
		rename,
	})
	d.Resize(fyne.NewSize(520, 0))
	d.Show()
}

// resumeUpload runs the broken-off upload again, skipping the files that
// already arrived.
func (b *browser) resumeUpload() {
	if b.pending != nil && b.fs != nil && !b.busy && b.rootIndex(b.pending.rootID) >= 0 {
		b.startUpload(b.pending, true)
	}
}

func (b *browser) startUpload(job *pendingUpload, resume bool) {
	items, total := collectLocal(job.paths)
	if len(items) == 0 {
		return
	}
	folders := 0
	for _, it := range items {
		if it.dir {
			folders++
		}
	}
	b.pending = job // until it succeeds
	redo := job.redo
	var stats uploadStats
	b.run(i18n.T("mtp.uploading", path.Base(items[0].rel)), func(ctx context.Context, fs remoteFS, progress func(int64, int64)) error {
		dir, err := resolveDir(ctx, fs, job.rootID, job.dirs)
		if err != nil {
			return err
		}
		u := newUploader(ctx, fs, dir, job.flat, b.u.log)
		u.resume, u.redo, u.names = resume, redo, job.names
		u.progress = func(done int64) { progress(done, total) }
		u.onFile = func(name string, n, count int) {
			text := i18n.T("mtp.uploading", fmt.Sprintf("%s  (%d/%d)", name, n, count))
			runOnUI(func() {
				job.redo = name // runs before the result: the UI queue keeps order
				b.progressLbl.SetText(text)
			})
		}
		err = u.run(items)
		stats = u.stats
		return err
	}, func() {
		b.pending = nil
		b.refresh()
		b.u.notifyAfter(b.started, "notify.upload_done", stats.uploaded)
		switch {
		case resume:
			dialog.ShowInformation(i18n.T(b.titleKey), i18n.T("mtp.resume_done", stats.uploaded, stats.present, stats.replaced, stats.skipped), b.u.win)
		case folders > 0 || stats.replaced > 0 || stats.skipped > 0:
			dialog.ShowInformation(i18n.T(b.titleKey), i18n.T("mtp.upload_done", stats.uploaded, stats.replaced, stats.skipped), b.u.win)
		}
	})
}

// pickDownload asks where to save e: a file name for a file, a parent
// folder for a folder.
func (b *browser) pickDownload(e rEntry) {
	go func() {
		var dest string
		var err error
		if e.Dir {
			dest, err = zenity.SelectFile(zenity.Directory(), zenity.Title(i18n.T("mtp.download_folder")))
		} else {
			dest, err = zenity.SelectFileSave(zenity.Title(i18n.T("mtp.download")), zenity.Filename(e.Name), zenity.ConfirmOverwrite())
		}
		if err != nil {
			if !errors.Is(err, zenity.ErrCanceled) {
				b.u.log.Warn("File picker failed", "err", err)
			}
			return
		}
		runOnUI(func() {
			if e.Dir {
				b.downloadFolder(e, filepath.Join(dest, safeName(e.Name)), nil)
			} else {
				b.download(e, dest)
			}
		})
	}()
}

func (b *browser) download(e rEntry, dest string) {
	b.run(i18n.T("mtp.downloading", e.Name), func(ctx context.Context, fs remoteFS, progress func(int64, int64)) error {
		f, err := os.Create(dest)
		if err != nil {
			return err
		}
		err = fs.Download(ctx, e, &countingWriter{w: f, report: func(n int64) { progress(n, e.Size) }})
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			os.Remove(dest) // don't leave a truncated file behind
			return err
		}
		b.u.log.Info("Downloaded", "file", e.Name, "to", dest)
		return nil
	}, func() { b.u.notifyAfter(b.started, "notify.download_done", e.Name) })
}

// downloadFolder copies the remote folder e with its contents into dest;
// done, if set, runs after a success with the number of files.
func (b *browser) downloadFolder(e rEntry, dest string, done func(files int)) {
	files := 0
	b.run(i18n.T("mtp.downloading", e.Name), func(ctx context.Context, fs remoteFS, progress func(int64, int64)) error {
		t := &treeDownload{ctx: ctx, fs: fs, progress: progress, onFile: func(rel string, n, count int) {
			text := i18n.T("mtp.downloading", fmt.Sprintf("%s  (%d/%d)", rel, n, count))
			runOnUI(func() { b.progressLbl.SetText(text) })
		}}
		var err error
		files, err = t.run(e.ID, dest)
		if err == nil {
			b.u.log.Info("Downloaded folder", "folder", e.Name, "files", files, "to", dest)
		}
		return err
	}, func() {
		b.u.notifyAfter(b.started, "notify.download_folder_done", e.Name, files)
		if done != nil {
			done(files)
		} else {
			dialog.ShowInformation(i18n.T(b.titleKey), i18n.T("mtp.download_folder_done", files, dest), b.u.win)
		}
	})
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
	b.run("", func(ctx context.Context, fs remoteFS, _ func(int64, int64)) error {
		_, err := fs.MakeDir(ctx, dir, name)
		return err
	}, b.refresh)
}

func (b *browser) delete(e rEntry) {
	b.run("", func(ctx context.Context, fs remoteFS, _ func(int64, int64)) error {
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
