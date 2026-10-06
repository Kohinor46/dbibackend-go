package winusb

import (
	"context"
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var procShellExecuteExW = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteExW")

// shellExecuteInfo is SHELLEXECUTEINFOW.
type shellExecuteInfo struct {
	cbSize       uint32
	fMask        uint32
	hwnd         windows.HWND
	lpVerb       *uint16
	lpFile       *uint16
	lpParameters *uint16
	lpDirectory  *uint16
	nShow        int32
	hInstApp     windows.Handle
	lpIDList     uintptr
	lpClass      *uint16
	hkeyClass    windows.Handle
	dwHotKey     uint32
	hIcon        windows.Handle
	hProcess     windows.Handle
}

const (
	seeMaskNoCloseProcess = 0x00000040
	seeMaskNoAsync        = 0x00000100
	seeMaskFlagNoUI       = 0x00000400
)

// runAs starts exe with args through the UAC prompt and waits for it.
// Declining the prompt returns ErrCanceled; any other start failure is
// returned as is. Cancelling ctx stops waiting (and tries to end the process).
func runAs(ctx context.Context, exe, args string) (exitCode uint32, err error) {
	verb, _ := windows.UTF16PtrFromString("runas")
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return 0, err
	}
	params, err := windows.UTF16PtrFromString(args)
	if err != nil {
		return 0, err
	}
	info := shellExecuteInfo{
		fMask:        seeMaskNoCloseProcess | seeMaskNoAsync | seeMaskFlagNoUI,
		lpVerb:       verb,
		lpFile:       file,
		lpParameters: params,
		nShow:        windows.SW_HIDE,
	}
	info.cbSize = uint32(unsafe.Sizeof(info))
	if ok, _, callErr := procShellExecuteExW.Call(uintptr(unsafe.Pointer(&info))); ok == 0 {
		if errors.Is(callErr, windows.ERROR_CANCELLED) {
			return 0, ErrCanceled
		}
		return 0, fmt.Errorf("start elevated installer: %w", callErr)
	}
	if info.hProcess == 0 {
		return 0, errors.New("start elevated installer: no process handle")
	}
	defer windows.CloseHandle(info.hProcess)

	done := make(chan struct{})
	go func() {
		windows.WaitForSingleObject(info.hProcess, windows.INFINITE)
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		windows.TerminateProcess(info.hProcess, 1)
		return 0, ctx.Err()
	}
	if err := windows.GetExitCodeProcess(info.hProcess, &exitCode); err != nil {
		return 0, err
	}
	return exitCode, nil
}
