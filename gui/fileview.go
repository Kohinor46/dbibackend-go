package gui

import (
	"image/color"
	"path"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Kohinor46/dbibackend-go/i18n"
)

// fileStyle is how the MTP and FTP browsers draw folders and files; it is
// chosen in Settings.
type fileStyle int

const (
	styleClassic fileStyle = iota // plain icons, as in the first versions
	styleColored                  // coloured icons by type
	styleBadges                   // coloured format badges (NSP, NSZ, XCI...)
	styleTable                    // columns: name, type, size
	styleTwoLine                  // large icons, details under the name
	styleTiles                    // icon grid

	numFileStyles

	prefFileStyle    = "fileStyle"
	defaultFileStyle = styleColored
)

func (s fileStyle) name() string {
	return i18n.T([...]string{"style.classic", "style.colored", "style.badges", "style.table", "style.two_line", "style.tiles"}[s])
}

// loadFileStyle reads the chosen style; unknown values fall back to the default.
func loadFileStyle(p fyne.Preferences) fileStyle {
	s := fileStyle(p.IntWithFallback(prefFileStyle, int(defaultFileStyle)))
	if s < 0 || s >= numFileStyles {
		return defaultFileStyle
	}
	return s
}

// viewHost is what a file view shows and what its rows do.
type viewHost struct {
	items    func() []rEntry
	busy     func() bool
	enter    func(rEntry) // open a folder
	download func(rEntry)
	delete   func(rEntry)
}

// fileView is a list (or grid) of entries drawn in one style.
type fileView struct {
	obj     fyne.CanvasObject // what goes into the layout
	refresh func()            // redraws the rows after items or busy changed
}

// newFileView builds the view for style s.
func newFileView(s fileStyle, h viewHost) fileView {
	switch s {
	case styleColored:
		return listView(h, func() rowParts { return newIconRow(true) })
	case styleBadges:
		return listView(h, newBadgeRow)
	case styleTable:
		return tableView(h)
	case styleTwoLine:
		return listView(h, newTwoLineRow)
	case styleTiles:
		return tilesView(h)
	}
	return listView(h, func() rowParts { return newIconRow(false) })
}

// --- Entry kinds -----------------------------------------------------------

type entryKind int

const (
	kindFile entryKind = iota
	kindGame
	kindApp
	kindImage
)

func ext(name string) string { return strings.ToUpper(strings.TrimPrefix(path.Ext(name), ".")) }

func kindOf(e rEntry) entryKind {
	switch ext(e.Name) {
	case "NSP", "NSZ", "XCI", "XCZ":
		return kindGame
	case "NRO":
		return kindApp
	case "JPG", "JPEG", "PNG", "MP4":
		return kindImage
	}
	return kindFile
}

// kindName is the "type" column: "Folder", "Game NSP", "File INI"...
func kindName(e rEntry) string {
	if e.Dir {
		return i18n.T("kind.folder")
	}
	switch kindOf(e) {
	case kindGame:
		return i18n.T("kind.game", ext(e.Name))
	case kindApp:
		return i18n.T("kind.app")
	case kindImage:
		return i18n.T("kind.image")
	}
	if x := ext(e.Name); x != "" {
		return i18n.T("kind.file_ext", x)
	}
	return i18n.T("kind.file")
}

// gamepadIcon is Material Design's "videogame_asset" (Apache 2.0).
var gamepadIcon = fyne.NewStaticResource("gamepad.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="#000000" d="M21 6H3c-1.1 0-2 .9-2 2v8c0 1.1.9 2 2 2h18c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2zm-10 7H8v3H6v-3H3v-2h3V8h2v3h3v2zm4.5 2c-.83 0-1.5-.67-1.5-1.5s.67-1.5 1.5-1.5 1.5.67 1.5 1.5-.67 1.5-1.5 1.5zm4-3c-.83 0-1.5-.67-1.5-1.5S18.67 9 19.5 9s1.5.67 1.5 1.5-.67 1.5-1.5 1.5z"/></svg>`))

func coloredIcon(e rEntry) fyne.Resource {
	if e.Dir {
		return theme.NewColoredResource(theme.FolderIcon(), theme.ColorNameWarning)
	}
	switch kindOf(e) {
	case kindGame:
		return theme.NewColoredResource(gamepadIcon, theme.ColorNamePrimary)
	case kindApp:
		return theme.NewColoredResource(theme.FileApplicationIcon(), theme.ColorNameSuccess)
	case kindImage:
		return theme.FileImageIcon()
	}
	return theme.FileIcon()
}

func plainIcon(e rEntry) fyne.Resource {
	if e.Dir {
		return theme.FolderIcon()
	}
	return theme.FileIcon()
}

func badgeColor(x string) color.Color {
	switch x {
	case "NSP":
		return color.NRGBA{0x3d, 0x7e, 0xf0, 0xff}
	case "NSZ":
		return color.NRGBA{0x8e, 0x5c, 0xe6, 0xff}
	case "XCI", "XCZ":
		return color.NRGBA{0x2f, 0xa8, 0x6a, 0xff}
	case "NRO":
		return color.NRGBA{0xe0, 0x8a, 0x1e, 0xff}
	}
	return color.NRGBA{0x60, 0x64, 0x6c, 0xff}
}

func mutedText() *canvas.Text {
	t := canvas.NewText("", theme.Color(theme.ColorNamePlaceHolder))
	t.TextSize = theme.TextSize() - 1
	return t
}

// --- Rows and actions ------------------------------------------------------

// tapBox makes its content react to taps and right clicks; list rows and
// tiles use it so that a right click anywhere opens the entry's menu.
type tapBox struct {
	widget.BaseWidget
	content fyne.CanvasObject
	onTap   func(fyne.Position) // absolute position
	onMenu  func(fyne.Position)
}

func newTapBox(content fyne.CanvasObject) *tapBox {
	t := &tapBox{content: content}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tapBox) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(t.content) }

