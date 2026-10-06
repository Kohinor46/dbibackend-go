package mtp

import (
	"encoding/binary"
	"errors"
	"unicode/utf16"
)

var errShortData = errors.New("MTP dataset is truncated")

// encoder builds little-endian PTP datasets.
type encoder struct{ b []byte }

func (e *encoder) u16(v uint16) { e.b = binary.LittleEndian.AppendUint16(e.b, v) }
func (e *encoder) u32(v uint32) { e.b = binary.LittleEndian.AppendUint32(e.b, v) }
func (e *encoder) u64(v uint64) { e.b = binary.LittleEndian.AppendUint64(e.b, v) }

// str writes a PTP string: character count (with the terminating NUL) as a
// byte, then UTF-16LE characters; the empty string is a single zero byte.
func (e *encoder) str(s string) {
	if s == "" {
		e.b = append(e.b, 0)
		return
	}
	u := utf16.Encode([]rune(s))
	if len(u) > 254 {
		u = u[:254]
	}
	e.b = append(e.b, byte(len(u)+1))
	for _, c := range u {
		e.u16(c)
	}
	e.u16(0)
}

// decoder reads PTP datasets; the first error sticks and later reads return zero.
type decoder struct {
	b   []byte
	err error
}

func (d *decoder) take(n int) []byte {
	if d.err != nil {
		return nil
	}
	if len(d.b) < n {
		d.err = errShortData
		return nil
	}
	v := d.b[:n]
	d.b = d.b[n:]
	return v
}

func (d *decoder) u16() uint16 {
	if v := d.take(2); v != nil {
		return binary.LittleEndian.Uint16(v)
	}
	return 0
}

func (d *decoder) u32() uint32 {
	if v := d.take(4); v != nil {
		return binary.LittleEndian.Uint32(v)
	}
	return 0
}

func (d *decoder) u64() uint64 {
	if v := d.take(8); v != nil {
		return binary.LittleEndian.Uint64(v)
	}
	return 0
}

func (d *decoder) str() string {
	n := d.take(1)
	if n == nil || n[0] == 0 {
		return ""
	}
	u := make([]uint16, n[0])
	for i := range u {
		u[i] = d.u16()
	}
	if len(u) > 0 && u[len(u)-1] == 0 {
		u = u[:len(u)-1]
	}
	return string(utf16.Decode(u))
}

func (d *decoder) u16s() []uint16 {
	n := d.u32()
	if d.err != nil || int(n) > len(d.b)/2 {
		d.err = errShortData
		return nil
	}
	v := make([]uint16, n)
	for i := range v {
		v[i] = d.u16()
	}
	return v
}

func (d *decoder) u32s() []uint32 {
	n := d.u32()
	if d.err != nil || int(n) > len(d.b)/4 {
		d.err = errShortData
		return nil
	}
	v := make([]uint32, n)
	for i := range v {
		v[i] = d.u32()
	}
	return v
}

// DeviceInfo is the PTP DeviceInfo dataset.
type DeviceInfo struct {
	StandardVersion     uint16
	VendorExtensionID   uint32
	VendorExtensionVer  uint16
	VendorExtensionDesc string
	FunctionalMode      uint16
	Operations          []uint16
	Events              []uint16
	DeviceProperties    []uint16
	CaptureFormats      []uint16
	PlaybackFormats     []uint16
	Manufacturer        string
	Model               string
	DeviceVersion       string
	SerialNumber        string
}

// Supports reports whether the device lists op among its operations.
func (di *DeviceInfo) Supports(op uint16) bool {
	for _, o := range di.Operations {
		if o == op {
			return true
		}
	}
	return false
}

func (di *DeviceInfo) marshal() []byte {
	e := &encoder{}
	e.u16(di.StandardVersion)
	e.u32(di.VendorExtensionID)
	e.u16(di.VendorExtensionVer)
	e.str(di.VendorExtensionDesc)
	e.u16(di.FunctionalMode)
	for _, list := range [][]uint16{di.Operations, di.Events, di.DeviceProperties, di.CaptureFormats, di.PlaybackFormats} {
		e.u32(uint32(len(list)))
		for _, v := range list {
			e.u16(v)
		}
	}
	e.str(di.Manufacturer)
	e.str(di.Model)
	e.str(di.DeviceVersion)
	e.str(di.SerialNumber)
	return e.b
}

func unmarshalDeviceInfo(b []byte) (*DeviceInfo, error) {
	d := &decoder{b: b}
	di := &DeviceInfo{
		StandardVersion:     d.u16(),
		VendorExtensionID:   d.u32(),
		VendorExtensionVer:  d.u16(),
		VendorExtensionDesc: d.str(),
		FunctionalMode:      d.u16(),
		Operations:          d.u16s(),
		Events:              d.u16s(),
		DeviceProperties:    d.u16s(),
		CaptureFormats:      d.u16s(),
		PlaybackFormats:     d.u16s(),
		Manufacturer:        d.str(),
		Model:               d.str(),
		DeviceVersion:       d.str(),
		SerialNumber:        d.str(),
	}
	return di, d.err
}

