package mtp

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"sort"
)

// fakeDevice is an in-memory MTP responder speaking the USB bulk protocol
// byte by byte, so the client's container framing, zero-length packets and
// >4 GB handling are exercised without hardware.
type fakeDevice struct {
	maxPacket int

	// Behaviour switches.
	propList        bool // supports SendObjectPropList and GetObjectPropValue
	unknownLenData  bool // data-in containers use lengthUnknown in the header
	mergeResponse   bool // response shares the transfer with the data (no ZLP)
	keepLimit       int  // objects up to this size keep their bytes
	ignoreSize      bool // end lengthUnknown uploads only at a short packet
	errors          []string
	lastSendObjHdr  uint32 // header length of the last SendObject data container
	lastPropListLen uint64 // 64-bit size given in the last SendObjectPropList

	storages []fakeStorage
	objects  map[uint32]*fakeObj
	next     uint32

	in        []byte
	out       [][]byte
	expectZLP bool

	// Host-to-device data phase in progress.
	data *dataPhase
	// Object created by SendObjectInfo/SendObjectPropList, filled by SendObject.
	target *fakeObj
}

type fakeStorage struct {
	id   uint32
	desc string
}

type fakeObj struct {
	handle, storage, parent uint32
	name                    string
	folder                  bool
	size                    uint64
	data                    []byte
	crc                     uint32
}

type dataPhase struct {
	op       uint16
	params   []uint32
	tid      uint32
	header   []byte
	expect   int64 // payload bytes expected; -1: until a short packet
	got      int64
	total    uint64 // container bytes seen, header included
	buf      []byte // small payloads are collected here
	onFinish func(*dataPhase)
	sink     func([]byte)
}

func newFakeDevice() *fakeDevice {
	return &fakeDevice{
		maxPacket: 512,
		propList:  true,
		keepLimit: 1 << 20,
		storages:  []fakeStorage{{0x00010001, "SD Card"}, {0x00020001, "SD Card install"}},
		objects:   map[uint32]*fakeObj{},
		next:      1,
	}
}

func (d *fakeDevice) fail(format string, a ...any) {
	d.errors = append(d.errors, fmt.Sprintf(format, a...))
}

// addObject puts an object on the device directly (for tests).
func (d *fakeDevice) addObject(storage, parent uint32, name string, folder bool, size uint64, data []byte) *fakeObj {
	o := &fakeObj{handle: d.next, storage: storage, parent: parent, name: name, folder: folder, size: size, data: data}
	d.objects[o.handle] = o
	d.next++
	return o
}

func (d *fakeDevice) MaxPacketSize() int { return d.maxPacket }

func (d *fakeDevice) Read(_ context.Context, buf []byte) (int, error) {
	if len(d.out) == 0 {
		return 0, errors.New("fake: host reads, but the device has nothing to send")
	}
	t := d.out[0]
	n := copy(buf, t)
	if n < len(t) {
		d.out[0] = t[n:]
	} else {
		d.out = d.out[1:]
	}
	return n, nil
}

func (d *fakeDevice) Write(_ context.Context, b []byte) (int, error) {
	if len(b) == 0 {
		switch {
		case d.expectZLP:
			d.expectZLP = false
		case d.data != nil && d.data.expect < 0:
			d.finishData()
		default:
			d.fail("unexpected zero-length packet")
		}
		return 0, nil
	}
	if d.expectZLP {
		d.fail("missing zero-length packet before %d new bytes", len(b))
		d.expectZLP = false
	}
	if d.data != nil {
		d.feedData(b)
		return len(b), nil
	}
	d.in = append(d.in, b...)
	for len(d.in) >= headerSize {
		length := binary.LittleEndian.Uint32(d.in)
		if len(d.in) < int(length) {
			break
		}
		c := d.in[:length]
		d.in = d.in[length:]
		if typ := binary.LittleEndian.Uint16(c[4:]); typ != containerCommand {
			d.fail("expected a command container, got type %d", typ)
			continue
		}
		var params []uint32
		for p := c[headerSize:]; len(p) >= 4; p = p[4:] {
			params = append(params, binary.LittleEndian.Uint32(p))
		}
		d.command(binary.LittleEndian.Uint16(c[6:]), binary.LittleEndian.Uint32(c[8:]), params)
	}
	return len(b), nil
}

