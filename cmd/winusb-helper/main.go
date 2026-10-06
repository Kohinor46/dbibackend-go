// winusb-helper is the native ARM64 Windows build of the driver installer.
// The x64 app embeds it and runs it from its elevated process on ARM64
// Windows (see winusb/helper_windows_amd64.go); output goes to stdout.
package main

import (
	"os"

	"github.com/Kohinor46/dbibackend-go/winusb"
)

func main() {
	os.Exit(winusb.RunInstall(os.Stdout))
}
