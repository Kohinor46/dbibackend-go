package mtp

import (
	"bytes"
	"context"
	"errors"
	"hash/crc32"
	"io"
	"strings"
	"testing"
)

func open(t *testing.T, d *fakeDevice) *Client {
	t.Helper()
	c := NewClient(d)
	if err := c.Open(context.Background()); err != nil {
		t.Fatal(err)
	}
	return c
}

func checkFake(t *testing.T, d *fakeDevice) {
	t.Helper()
	for _, e := range d.errors {
		t.Error("device:", e)
	}
	if len(d.out) != 0 {
		t.Errorf("device has %d unread transfers", len(d.out))
	}
}

func TestStringRoundTrip(t *testing.T) {
	for _, s := range []string{"", "a", "Игры", "ゲーム.nsp", "emoji 🎮 (surrogate pair)"} {
		e := &encoder{}
		e.str(s)
		d := &decoder{b: e.b}
		if got := d.str(); got != s || d.err != nil || len(d.b) != 0 {
			t.Errorf("%q -> %q (err %v, %d left)", s, got, d.err, len(d.b))
		}
	}
}

func TestBrowseUploadDownloadDelete(t *testing.T) {
	ctx := context.Background()
	d := newFakeDevice()
	c := open(t, d)
	if got := c.Info().Model; got != "Switch (DBI)" {
		t.Errorf("model = %q", got)
	}

	storages, err := c.Storages(ctx)
	if err != nil || len(storages) != 2 || storages[1].Info.Description != "SD Card install" {
		t.Fatalf("storages = %+v, %v", storages, err)
	}
	sd := storages[0].ID

	folder, err := c.MakeFolder(ctx, sd, ParentRoot, "Игры")
	if err != nil {
		t.Fatal(err)
	}
	payload := bytes.Repeat([]byte("switch"), 50_000) // 300 KB
	h, err := c.Upload(ctx, sd, folder, "game.nsp", uint64(len(payload)), bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}

	root, err := c.List(ctx, sd, ParentRoot)
	if err != nil || len(root) != 1 || !root[0].IsFolder() || root[0].Name() != "Игры" {
		t.Fatalf("root = %+v, %v", root, err)
	}
	inside, err := c.List(ctx, sd, folder)
	if err != nil || len(inside) != 1 || inside[0].Handle != h || inside[0].Size != uint64(len(payload)) {
		t.Fatalf("folder = %+v, %v", inside, err)
	}

	var got bytes.Buffer
	if err := c.Download(ctx, inside[0], &got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), payload) {
		t.Fatal("downloaded data differs")
	}

	if err := c.Delete(ctx, folder); err != nil {
		t.Fatal(err)
	}
	if root, _ := c.List(ctx, sd, ParentRoot); len(root) != 0 {
		t.Errorf("after delete: %+v", root)
	}
	checkFake(t, d)
}

// A data container that is an exact multiple of the packet size must be
// followed by a zero-length packet in both directions.
func TestZeroLengthPackets(t *testing.T) {
	ctx := context.Background()
	d := newFakeDevice()
	c := open(t, d)
	for _, size := range []int{500, 512 - headerSize + 512, 0, 1} { // 12+500 = 512
		data := bytes.Repeat([]byte{7}, size)
		h, err := c.Upload(ctx, 0x00010001, ParentRoot, "f.bin", uint64(size), bytes.NewReader(data))
		if err != nil {
			t.Fatalf("size %d: %v", size, err)
		}
		o, err := c.Object(ctx, h)
		if err != nil {
			t.Fatal(err)
		}
		var got bytes.Buffer
		if err := c.Download(ctx, o, &got); err != nil || !bytes.Equal(got.Bytes(), data) {
			t.Fatalf("size %d: download %v", size, err)
		}
	}
	checkFake(t, d)
}

// Devices may pack the response into the same transfer as the data, or
// mark the data length as unknown.
func TestDeviceFraming(t *testing.T) {
	for _, mode := range []string{"merged", "unknown-length"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			d := newFakeDevice()
			payload := bytes.Repeat([]byte("abc"), 1000)
			d.addObject(0x00010001, ParentRoot, "a.bin", false, uint64(len(payload)), payload)
			d.mergeResponse = mode == "merged"
			d.unknownLenData = mode == "unknown-length"
			// Device info has no announced size, so with unknown lengths
			// Open reads it until the short packet that ends the transfer.
			c := open(t, d)
			list, err := c.List(ctx, 0x00010001, ParentRoot)
			if err != nil || len(list) != 1 {
				t.Fatalf("list = %+v, %v", list, err)
			}
			var got bytes.Buffer
			if err := c.Download(ctx, list[0], &got); err != nil || !bytes.Equal(got.Bytes(), payload) {
				t.Fatalf("download: %v", err)
			}
			checkFake(t, d)
		})
	}
}