// feedData consumes host-to-device data bytes of the current data phase.
func (d *fakeDevice) feedData(b []byte) {
	p := d.data
	p.total += uint64(len(b))
	short := len(b)%d.maxPacket != 0
	if len(p.header) < headerSize {
		k := min(headerSize-len(p.header), len(b))
		p.header = append(p.header, b[:k]...)
		b = b[k:]
		if len(p.header) == headerSize {
			length := binary.LittleEndian.Uint32(p.header)
			if typ := binary.LittleEndian.Uint16(p.header[4:]); typ != containerData {
				d.fail("expected a data container, got type %d", typ)
			}
			if p.op == OpSendObject {
				d.lastSendObjHdr = length
			}
			if length != lengthUnknown {
				p.expect = int64(length) - headerSize
			}
		}
	}
	if len(b) > 0 {
		p.got += int64(len(b))
		if p.sink != nil {
			p.sink(b)
		} else {
			p.buf = append(p.buf, b...)
		}
	}
	switch {
	case p.expect >= 0 && p.got > p.expect:
		d.fail("data phase: got %d bytes, expected %d", p.got, p.expect)
	case p.expect >= 0 && p.got == p.expect && len(p.header) == headerSize:
		if p.total%uint64(d.maxPacket) == 0 {
			d.expectZLP = true
		}
		d.finishData()
	case p.expect < 0 && len(p.header) == headerSize && short:
		d.finishData()
	}
}

func (d *fakeDevice) finishData() {
	p := d.data
	d.data = nil
	p.onFinish(p)
}

func (d *fakeDevice) respond(tid uint32, code uint16, params ...uint32) {
	r := make([]byte, headerSize, headerSize+4*len(params))
	putHeader(r, uint32(headerSize+4*len(params)), containerResponse, code, tid)
	for _, v := range params {
		r = binary.LittleEndian.AppendUint32(r, v)
	}
	d.out = append(d.out, r)
}

// sendData queues a device-to-host data container followed by the response.
func (d *fakeDevice) sendData(op uint16, tid uint32, payload []byte) {
	length := uint32(headerSize + len(payload))
	if d.unknownLenData {
		length = lengthUnknown
	}
	c := make([]byte, headerSize, headerSize+len(payload))
	putHeader(c, length, containerData, op, tid)
	c = append(c, payload...)
	switch {
	case d.mergeResponse:
		d.respond(tid, RespOK)
		r := d.out[len(d.out)-1]
		d.out[len(d.out)-1] = append(c, r...)
	default:
		d.out = append(d.out, c)
		if len(c)%d.maxPacket == 0 {
			d.out = append(d.out, []byte{}) // zero-length packet
		}
		d.respond(tid, RespOK)
	}
}

func (d *fakeDevice) objectInfo(o *fakeObj) []byte {
	oi := ObjectInfo{StorageID: o.storage, Format: FormatUndefined, CompressedSize: uint32(min(o.size, maxU32Length)), Filename: o.name}
	if o.folder {
		oi.Format, oi.AssociationType = FormatAssociation, associationGenericFolder
	}
	if o.parent != ParentRoot {
		oi.Parent = o.parent
	}
	return oi.marshal()
}

