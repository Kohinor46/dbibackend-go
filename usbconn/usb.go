// Package usbconn opens the Switch running DBI over libusb.
package usbconn

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/google/gousb"
)

var ErrNotFound = errors.New("device not found")

// ErrNoDriver means the Switch is connected but libusb can't use it: on
// Windows it has no WinUSB driver yet (see package winusb).
var ErrNoDriver = errors.New("device found, but its USB driver is not supported (WinUSB needed)")

// ErrBusy means another program holds the device. On macOS that is usually
// the system camera service (ptpcamerad) or Android File Transfer, which
// grab MTP devices as soon as they are connected.
var ErrBusy = errors.New("device is in use by another program")

// Device is an open bulk IN/OUT pair on one interface of the Switch.
type Device struct {
	ctx       *gousb.Context
	dev       *gousb.Device
	cfg       *gousb.Config
	intf      *gousb.Interface
	in        *gousb.InEndpoint
	out       *gousb.OutEndpoint
	maxPacket int
}

// target is the interface to claim on a matched device.
type target struct{ config, intf, alt int }

// selector picks the device and its interface; ok is false for other devices.
type selector func(desc *gousb.DeviceDesc) (t target, ok bool)

// Open opens the first device with the given VID/PID on interface 0
// (DBI's "Install title from USB" mode), resetting it first.
func Open(vid, pid uint16) (*Device, error) {
	sel := func(desc *gousb.DeviceDesc) (target, bool) {
		return target{1, 0, 0}, desc.Vendor == gousb.ID(vid) && desc.Product == gousb.ID(pid)
	}
	d, err := open(sel, true)
	if err != nil && !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrNoDriver) && !errors.Is(err, ErrBusy) {
		// Some platforms invalidate the handle after a reset; retry without it.
		d, err = open(sel, false)
	}
	return d, err
}

// OpenMTP opens the MTP interface of a device from vid (DBI's "MTP
// responder" mode), skipping the given product IDs.
func OpenMTP(vid uint16, skipPIDs ...uint16) (*Device, error) {
	return open(func(desc *gousb.DeviceDesc) (target, bool) {
		if desc.Vendor != gousb.ID(vid) || slices.Contains(skipPIDs, uint16(desc.Product)) {
			return target{}, false
		}
		return findMTP(desc)
	}, false)
}

// findMTP finds an MTP interface: the standard still-image class
// (6/1/1), or a vendor-specific one with bulk IN/OUT and an interrupt IN
// endpoint, which is how MTP looks when it isn't declared as a camera.
func findMTP(desc *gousb.DeviceDesc) (target, bool) {
	cfgs := slices.Sorted(maps.Keys(desc.Configs))
	for _, n := range cfgs {
		for _, intf := range desc.Configs[n].Interfaces {
			for _, alt := range intf.AltSettings {
				if isMTP(alt) {
					return target{n, intf.Number, alt.Alternate}, true
				}
			}
		}
	}
	return target{}, false
}

func isMTP(s gousb.InterfaceSetting) bool {
	var bulkIn, bulkOut, intrIn bool
	for _, ep := range s.Endpoints {
		switch {
		case ep.TransferType == gousb.TransferTypeBulk && ep.Direction == gousb.EndpointDirectionIn:
			bulkIn = true
		case ep.TransferType == gousb.TransferTypeBulk && ep.Direction == gousb.EndpointDirectionOut:
			bulkOut = true
		case ep.TransferType == gousb.TransferTypeInterrupt && ep.Direction == gousb.EndpointDirectionIn:
			intrIn = true
		}
	}
	if !bulkIn || !bulkOut {
		return false
	}
	if s.Class == gousb.ClassPTP && s.SubClass == 1 && s.Protocol == 1 {
		return true
	}
	return s.Class == gousb.ClassVendorSpec && intrIn
}

