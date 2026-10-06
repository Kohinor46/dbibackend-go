package gui

import (
	"errors"

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
		zenity.Filename(u.dirEntry.Text),
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
