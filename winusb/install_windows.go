package winusb

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// Exit codes of the elevated "-install-winusb" process.
const (
	exitOK           = 0
	exitFailed       = 1
	exitNotConnected = 3
	exitReboot       = 4
)

// Install shows a UAC prompt and assigns WinUSB to every connected Switch by
// running this executable elevated with "-install-winusb <log>" (see
// RunElevated). Cancelling ctx stops waiting for it.
func Install(ctx context.Context) (Result, error) {
	exe, err := os.Executable()
	if err != nil {
		return Result{}, err
	}
	// The elevated process creates the log itself (and refuses an existing
	// file), so nothing can be planted at this path in advance.
	var rnd [8]byte
	rand.Read(rnd[:])
	logPath := filepath.Join(os.TempDir(), "dbi-winusb-"+hex.EncodeToString(rnd[:])+".log")
	defer os.Remove(logPath)

	code, runErr := runAs(ctx, exe, `-install-winusb "`+logPath+`"`)
	logText, _ := os.ReadFile(logPath)
	res := Result{Log: strings.TrimSpace(string(logText))}
	switch {
	case runErr != nil:
		return res, runErr
	case code == exitOK:
		return res, nil
	case code == exitReboot:
		res.Reboot = true
		return res, nil
	case code == exitNotConnected:
		return res, ErrNotConnected
	default:
		return res, fmt.Errorf("driver installer exited with code %d", code)
	}
}

// RunElevated is the body of the elevated "-install-winusb <log>" process.
// It returns the process exit code.
func RunElevated(logPath string) int {
	f, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return exitFailed
	}
	defer f.Close()
	if needsHelper() {
		// Device installation from an emulated x64 process fails with
		// ERROR_IN_WOW64, so the native ARM64 helper does it.
		return runHelper(f)
	}
	return RunInstall(f)
}

// RunInstall assigns WinUSB in this (elevated, native) process and logs to w.
// It returns the process exit code.
func RunInstall(w io.Writer) int {
	logf := func(format string, a ...any) { fmt.Fprintf(w, format+"\n", a...) }
	reboot, err := installForConnectedDevices(logf)
	switch {
	case errors.Is(err, ErrNotConnected):
		logf("%v", err)
		return exitNotConnected
	case err != nil:
		logf("ERROR: %v", err)
		return exitFailed
	case reboot:
		logf("Windows asks for a restart to finish the installation")
		return exitReboot
	}
	return exitOK
}

// installForConnectedDevices assigns the inbox WinUSB driver to every present
// device with HardwareID. Requires administrator rights.
func installForConnectedDevices(logf func(string, ...any)) (reboot bool, err error) {
	devs, err := windows.SetupDiGetClassDevsEx(nil, "", 0, windows.DIGCF_ALLCLASSES|windows.DIGCF_PRESENT, 0, "")
	if err != nil {
		return false, fmt.Errorf("list devices: %w", err)
	}
	defer devs.Close()

	found := 0
	for i := 0; ; i++ {
		data, err := devs.EnumDeviceInfo(i)
		if errors.Is(err, windows.ERROR_NO_MORE_ITEMS) {
			break
		}
		if err != nil || !hasHardwareID(devs, data) {
			continue
		}
		found++
		id, _ := devs.DeviceInstanceID(data)
		if svc, _ := devs.DeviceRegistryProperty(data, windows.SPDRP_SERVICE); strings.EqualFold(fmt.Sprint(svc), "WinUSB") {
			logf("%s: WinUSB is already installed", id)
			continue
		}
		r, err := installWinUSB(devs, data)
		if err != nil {
			return false, fmt.Errorf("%s: %w", id, err)
		}
		logf("%s: WinUSB installed", id)
		reboot = reboot || r
	}
	if found == 0 {
		return false, ErrNotConnected
	}
	return reboot, nil
}

func hasHardwareID(devs windows.DevInfo, data *windows.DevInfoData) bool {
	v, err := devs.DeviceRegistryProperty(data, windows.SPDRP_HARDWAREID)
	if err != nil {
		return false
	}
	ids, _ := v.([]string)
	for _, id := range ids {
		// e.g. USB\VID_057E&PID_3000&REV_0100 and USB\VID_057E&PID_3000
		if strings.EqualFold(id, HardwareID) {
			return true
		}
	}
	return false
}

// installWinUSB selects the WinUSB driver from %WINDIR%\INF\winusb.inf for the
// device and installs it, like picking "WinUsb Device" in Device Manager.
func installWinUSB(devs windows.DevInfo, data *windows.DevInfoData) (reboot bool, err error) {
	inf := filepath.Join(os.Getenv("WINDIR"), "INF", "winusb.inf")
	params, err := devs.DeviceInstallParams(data)
	if err != nil {
		return false, err
	}
	if err := params.SetDriverPath(inf); err != nil {
		return false, err
	}
	params.Flags |= windows.DI_ENUMSINGLEINF | windows.DI_QUIETINSTALL
	params.FlagsEx |= windows.DI_FLAGSEX_ALLOWEXCLUDEDDRVS
	if err := devs.SetDeviceInstallParams(data, params); err != nil {
		return false, fmt.Errorf("set install params: %w", err)
	}

	if err := devs.BuildDriverInfoList(data, windows.SPDIT_CLASSDRIVER); err != nil {
		return false, fmt.Errorf("read %s: %w", inf, err)
	}
	defer devs.DestroyDriverInfoList(data, windows.SPDIT_CLASSDRIVER)

	var pick *windows.DrvInfoData
	for j := 0; ; j++ {
		drv, err := devs.EnumDriverInfo(data, windows.SPDIT_CLASSDRIVER, j)
		if err != nil {
			break
		}
		detail, err := devs.DriverInfoDetail(data, drv)
		if err == nil && strings.EqualFold(filepath.Base(detail.InfFileName()), "winusb.inf") {
			pick = drv
			break
		}
	}
	if pick == nil {
		return false, fmt.Errorf("no WinUSB driver found in %s", inf)
	}
	if err := devs.SetSelectedDriver(data, pick); err != nil {
		return false, fmt.Errorf("select driver: %w", err)
	}
	if err := devs.CallClassInstaller(windows.DIF_INSTALLDEVICE, data); err != nil {
		return false, fmt.Errorf("install driver: %w", err)
	}
	if p, err := devs.DeviceInstallParams(data); err == nil {
		reboot = p.Flags&(windows.DI_NEEDREBOOT|windows.DI_NEEDRESTART) != 0
	}
	return reboot, nil
}