func (t *tapBox) Tapped(e *fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap(e.AbsolutePosition)
	}
}

func (t *tapBox) TappedSecondary(e *fyne.PointEvent) {
	if t.onMenu != nil {
		t.onMenu(e.AbsolutePosition)
	}
}

// menu is the entry's context menu: open (folders), download, delete.
func (h viewHost) menu(e rEntry) *fyne.Menu {
	guard := func(f func(rEntry)) func() {
		return func() {
			if !h.busy() {
				f(e)
			}
		}
	}
	item := func(key string, icon fyne.Resource, f func(rEntry)) *fyne.MenuItem {
		it := fyne.NewMenuItem(i18n.T(key), guard(f))
		it.Icon = icon
		return it
	}
	var items []*fyne.MenuItem
	if e.Dir {
		items = append(items, item("mtp.open", theme.FolderOpenIcon(), h.enter))
	}
	items = append(items,
		item("mtp.download", theme.DownloadIcon(), h.download),
		item("mtp.delete", theme.DeleteIcon(), h.delete))
	return fyne.NewMenu("", items...)
}

func (h viewHost) showMenu(e rEntry, pos fyne.Position, on fyne.CanvasObject) {
	if c := fyne.CurrentApp().Driver().CanvasForObject(on); c != nil {
		widget.ShowPopUpMenuAtPosition(h.menu(e), c, pos)
	}
}

// rowParts is a list row: its root object and how to show an entry in it.
type rowParts struct {
	root   fyne.CanvasObject
	update func(h viewHost, e rEntry, odd bool)
}

// actions are the download and delete buttons at the end of a row, and the
// chevron of folders.
type actions struct {
	size     *widget.Label
	dl, del  *widget.Button
	chevron  *widget.Icon
	box      *fyne.Container
	withSize bool
}

func newActions(withSize, withChevron bool) *actions {
	a := &actions{withSize: withSize}
	a.size = widget.NewLabel(humanBytes(1023 << 30)) // sets the min width
	a.size.Alignment = fyne.TextAlignTrailing
	a.dl = widget.NewButtonWithIcon("", theme.DownloadIcon(), nil)
	a.del = widget.NewButtonWithIcon("", theme.DeleteIcon(), nil)
	a.chevron = widget.NewIcon(theme.NavigateNextIcon())
	a.box = container.NewHBox(a.size, a.dl, a.del, a.chevron)
	if !withSize {
		a.size.Hide()
	}
	if !withChevron {
		a.chevron = nil
		a.box.Objects = a.box.Objects[:3]
	}
	return a
}

func (a *actions) update(h viewHost, e rEntry) {
	a.del.OnTapped = func() { h.delete(e) }
	a.dl.OnTapped = func() { h.download(e) } // a folder is downloaded with its contents
	if e.Dir {
		a.size.SetText("")
	} else {
		a.size.SetText(humanBytes(e.Size))
	}
	if a.chevron != nil {
		if e.Dir {
			a.chevron.Show()
		} else {
			a.chevron.Hide()
		}
	}
	if h.busy() {
		a.dl.Disable()
		a.del.Disable()
	} else {
		a.dl.Enable()
		a.del.Enable()
	}
}

func newNameLabel() *widget.Label {
	l := widget.NewLabel("")
	l.Truncation = fyne.TextTruncateEllipsis
	return l
}