func open(sel selector, reset bool) (_ *Device, err error) {
	d := &Device{ctx: gousb.NewContext()}
	defer func() {
		if err != nil {
			d.Close()
		}
	}()

	var t target
	if d.dev, t, err = openFirst(d.ctx, sel); err != nil {
		return nil, err
	}
	if reset {
		if err := d.dev.Reset(); err != nil {
			return nil, fmt.Errorf("reset: %w", err)
		}
	}

	if d.cfg, err = d.dev.Config(t.config); err != nil {
		return nil, fmt.Errorf("set configuration: %w", busy(err))
	}
	if d.intf, err = d.cfg.Interface(t.intf, t.alt); err != nil {
		return nil, fmt.Errorf("claim interface: %w", busy(err))
	}
	for _, ep := range d.intf.Setting.Endpoints {
		if ep.TransferType != gousb.TransferTypeBulk {
			continue
		}
		switch {
		case ep.Direction == gousb.EndpointDirectionIn && d.in == nil:
			d.in, err = d.intf.InEndpoint(ep.Number)
		case ep.Direction == gousb.EndpointDirectionOut && d.out == nil:
			d.out, err = d.intf.OutEndpoint(ep.Number)
			d.maxPacket = ep.MaxPacketSize
		}
		if err != nil {
			return nil, err
		}
	}
	if d.in == nil {
		return nil, fmt.Errorf("input endpoint not found")
	}
	if d.out == nil {
		return nil, fmt.Errorf("output endpoint not found")
	}
	return d, nil
}

// busy marks "another program has the device" errors with ErrBusy. On macOS
// an exclusive claim by another process reports ACCESS; elsewhere ACCESS
// means missing permissions (e.g. no udev rule on Linux), so it stays as is.
func busy(err error) error {
	if isUSBError(err, gousb.ErrorBusy) || (runtime.GOOS == "darwin" && isUSBError(err, gousb.ErrorAccess)) {
		return fmt.Errorf("%w (%v)", ErrBusy, err)
	}
	return err
}

// isUSBError matches a libusb error even when gousb only formatted it into
// the message (claiming an interface uses %v: "failed to claim interface 0
// on ...: libusb: bad access [code -3]").
func isUSBError(err error, target gousb.Error) bool {
	return errors.Is(err, target) || strings.Contains(err.Error(), target.Error())
}

// openFirst opens the first device sel accepts. Unlike
// gousb.OpenDeviceWithVIDPID it ignores errors from other devices (e.g. an
// unreadable descriptor on some hub), which would otherwise turn "not
// connected" into an open error.
func openFirst(ctx *gousb.Context, sel selector) (*gousb.Device, target, error) {
	found := false
	var t target
	devs, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		if found {
			return false
		}
		t, found = sel(desc)
		return found
	})
	switch {
	case len(devs) > 0:
		return devs[0], t, nil
	case !found:
		return nil, t, ErrNotFound
	case errors.Is(err, gousb.ErrorNotSupported):
		return nil, t, ErrNoDriver
	default:
		return nil, t, fmt.Errorf("open device: %w", busy(err))
	}
}

// Wait polls Open every second until the device appears or ctx is done.
// onWait is called before each sleep (may be nil).
func Wait(ctx context.Context, vid, pid uint16, onWait func(error)) (*Device, error) {
	return WaitFor(ctx, func() (*Device, error) { return Open(vid, pid) }, onWait)
}

// WaitFor polls open every second until it succeeds or ctx is done.
func WaitFor(ctx context.Context, open func() (*Device, error), onWait func(error)) (*Device, error) {
	for {
		d, err := open()
		if err == nil {
			return d, nil
		}
		if onWait != nil {
			onWait(err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func (d *Device) Read(ctx context.Context, buf []byte) (int, error) {
	return d.in.ReadContext(ctx, buf)
}

func (d *Device) Write(ctx context.Context, buf []byte) (int, error) {
	return d.out.WriteContext(ctx, buf)
}

// MaxPacketSize is the bulk OUT endpoint's max packet size (mtp.PacketSizer).
func (d *Device) MaxPacketSize() int { return d.maxPacket }

func (d *Device) Close() error {
	if d.intf != nil {
		d.intf.Close()
	}
	if d.cfg != nil {
		d.cfg.Close()
	}
	if d.dev != nil {
		d.dev.Close()
	}
	return d.ctx.Close()
}