// StorageInfo is the PTP StorageInfo dataset.
type StorageInfo struct {
	StorageType      uint16
	FilesystemType   uint16
	AccessCapability uint16 // 0 read-write, 1 read-only, 2 read-only with delete
	MaxCapacity      uint64
	FreeSpace        uint64
	FreeObjects      uint32
	Description      string
	VolumeLabel      string
}

func (si *StorageInfo) marshal() []byte {
	e := &encoder{}
	e.u16(si.StorageType)
	e.u16(si.FilesystemType)
	e.u16(si.AccessCapability)
	e.u64(si.MaxCapacity)
	e.u64(si.FreeSpace)
	e.u32(si.FreeObjects)
	e.str(si.Description)
	e.str(si.VolumeLabel)
	return e.b
}

func unmarshalStorageInfo(b []byte) (*StorageInfo, error) {
	d := &decoder{b: b}
	si := &StorageInfo{
		StorageType:      d.u16(),
		FilesystemType:   d.u16(),
		AccessCapability: d.u16(),
		MaxCapacity:      d.u64(),
		FreeSpace:        d.u64(),
		FreeObjects:      d.u32(),
		Description:      d.str(),
		VolumeLabel:      d.str(),
	}
	return si, d.err
}

// ObjectInfo is the PTP ObjectInfo dataset. CompressedSize is 32-bit;
// files of 4 GB and more report 0xFFFFFFFF there (see Client.ObjectSize).
type ObjectInfo struct {
	StorageID        uint32
	Format           uint16
	ProtectionStatus uint16
	CompressedSize   uint32
	ThumbFormat      uint16
	ThumbSize        uint32
	ThumbWidth       uint32
	ThumbHeight      uint32
	ImageWidth       uint32
	ImageHeight      uint32
	ImageBitDepth    uint32
	Parent           uint32
	AssociationType  uint16
	AssociationDesc  uint32
	SequenceNumber   uint32
	Filename         string
	CaptureDate      string
	ModificationDate string
	Keywords         string
}

// IsFolder reports whether the object is a folder (association).
func (oi *ObjectInfo) IsFolder() bool { return oi.Format == FormatAssociation }

func (oi *ObjectInfo) marshal() []byte {
	e := &encoder{}
	e.u32(oi.StorageID)
	e.u16(oi.Format)
	e.u16(oi.ProtectionStatus)
	e.u32(oi.CompressedSize)
	e.u16(oi.ThumbFormat)
	e.u32(oi.ThumbSize)
	e.u32(oi.ThumbWidth)
	e.u32(oi.ThumbHeight)
	e.u32(oi.ImageWidth)
	e.u32(oi.ImageHeight)
	e.u32(oi.ImageBitDepth)
	e.u32(oi.Parent)
	e.u16(oi.AssociationType)
	e.u32(oi.AssociationDesc)
	e.u32(oi.SequenceNumber)
	e.str(oi.Filename)
	e.str(oi.CaptureDate)
	e.str(oi.ModificationDate)
	e.str(oi.Keywords)
	return e.b
}

func unmarshalObjectInfo(b []byte) (*ObjectInfo, error) {
	d := &decoder{b: b}
	oi := &ObjectInfo{
		StorageID:        d.u32(),
		Format:           d.u16(),
		ProtectionStatus: d.u16(),
		CompressedSize:   d.u32(),
		ThumbFormat:      d.u16(),
		ThumbSize:        d.u32(),
		ThumbWidth:       d.u32(),
		ThumbHeight:      d.u32(),
		ImageWidth:       d.u32(),
		ImageHeight:      d.u32(),
		ImageBitDepth:    d.u32(),
		Parent:           d.u32(),
		AssociationType:  d.u16(),
		AssociationDesc:  d.u32(),
		SequenceNumber:   d.u32(),
		Filename:         d.str(),
		CaptureDate:      d.str(),
		ModificationDate: d.str(),
		Keywords:         d.str(),
	}
	return oi, d.err
}

// objectPropList encodes the ObjectPropList dataset for SendObjectPropList:
// the file name and the full 64-bit size of the new object.
func objectPropList(name string, size uint64) []byte {
	e := &encoder{}
	e.u32(2) // number of elements
	e.u32(0) // object handle: 0 for a new object
	e.u16(PropObjectFileName)
	e.u16(typeString)
	e.str(name)
	e.u32(0)
	e.u16(PropObjectSize)
	e.u16(typeUint64)
	e.u64(size)
	return e.b
}
