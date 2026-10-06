//go:build !windows

package winusb

import (
	"context"
	"errors"
	"io"
)

var errNotWindows = errors.New("driver installation is only needed on Windows")

// Install is only needed on Windows.
func Install(context.Context) (Result, error) { return Result{}, errNotWindows }

// RunElevated is only used on Windows.
func RunElevated(string) int { return 1 }

// RunInstall is only used on Windows.
func RunInstall(io.Writer) int { return 1 }
