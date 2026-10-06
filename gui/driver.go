package gui

import (
	"context"
	"errors"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/Kohinor46/dbibackend-go/i18n"
	"github.com/Kohinor46/dbibackend-go/winusb"
)

const driverInstallTimeout = 5 * time.Minute

// offerDriver asks once per run whether to install WinUSB, after the Switch
// was found without a usable driver (Windows only).
func (u *ui) offerDriver() {
	if !winusb.Supported || u.driverOffered || u.installing {
		return
	}
	u.driverOffered = true
	msg := widget.NewLabel(i18n.T("driver.prompt"))
	msg.Wrapping = fyne.TextWrapWord
	d := dialog.NewCustomConfirm(i18n.T("driver.title"), i18n.T("driver.install"), i18n.T("driver.later"), msg,
		func(ok bool) {
			if ok {
				u.installDriver()
			}
		}, u.win)
	d.Resize(fyne.NewSize(480, 0))
	d.Show()
}

// installDriver runs the elevated WinUSB installer (UAC prompt) in the background.
func (u *ui) installDriver() {
	if u.installing {
		return
	}
	u.installing = true
	if u.driverBtn != nil {
		u.driverBtn.Disable()
	}
	// Cancel (or the timeout) stops waiting for a stuck installer.
	ctx, cancel := context.WithTimeout(context.Background(), driverInstallTimeout)
	wait := dialog.NewCustom(i18n.T("driver.installing"), i18n.T("cancel"), widget.NewProgressBarInfinite(), u.win)
	wait.SetOnClosed(cancel)
	wait.Show()
	go func() {
		res, err := winusb.Install(ctx)
		fyne.Do(func() {
			wait.Hide()
			u.installing = false
			if u.driverBtn != nil {
				u.driverBtn.Enable()
			}
			for _, line := range strings.Split(res.Log, "\n") {
				if line = strings.TrimSpace(line); line != "" {
					u.log.Info("Driver installer: " + line)
				}
			}
			title := i18n.T("driver.title")
			switch {
			case errors.Is(err, winusb.ErrCanceled), errors.Is(err, context.Canceled):
				u.log.Info("Driver installation canceled")
			case errors.Is(err, winusb.ErrNotConnected):
				dialog.ShowInformation(title, i18n.T("driver.not_connected"), u.win)
			case err != nil:
				u.log.Error("Driver installation failed", "err", err)
				dialog.ShowError(errors.New(i18n.T("driver.failed", err)), u.win)
			case res.Reboot:
				dialog.ShowInformation(title, i18n.T("driver.reboot"), u.win)
			default:
				dialog.ShowInformation(title, i18n.T("driver.done"), u.win)
			}
		})
	}()
}
