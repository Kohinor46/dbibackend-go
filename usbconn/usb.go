// Package usbconn opens the Switch running DBI over libusb.
package usbconn

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/gousb"
)

var ErrNotFound = errors.New("device not found")

// ErrNoDriver means the Switch is connected but libusb can't use it: on
// Windows it has no WinUSB driver yet (see package winusb).
var ErrNoDriver = errors.New("device found, but its USB driver is not supported (WinUSB needed)")

// Device is an open bulk IN/OUT pair on interface 0 of the Switch.
type Device struct {
	ctx  *gousb.Context
	dev  *gousb.Device
	cfg  *gousb.Config
	intf *gousb.Interface
	in   *gousb.InEndpoint
	out  *gousb.OutEndpoint
}

// Open opens the first device with the given VID/PID.
func Open(vid, pid uint16) (*Device, error) {
	d, err := open(vid, pid, true)
	if err != nil && !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrNoDriver) {
		// Some platforms invalidate the handle after a reset; retry without it.
		d, err = open(vid, pid, false)
	}
	return d, err
}

func open(vid, pid uint16, reset bool) (_ *Device, err error) {
	d := &Device{ctx: gousb.NewContext()}
	defer func() {
		if err != nil {
			d.Close()
		}
	}()

	if d.dev, err = openFirst(d.ctx, vid, pid); err != nil {
		return nil, err
	}
	if reset {
		if err := d.dev.Reset(); err != nil {
			return nil, fmt.Errorf("reset: %w", err)
		}
	}

	if d.cfg, err = d.dev.Config(1); err != nil {
		return nil, fmt.Errorf("set configuration: %w", err)
	}
	if d.intf, err = d.cfg.Interface(0, 0); err != nil {
		return nil, fmt.Errorf("claim interface: %w", err)
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

// openFirst opens the first device with vid:pid. Unlike
// gousb.OpenDeviceWithVIDPID it ignores errors from other devices (e.g. an
// unreadable descriptor on some hub), which would otherwise turn "not
// connected" into an open error.
func openFirst(ctx *gousb.Context, vid, pid uint16) (*gousb.Device, error) {
	found := false
	devs, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		if found || desc.Vendor != gousb.ID(vid) || desc.Product != gousb.ID(pid) {
			return false
		}
		found = true
		return true
	})
	switch {
	case len(devs) > 0:
		return devs[0], nil
	case !found:
		return nil, ErrNotFound
	case errors.Is(err, gousb.ErrorNotSupported):
		return nil, ErrNoDriver
	default:
		return nil, fmt.Errorf("open device: %w", err)
	}
}

// Wait polls every second until the device appears or ctx is done.
// onWait is called before each sleep (may be nil).
func Wait(ctx context.Context, vid, pid uint16, onWait func(error)) (*Device, error) {
	for {
		d, err := Open(vid, pid)
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
