package gui

import "testing"

// ioreg is queried for real; without a Switch in MTP mode the interface
// isn't there.
func TestDeviceOwnerWithoutSwitch(t *testing.T) {
	if got := deviceOwner(); got != "interface not found" {
		t.Logf("deviceOwner() = %q (is a Switch in MTP mode connected?)", got)
	}
	_ = runningHolders()
}
