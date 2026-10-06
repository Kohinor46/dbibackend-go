package gui

import (
	"errors"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"github.com/ncruces/zenity"

	"github.com/Kohinor46/dbibackend-go/i18n"
)

// chooseDir opens the system folder picker: Finder panel on macOS, Explorer
// dialog on Windows, zenity/kdialog on Linux. Falls back to the Fyne dialog
// if none is available.
func (u *ui) chooseDir() {
	opts := []zenity.Option{
		zenity.Directory(),
		zenity.Title(i18n.T("folder.dialog_title")),
	}
	// zenity rejects a start path that doesn't exist (e.g. a renamed folder
	// or drive), which would look like "no native dialog" below.
	if start := startDir(u.dirEntry.Text); start != "" {
		opts = append(opts, zenity.Filename(start))
	}
	opts = append(opts, parentWindow(u.win)...)

	u.browseBtn.Disable()
	go func() {
		// The native dialog blocks, so it must not run on the UI goroutine.
		dir, err := zenity.SelectFile(opts...)
		fyne.Do(func() {
			u.browseBtn.Enable()
			switch {
			case err == nil:
				u.setDir(dir)
			case errors.Is(err, zenity.ErrCanceled):
			default:
				u.log.Warn("System folder dialog unavailable, using built-in", "err", err)
				u.chooseDirFyne()
			}
		})
	}()
}

// startDir returns the folder a picker should open in: p itself, or its
// nearest existing parent if p was renamed or removed; "" if p is empty,
// relative or nothing on its path exists.
func startDir(p string) string {
	p = strings.TrimSpace(p)
	if !filepath.IsAbs(p) {
		// Relative paths would resolve against the app's working directory
		// ("/" for a macOS app), not anything the user meant.
		return ""
	}
	p = filepath.Clean(p)
	for {
		if isDir(p) {
			return p
		}
		parent := filepath.Dir(p)
		if parent == p {
			return ""
		}
		p = parent
	}
}
