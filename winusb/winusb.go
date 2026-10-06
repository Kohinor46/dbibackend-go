// Package winusb assigns Windows' built-in WinUSB driver to the Switch, which
// libusb needs before it can open the device. It does what you'd do by hand in
// Device Manager ("Update driver → Let me pick → WinUsb Device", or Zadig),
// but uses the Microsoft-signed winusb.inf, so no custom certificate is needed
// and it works with Smart App Control enabled.
package winusb

import (
	"errors"
	"runtime"
)

// Supported reports whether driver installation applies to this OS.
const Supported = runtime.GOOS == "windows"

// HardwareID is the Switch running DBI in USB install mode.
const HardwareID = `USB\VID_057E&PID_3000`

// Result of a driver installation.
type Result struct {
	Log    string // installer log, one line per device
	Reboot bool   // Windows asks for a restart to finish
}

var (
	// ErrCanceled is returned when the user declines the UAC prompt.
	ErrCanceled = errors.New("installation canceled by the user")
	// ErrNotConnected: the driver can only be assigned to a connected device.
	ErrNotConnected = errors.New("Switch is not connected (start DBI → Install title from USB)")
)
