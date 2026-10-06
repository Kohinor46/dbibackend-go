//go:build !windows

package gui

import (
	"fyne.io/fyne/v2"
	"github.com/ncruces/zenity"
)

// parentWindow: on macOS attaching would need Automation permission, so the
// panel is shown standalone; on Linux Fyne doesn't expose the X11 window id.
func parentWindow(fyne.Window) []zenity.Option { return nil }
