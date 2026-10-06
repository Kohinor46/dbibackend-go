//go:build windows && !amd64

package winusb

import "io"

func needsHelper() bool { return false }

func runHelper(w io.Writer) int { return RunInstall(w) }