// newIconRow: an icon, the name, size and buttons. colored adds coloured
// icons, bold folder names and a chevron.
func newIconRow(colored bool) rowParts {
	icon := widget.NewIcon(theme.FileIcon())
	name := newNameLabel()
	a := newActions(true, colored)
	return rowParts{
		root: container.NewBorder(nil, nil, icon, a.box, name),
		update: func(h viewHost, e rEntry, _ bool) {
			name.SetText(e.Name)
			if colored {
				icon.SetResource(coloredIcon(e))
				name.TextStyle.Bold = e.Dir
				name.Refresh()
			} else {
				icon.SetResource(plainIcon(e))
			}
			a.update(h, e)
		},
	}
}

// newBadgeRow: large folders, a coloured format badge for files.
func newBadgeRow() rowParts {
	bg := canvas.NewRectangle(color.Black)
	bg.CornerRadius = 4
	label := canvas.NewText("", color.White)
	label.TextStyle.Bold = true
	label.TextSize = 11
	label.Alignment = fyne.TextAlignCenter
	badge := container.NewCenter(container.NewGridWrap(fyne.NewSize(42, 20), container.NewStack(bg, container.NewCenter(label))))
	folder := widget.NewIcon(theme.NewColoredResource(theme.FolderIcon(), theme.ColorNameWarning))
	left := container.NewGridWrap(fyne.NewSize(42, 36), container.NewStack(badge, folder))
	name := newNameLabel()
	a := newActions(true, true)
	return rowParts{
		root: container.NewBorder(nil, nil, left, a.box, name),
		update: func(h viewHost, e rEntry, _ bool) {
			name.SetText(e.Name)
			if e.Dir {
				badge.Hide()
				folder.Show()
			} else {
				x := ext(e.Name)
				if len(x) > 4 {
					x = x[:4]
				}
				label.Text = x
				bg.FillColor = badgeColor(x)
				label.Refresh()
				bg.Refresh()
				badge.Show()
				folder.Hide()
			}
			a.update(h, e)
		},
	}
}

// newTwoLineRow: a large icon, the name, and "size · type" under it.
func newTwoLineRow() rowParts {
	icon := canvas.NewImageFromResource(theme.FileIcon())
	icon.FillMode = canvas.ImageFillContain
	name := newNameLabel()
	sub := mutedText()
	text := container.NewVBox(name, container.New(layout.NewCustomPaddedLayout(-14, 0, 8, 0), sub))
	left := container.NewGridWrap(fyne.NewSize(40, 52), container.NewCenter(container.NewGridWrap(fyne.NewSize(32, 32), icon)))
	a := newActions(false, true)
	return rowParts{
		root: container.NewBorder(nil, nil, left, container.NewCenter(a.box), text),
		update: func(h viewHost, e rEntry, _ bool) {
			name.SetText(e.Name)
			icon.Resource = coloredIcon(e)
			icon.Refresh()
			if e.Dir {
				sub.Text = kindName(e)
			} else {
				sub.Text = humanBytes(e.Size) + "  ·  " + kindName(e)
			}
			sub.Color = theme.Color(theme.ColorNamePlaceHolder)
			sub.Refresh()
			a.update(h, e)
		},
	}
}

// listView puts rows made by newRow into a list. Tapping a folder opens
// it; a right click opens the entry's menu.
func listView(h viewHost, newRow func() rowParts) fileView {
	rows := map[*tapBox]rowParts{}
	var list *widget.List
	list = widget.NewList(
		func() int { return len(h.items()) },
		func() fyne.CanvasObject {
			r := newRow()
			box := newTapBox(r.root)
			rows[box] = r
			return box
		},
		func(id widget.ListItemID, o fyne.CanvasObject) {
			items := h.items()
			if id >= len(items) {
				return
			}
			e, box := items[id], o.(*tapBox)
			rows[box].update(h, e, id%2 == 1)
			box.onTap = func(fyne.Position) {
				if e.Dir && !h.busy() {
					h.enter(e)
				}
			}
			box.onMenu = func(pos fyne.Position) { h.showMenu(e, pos, list) }
		},
	)
	list.OnSelected = func(id widget.ListItemID) { // keyboard
		list.UnselectAll()
		if items := h.items(); id < len(items) && items[id].Dir && !h.busy() {
			h.enter(items[id])
		}
	}
	return fileView{obj: list, refresh: list.Refresh}
}