// Without SendObjectPropList, objects too big for the 32-bit container
// length are sent with lengthUnknown and end with a short packet.
func TestUploadUnknownLengthWithoutPropList(t *testing.T) {
	defer func(v uint64) { containerLimit = v }(containerLimit)
	containerLimit = 1000
	ctx := context.Background()
	for _, size := range []int{5000, 512*20 - headerSize} { // second one needs a ZLP to end
		d := newFakeDevice()
		d.propList = false
		d.ignoreSize = true
		c := open(t, d)
		data := bytes.Repeat([]byte{1, 2, 3}, size/3+1)[:size]
		h, err := c.Upload(ctx, 0x00010001, ParentRoot, "big.xci", uint64(size), bytes.NewReader(data))
		if err != nil {
			t.Fatalf("size %d: %v", size, err)
		}
		if d.lastSendObjHdr != lengthUnknown {
			t.Errorf("size %d: SendObject header length = %#x, want unknown", size, d.lastSendObjHdr)
		}
		if o := d.objects[h]; o == nil || !bytes.Equal(o.data, data) {
			t.Errorf("size %d: device got different data", size)
		}
		checkFake(t, d)
	}
}

// patternReader yields n deterministic bytes without allocating them.
type patternReader struct{ left uint64 }

func (r *patternReader) Read(p []byte) (int, error) {
	if r.left == 0 {
		return 0, io.EOF
	}
	n := int(min(uint64(len(p)), r.left))
	for i := range p[:n] {
		p[i] = byte(r.left - uint64(i))
	}
	r.left -= uint64(n)
	return n, nil
}

// The real thing: a file over 4 GB is announced with its 64-bit size via
// SendObjectPropList and streamed with an "unknown length" container.
func TestUploadOver4GB(t *testing.T) {
	if testing.Short() {
		t.Skip("streams 4 GB")
	}
	ctx := context.Background()
	d := newFakeDevice()
	d.keepLimit = 0
	c := open(t, d)
	const size = 4<<30 + 1000
	sent := crc32.NewIEEE()
	h, err := c.Upload(ctx, 0x00020001, ParentRoot, "huge.xci", size, io.TeeReader(&patternReader{left: size}, sent))
	if err != nil {
		t.Fatal(err)
	}
	o := d.objects[h]
	switch {
	case d.lastPropListLen != size:
		t.Errorf("prop list size = %d", d.lastPropListLen)
	case d.lastSendObjHdr != lengthUnknown:
		t.Errorf("header length = %#x, want unknown", d.lastSendObjHdr)
	case o.size != size || o.crc != sent.Sum32():
		t.Errorf("device got %d bytes (crc %08x), want %d (crc %08x)", o.size, o.crc, uint64(size), sent.Sum32())
	}
	// Listing reports the full 64-bit size.
	list, err := c.List(ctx, 0x00020001, ParentRoot)
	if err != nil || len(list) != 1 || list[0].Size != size {
		t.Fatalf("list = %+v, %v", list, err)
	}
	checkFake(t, d)
}

func TestSizeOver4GBWithoutPropSupport(t *testing.T) {
	d := newFakeDevice()
	d.propList = false
	d.addObject(0x00010001, ParentRoot, "huge.xci", false, 5<<30, nil)
	c := open(t, d)
	_, err := c.List(context.Background(), 0x00010001, ParentRoot)
	if err == nil || !strings.Contains(err.Error(), "4 GB") {
		t.Fatalf("err = %v", err)
	}
}

func TestErrors(t *testing.T) {
	ctx := context.Background()
	d := newFakeDevice()
	c := open(t, d)

	// A device error keeps the connection usable.
	var re *RespError
	if err := c.Delete(ctx, 999); !errors.As(err, &re) || re.Code != RespInvalidObjectHndl {
		t.Fatalf("delete missing: %v", err)
	}
	if _, err := c.Storages(ctx); err != nil {
		t.Fatalf("after device error: %v", err)
	}

	// An I/O error midway breaks it.
	d.out = nil
	failing := &failConn{fakeDevice: d}
	c.conn = failing
	if _, err := c.Storages(ctx); err == nil || errors.Is(err, ErrBroken) {
		t.Fatalf("I/O error: %v", err)
	}
	if _, err := c.Storages(ctx); !errors.Is(err, ErrBroken) {
		t.Fatalf("after I/O error: %v", err)
	}
}

type failConn struct{ *fakeDevice }

func (f *failConn) Read(context.Context, []byte) (int, error) {
	return 0, errors.New("usb: device gone")
}
