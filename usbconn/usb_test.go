package usbconn

import (
	"errors"
	"fmt"
	"runtime"
	"testing"

	"github.com/google/gousb"
)

func ep(addr int, tt gousb.TransferType, dir gousb.EndpointDirection) gousb.EndpointDesc {
	return gousb.EndpointDesc{Address: gousb.EndpointAddress(addr), Number: addr & 0x0f, TransferType: tt, Direction: dir, MaxPacketSize: 512}
}

func device(class gousb.Class, sub gousb.Class, proto gousb.Protocol, interrupt bool) *gousb.DeviceDesc {
	eps := map[gousb.EndpointAddress]gousb.EndpointDesc{
		0x81: ep(0x81, gousb.TransferTypeBulk, gousb.EndpointDirectionIn),
		0x01: ep(0x01, gousb.TransferTypeBulk, gousb.EndpointDirectionOut),
	}
	if interrupt {
		eps[0x82] = ep(0x82, gousb.TransferTypeInterrupt, gousb.EndpointDirectionIn)
	}
	alt := gousb.InterfaceSetting{Number: 0, Alternate: 0, Class: class, SubClass: sub, Protocol: proto, Endpoints: eps}
	return &gousb.DeviceDesc{
		Vendor: 0x057E, Product: 0x201D,
		Configs: map[int]gousb.ConfigDesc{1: {Number: 1, Interfaces: []gousb.InterfaceDesc{{Number: 0, AltSettings: []gousb.InterfaceSetting{alt}}}}},
	}
}

func TestFindMTP(t *testing.T) {
	for _, tc := range []struct {
		name string
		desc *gousb.DeviceDesc
		want bool
	}{
		{"still image class (MTP/PTP)", device(gousb.ClassPTP, 1, 1, true), true},
		{"vendor-specific with interrupt", device(gousb.ClassVendorSpec, 0xFF, 0, true), true},
		{"DBI install mode (vendor bulk only)", device(gousb.ClassVendorSpec, 0xFF, 0, false), false},
		{"mass storage", device(gousb.ClassMassStorage, 6, 0x50, false), false},
	} {
		if _, ok := findMTP(tc.desc); ok != tc.want {
			t.Errorf("%s: found = %v, want %v", tc.name, ok, tc.want)
		}
	}
}

// gousb formats the claim error with %v, so only its text says "bad access".
func TestBusy(t *testing.T) {
	claim := fmt.Errorf("failed to claim interface 0 on vid=057e,pid=201d,bus=0,addr=1,config=1: %v", gousb.ErrorAccess)
	if got, want := errors.Is(busy(claim), ErrBusy), runtime.GOOS == "darwin"; got != want {
		t.Errorf("claim ACCESS -> busy = %v, want %v (on %s)", got, want, runtime.GOOS)
	}
	if !errors.Is(busy(fmt.Errorf("claim: %v", gousb.ErrorBusy)), ErrBusy) {
		t.Error("BUSY not recognised")
	}
	if errors.Is(busy(errors.New("timeout")), ErrBusy) {
		t.Error("unrelated error marked busy")
	}
}