func (d *fakeDevice) command(op uint16, tid uint32, params []uint32) {
	arg := func(i int) uint32 {
		if i < len(params) {
			return params[i]
		}
		return 0
	}
	switch op {
	case OpOpenSession, OpCloseSession:
		d.respond(tid, RespOK)
	case OpGetDeviceInfo:
		ops := []uint16{OpGetDeviceInfo, OpOpenSession, OpCloseSession, OpGetStorageIDs, OpGetStorageInfo,
			OpGetObjectHandles, OpGetObjectInfo, OpGetObject, OpDeleteObject, OpSendObjectInfo, OpSendObject}
		if d.propList {
			ops = append(ops, OpGetObjectPropValue, OpSendObjectPropList)
		}
		info := DeviceInfo{StandardVersion: 100, Operations: ops, Manufacturer: "Fake", Model: "Switch (DBI)", SerialNumber: "0001"}
		d.sendData(op, tid, info.marshal())
	case OpGetStorageIDs:
		e := &encoder{}
		e.u32(uint32(len(d.storages)))
		for _, s := range d.storages {
			e.u32(s.id)
		}
		d.sendData(op, tid, e.b)
	case OpGetStorageInfo:
		for _, s := range d.storages {
			if s.id == arg(0) {
				si := StorageInfo{StorageType: 3, FilesystemType: 2, MaxCapacity: 256 << 30, FreeSpace: 100 << 30, Description: s.desc}
				d.sendData(op, tid, si.marshal())
				return
			}
		}
		d.respond(tid, 0x2008) // invalid storage ID
	case OpGetObjectHandles:
		var hs []uint32
		for h, o := range d.objects {
			if o.storage == arg(0) && o.parent == arg(2) {
				hs = append(hs, h)
			}
		}
		sort.Slice(hs, func(i, j int) bool { return hs[i] < hs[j] })
		e := &encoder{}
		e.u32(uint32(len(hs)))
		for _, h := range hs {
			e.u32(h)
		}
		d.sendData(op, tid, e.b)
	case OpGetObjectInfo, OpGetObject, OpGetObjectPropValue, OpDeleteObject:
		o := d.objects[arg(0)]
		if o == nil {
			d.respond(tid, RespInvalidObjectHndl)
			return
		}
		switch op {
		case OpGetObjectInfo:
			d.sendData(op, tid, d.objectInfo(o))
		case OpGetObject:
			if uint64(len(o.data)) != o.size {
				d.fail("GetObject on an object without stored data")
			}
			d.sendData(op, tid, o.data)
		case OpGetObjectPropValue:
			if !d.propList || uint16(arg(1)) != PropObjectSize {
				d.respond(tid, RespOperationNotSupp)
				return
			}
			d.sendData(op, tid, binary.LittleEndian.AppendUint64(nil, o.size))
		case OpDeleteObject:
			d.delete(o)
			d.respond(tid, RespOK)
		}
	case OpSendObjectInfo, OpSendObjectPropList:
		if op == OpSendObjectPropList && !d.propList {
			d.respond(tid, RespOperationNotSupp)
			return
		}
		d.data = &dataPhase{op: op, params: params, tid: tid, expect: -1, onFinish: d.objectCreated}
	case OpSendObject:
		t := d.target
		if t == nil {
			d.respond(tid, 0x2016) // no valid object info
			return
		}
		h := crc32.NewIEEE()
		t.data = nil
		var got uint64
		p := &dataPhase{op: op, tid: tid, expect: -1}
		p.sink = func(b []byte) {
			h.Write(b)
			got += uint64(len(b))
			if t.size <= uint64(d.keepLimit) {
				t.data = append(t.data, b...)
			}
			// With lengthUnknown the device relies on the announced size.
			if p.expect < 0 && !d.ignoreSize && t.size != maxU32Length && got == t.size {
				p.expect = int64(got)
			}
		}
		p.onFinish = func(p *dataPhase) {
			t.crc = h.Sum32()
			if t.size == maxU32Length || d.ignoreSize { // size not taken from the announcement
				t.size = got
			}
			if got != t.size {
				d.fail("SendObject: got %d bytes, announced %d", got, t.size)
			}
			d.target = nil
			d.respond(p.tid, RespOK)
		}
		d.data = p
	default:
		d.respond(tid, RespOperationNotSupp)
	}
}

// objectCreated handles the dataset of SendObjectInfo / SendObjectPropList.
func (d *fakeDevice) objectCreated(p *dataPhase) {
	storage, parent := p.params[0], p.params[1]
	o := &fakeObj{storage: storage, parent: parent}
	if p.op == OpSendObjectInfo {
		oi, err := unmarshalObjectInfo(p.buf)
		if err != nil {
			d.fail("SendObjectInfo dataset: %v", err)
		}
		o.name, o.folder, o.size = oi.Filename, oi.IsFolder(), uint64(oi.CompressedSize)
	} else {
		o.size = uint64(p.params[3])<<32 | uint64(p.params[4])
		dec := &decoder{b: p.buf}
		for n := dec.u32(); n > 0 && dec.err == nil; n-- {
			dec.u32()
			code, typ := dec.u16(), dec.u16()
			switch typ {
			case typeString:
				if v := dec.str(); code == PropObjectFileName {
					o.name = v
				}
			case typeUint64:
				if v := dec.u64(); code == PropObjectSize && v != o.size {
					d.fail("prop list size %d != operation size %d", v, o.size)
				}
			default:
				d.fail("unexpected prop type 0x%04X", typ)
			}
		}
		d.lastPropListLen = o.size
	}
	o.handle = d.next
	d.next++
	d.objects[o.handle] = o
	if !o.folder {
		d.target = o
	}
	d.respond(p.tid, RespOK, storage, parent, o.handle)
}

func (d *fakeDevice) delete(o *fakeObj) {
	for _, c := range d.objects {
		if c.parent == o.handle {
			d.delete(c)
		}
	}
	delete(d.objects, o.handle)
}