// tableView: column headers, zebra rows, one "..." menu per row.
func tableView(h viewHost) fileView {
	const typeW, sizeW = 130, 90
	col := func(w float32, o fyne.CanvasObject) fyne.CanvasObject {
		return container.NewGridWrap(fyne.NewSize(w, 36), o)
	}
	head := func(key string, align fyne.TextAlign) *widget.Label {
		return widget.NewLabelWithStyle(i18n.T(key), align, fyne.TextStyle{Bold: true})
	}
	moreW := widget.NewButtonWithIcon("", theme.MoreHorizontalIcon(), nil).MinSize().Width
	header := container.NewBorder(nil, nil, nil,
		container.NewHBox(col(typeW, head("column.type", fyne.TextAlignLeading)), col(sizeW, head("column.size", fyne.TextAlignTrailing)), col(moreW, layout.NewSpacer())),
		container.NewHBox(widget.NewIcon(nil), head("column.name", fyne.TextAlignLeading)))

	lv := listView(h, func() rowParts {
		bg := canvas.NewRectangle(color.Transparent)
		icon := widget.NewIcon(nil)
		name := newNameLabel()
		typ := newNameLabel()
		size := widget.NewLabel("")
		size.Alignment = fyne.TextAlignTrailing
		more := widget.NewButtonWithIcon("", theme.MoreHorizontalIcon(), nil)
		more.Importance = widget.LowImportance
		row := container.NewBorder(nil, nil, icon, container.NewHBox(col(typeW, typ), col(sizeW, size), more), name)
		return rowParts{
			root: container.NewStack(bg, row),
			update: func(h viewHost, e rEntry, odd bool) {
				if odd {
					bg.FillColor = theme.Color(theme.ColorNameHover)
				} else {
					bg.FillColor = color.Transparent
				}
				bg.Refresh()
				icon.SetResource(coloredIcon(e))
				name.SetText(e.Name)
				typ.SetText(kindName(e))
				if e.Dir {
					size.SetText("—")
				} else {
					size.SetText(humanBytes(e.Size))
				}
				more.OnTapped = func() {
					if c := fyne.CurrentApp().Driver().CanvasForObject(more); c != nil {
						widget.ShowPopUpMenuAtRelativePosition(h.menu(e), c, fyne.NewPos(0, more.Size().Height), more)
					}
				}
				if h.busy() {
					more.Disable()
				} else {
					more.Enable()
				}
			},
		}
	})
	return fileView{obj: container.NewBorder(container.NewVBox(header, widget.NewSeparator()), nil, nil, nil, lv.obj), refresh: lv.refresh}
}

// tilesView: a grid of large icons. Tapping a folder opens it, tapping a
// file (or right-clicking anything) opens its menu.
func tilesView(h viewHost) fileView {
	type tile struct {
		icon *canvas.Image
		name *widget.Label
		sub  *canvas.Text
	}
	tiles := map[*tapBox]tile{}
	var grid *widget.GridWrap
	grid = widget.NewGridWrap(
		func() int { return len(h.items()) },
		func() fyne.CanvasObject {
			t := tile{icon: canvas.NewImageFromResource(theme.FileIcon()), name: newNameLabel(), sub: mutedText()}
			t.icon.FillMode = canvas.ImageFillContain
			t.icon.SetMinSize(fyne.NewSize(48, 48))
			t.name.Alignment = fyne.TextAlignCenter
			t.sub.Alignment = fyne.TextAlignCenter
			box := newTapBox(container.NewGridWrap(fyne.NewSize(132, 116),
				container.NewVBox(container.NewCenter(t.icon), t.name, container.NewCenter(t.sub))))
			tiles[box] = t
			return box
		},
		func(id widget.GridWrapItemID, o fyne.CanvasObject) {
			items := h.items()
			if id >= len(items) {
				return
			}
			e, box := items[id], o.(*tapBox)
			t := tiles[box]
			t.icon.Resource = coloredIcon(e)
			t.icon.Refresh()
			t.name.SetText(e.Name)
			if e.Dir {
				t.sub.Text = kindName(e)
			} else {
				t.sub.Text = humanBytes(e.Size)
			}
			t.sub.Refresh()
			box.onTap = func(pos fyne.Position) {
				if e.Dir {
					if !h.busy() {
						h.enter(e)
					}
					return
				}
				h.showMenu(e, pos, grid)
			}
			box.onMenu = func(pos fyne.Position) { h.showMenu(e, pos, grid) }
		},
	)
	grid.OnSelected = func(id widget.GridWrapItemID) { // keyboard
		grid.UnselectAll()
		if items := h.items(); id < len(items) && items[id].Dir && !h.busy() {
			h.enter(items[id])
		}
	}
	return fileView{obj: grid, refresh: grid.Refresh}
}
