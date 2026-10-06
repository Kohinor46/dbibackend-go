// Package dbi implements the PC side of the DBI USB install protocol.
package dbi

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// Nintendo Switch USB IDs used by DBI.
const (
	SwitchVID = 0x057E
	SwitchPID = 0x3000
)

// BufferSegmentDataSize is the chunk size used when streaming file ranges.
const BufferSegmentDataSize = 0x100000

const headerSize = 16

var magic = [4]byte{'D', 'B', 'I', '0'}

type CommandID uint32

const (
	CmdExit           CommandID = 0
	CmdListDeprecated CommandID = 1
	CmdFileRange      CommandID = 2
	CmdList           CommandID = 3
)

func (c CommandID) String() string {
	switch c {
	case CmdExit:
		return "EXIT"
	case CmdListDeprecated:
		return "LIST_DEPRECATED"
	case CmdFileRange:
		return "FILE_RANGE"
	case CmdList:
		return "LIST"
	}
	return fmt.Sprintf("CMD(%d)", uint32(c))
}

type CommandType uint32

const (
	TypeRequest  CommandType = 0
	TypeResponse CommandType = 1
	TypeAck      CommandType = 2
)

// Header is the 16-byte packet header: magic, type, command id, data size.
type Header struct {
	Type     CommandType
	ID       CommandID
	DataSize uint32
}

func (h Header) Marshal() []byte {
	b := make([]byte, headerSize)
	copy(b[:4], magic[:])
	binary.LittleEndian.PutUint32(b[4:8], uint32(h.Type))
	binary.LittleEndian.PutUint32(b[8:12], uint32(h.ID))
	binary.LittleEndian.PutUint32(b[12:16], h.DataSize)
	return b
}

// ParseHeader decodes a header; ok is false if the magic doesn't match.
func ParseHeader(b []byte) (h Header, ok bool) {
	if len(b) < headerSize || !bytes.Equal(b[:4], magic[:]) {
		return Header{}, false
	}
	return Header{
		Type:     CommandType(binary.LittleEndian.Uint32(b[4:8])),
		ID:       CommandID(binary.LittleEndian.Uint32(b[8:12])),
		DataSize: binary.LittleEndian.Uint32(b[12:16]),
	}, true
}

// FileRange is the payload of a FILE_RANGE request.
type FileRange struct {
	Size   uint32
	Offset uint64
	Name   string
}

func ParseFileRange(b []byte) (FileRange, error) {
	if len(b) < 16 {
		return FileRange{}, fmt.Errorf("file range header too short: %d bytes", len(b))
	}
	fr := FileRange{
		Size:   binary.LittleEndian.Uint32(b[0:4]),
		Offset: binary.LittleEndian.Uint64(b[4:12]),
	}
	nameLen := int(binary.LittleEndian.Uint32(b[12:16]))
	name := b[16:]
	if nameLen < len(name) {
		name = name[:nameLen]
	}
	fr.Name = string(name)
	return fr, nil
}
