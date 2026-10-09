package gui

import (
	"time"

	"fyne.io/fyne/v2"

	"github.com/Kohinor46/dbibackend-go/i18n"
)

const (
	prefNotify = "notifications"

	// notifyMin: shorter operations finish while the user is still looking.
	// (macOS itself doesn't show banners of the app in front.)
	notifyMin = 10 * time.Second
)

// notifyAfter shows a system notification about an operation that started
// at start, if it took long enough and notifications are on.
func (u *ui) notifyAfter(start time.Time, key string, args ...any) {
	if time.Since(start) < notifyMin || !u.app.Preferences().BoolWithFallback(prefNotify, true) {
		return
	}
	text := i18n.T(key, args...)
	u.log.Debug("Notification", "text", text)
	u.sendNotification(fyne.NewNotification("DBI Backend", text))
}
