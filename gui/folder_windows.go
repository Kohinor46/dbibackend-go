package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
	"github.com/ncruces/zenity"
)

// parentWindow makes the Explorer dialog modal to the app window.
func parentWindow(w fyne.Window) []zenity.Option {
	var opts []zenity.Option
	if nw, ok := w.(driver.NativeWindow); ok {
		nw.RunNative(func(ctx any) {
			if c, ok := ctx.(driver.WindowsWindowContext); ok && c.HWND != 0 {
				opts = append(opts, zenity.Attach(c.HWND))
			}
		})
	}
	return opts
}
