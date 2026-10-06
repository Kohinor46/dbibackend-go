package gui

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// canRelease: on macOS the camera service (ptpcamerad) and Android File
// Transfer grab MTP devices as soon as they appear.
const canRelease = true

// stoppable programs are quit by "Free the device", with the signal to use.
// The background services get SIGKILL: ptpcamerad ignores SIGTERM while it
// holds a device (seen on macOS 27: same pid owned it after 45 SIGTERMs).
// launchd starts it again on demand.
var stoppable = []struct{ name, signal string }{
	{"ptpcamerad", "KILL"},
	{"Android File Transfer Agent", "KILL"},
	{"Android File Transfer", "TERM"},
}

// suspects may hold an MTP device; they are only reported in the log.
var suspects = []string{"ptpcamerad", "Android File Transfer Agent", "Android File Transfer",
	"OpenMTP", "MacDroid", "Commander One", "Image Capture", "Захват изображений"}

// runningHolders lists the running programs that may hold the device.
func runningHolders() []string {
	out, err := exec.Command("/bin/ps", "-axo", "comm=").Output()
	if err != nil {
		return nil
	}
	running := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		running[strings.ToLower(filepath.Base(strings.TrimSpace(line)))] = true
	}
	var found []string
	for _, name := range suspects {
		for proc := range running {
			if strings.HasPrefix(proc, strings.ToLower(name)) {
				found = append(found, name)
				break
			}
		}
	}
	return found
}

// releaseDevice quits the stoppable programs that are running and returns
// their names. Tests replace it.
var releaseDevice = func() ([]string, error) {
	var stopped []string
	var errs []error
	for _, p := range stoppable {
		if exec.Command("/usr/bin/pgrep", "-x", p.name).Run() != nil {
			continue // not running
		}
		if err := exec.Command("/usr/bin/killall", "-"+p.signal, p.name).Run(); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", p.name, err))
			continue
		}
		stopped = append(stopped, p.name)
	}
	return stopped, errors.Join(errs...)
}

// deviceOwner reports which process holds the Switch's MTP interface, as
// IOKit records it ("UsbExclusiveOwner"), for the log.
func deviceOwner() string {
	out, err := exec.Command("/usr/sbin/ioreg", "-r", "-c", "IOUSBHostInterface", "-l", "-w0").Output()
	if err != nil {
		return "?"
	}
	for _, block := range strings.Split(string(out), "+-o ") {
		if !strings.Contains(block, `"idVendor" = 1406`) || !strings.Contains(block, `"idProduct" = 8221`) {
			continue // not 057E:201D
		}
		for _, line := range strings.Split(block, "\n") {
			if k := strings.Index(line, `"UsbExclusiveOwner" = `); k >= 0 {
				return strings.Trim(strings.TrimSpace(line[k+len(`"UsbExclusiveOwner" = `):]), `"`)
			}
		}
		return "none recorded"
	}
	return "interface not found"
}
