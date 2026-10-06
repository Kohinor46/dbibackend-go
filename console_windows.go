package main

import (
	"os"

	"golang.org/x/sys/windows"
)

var procAttachConsole = windows.NewLazySystemDLL("kernel32.dll").NewProc("AttachConsole")

// attachConsole sends output to the terminal the app was started from: the
// release build is a GUI-subsystem exe, which has no console of its own.
// Streams that are already redirected (to a file or pipe) are left alone.
func attachConsole() {
	redirected := func(std uint32) bool {
		h, err := windows.GetStdHandle(std)
		return err == nil && h != 0 && h != windows.InvalidHandle
	}
	outRedirected, errRedirected := redirected(windows.STD_OUTPUT_HANDLE), redirected(windows.STD_ERROR_HANDLE)
	if outRedirected && errRedirected {
		return
	}
	const attachParentProcess = 0xFFFFFFFF // ATTACH_PARENT_PROCESS = (DWORD)-1
	if ok, _, _ := procAttachConsole.Call(attachParentProcess); ok == 0 {
		return
	}
	f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if err != nil {
		return
	}
	if !outRedirected {
		os.Stdout = f
	}
	if !errRedirected {
		os.Stderr = f
	}
}
