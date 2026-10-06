package winusb

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
)

// On ARM64 Windows this x64 program runs under emulation, where device
// installation fails with ERROR_IN_WOW64, so the elevated process runs a
// native ARM64 build of cmd/winusb-helper instead. scripts/release.sh puts it
// into helper/; the directory also holds a README so the embed pattern
// matches (and the app builds) without it.
//
//go:embed helper
var helperFS embed.FS

const helperName = "helper/winusb-helper-arm64.exe"

func needsHelper() bool {
	var process, native uint16
	err := windows.IsWow64Process2(windows.CurrentProcess(), &process, &native)
	return err == nil && native == 0xAA64 // IMAGE_FILE_MACHINE_ARM64
}

// runHelper runs the embedded ARM64 helper from the elevated process and
// copies its output to w. The helper is written to a directory this elevated
// process creates; Windows labels it High integrity, so the user's normal
// (Medium integrity) processes can't replace the file before it runs.
func runHelper(w io.Writer) int {
	bin, err := helperFS.ReadFile(helperName)
	if err != nil {
		fmt.Fprintln(w, "ERROR: this build has no ARM64 driver helper (build with scripts/release.sh)")
		return exitFailed
	}
	dir, err := os.MkdirTemp("", "dbi-winusb-")
	if err != nil {
		fmt.Fprintln(w, "ERROR:", err)
		return exitFailed
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, filepath.Base(helperName))
	if err := os.WriteFile(path, bin, 0o755); err != nil {
		fmt.Fprintln(w, "ERROR:", err)
		return exitFailed
	}
	cmd := exec.Command(path)
	cmd.Stdout, cmd.Stderr = w, w
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	err = cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return exitOK
	case errors.As(err, &exitErr):
		return exitErr.ExitCode()
	default:
		fmt.Fprintln(w, "ERROR: run ARM64 helper:", err)
		return exitFailed
	}
}
