// Package mtp is a minimal MTP (Media Transfer Protocol, PTP over USB)
// initiator: enough to browse a device's storages, download, upload and
// delete files. DBI's "MTP responder" mode on the Switch is such a device.
package mtp

import "fmt"

// Container types (PTP over USB, "Still Image Capture Device Definition").
const (
	containerCommand  uint16 = 1
	containerData     uint16 = 2
	containerResponse uint16 = 3
	containerEvent    uint16 = 4
)

const headerSize = 12

// lengthUnknown in a container header means "more than 4 GB": the receiver
// relies on the size it knows from elsewhere, or on the end of the transfer.
const lengthUnknown = 0xFFFFFFFF

// Operation codes.
const (
	OpGetDeviceInfo      uint16 = 0x1001
	OpOpenSession        uint16 = 0x1002
	OpCloseSession       uint16 = 0x1003
	OpGetStorageIDs      uint16 = 0x1004
	OpGetStorageInfo     uint16 = 0x1005
	OpGetObjectHandles   uint16 = 0x1007
	OpGetObjectInfo      uint16 = 0x1008
	OpGetObject          uint16 = 0x1009
	OpDeleteObject       uint16 = 0x100B
	OpSendObjectInfo     uint16 = 0x100C
	OpSendObject         uint16 = 0x100D
	OpGetObjectPropValue uint16 = 0x9803
	OpSendObjectPropList uint16 = 0x9808
)

// Response codes.
const (
	RespOK                 uint16 = 0x2001
	RespGeneralError       uint16 = 0x2002
	RespSessionNotOpen     uint16 = 0x2003
	RespOperationNotSupp   uint16 = 0x2005
	RespInvalidObjectHndl  uint16 = 0x2009
	RespStoreFull          uint16 = 0x200C
	RespAccessDenied       uint16 = 0x200F
	RespDeviceBusy         uint16 = 0x2019
	RespSessionAlreadyOpen uint16 = 0x201E
)

// Object formats and properties.
const (
	FormatUndefined   uint16 = 0x3000
	FormatAssociation uint16 = 0x3001 // folder

	associationGenericFolder uint16 = 0x0001

	PropObjectSize     uint16 = 0xDC04
	PropObjectFileName uint16 = 0xDC07
)

// Special handles.
const (
	StorageAll   uint32 = 0xFFFFFFFF
	ParentRoot   uint32 = 0xFFFFFFFF // the storage root as a parent
	maxU32Length        = 0xFFFFFFFF
)

// Data types used in object property lists.
const (
	typeUint16 uint16 = 0x0004
	typeUint32 uint16 = 0x0006
	typeUint64 uint16 = 0x0008
	typeString uint16 = 0xFFFF
)

// RespError is a non-OK MTP response.
type RespError struct {
	Op   uint16
	Code uint16
}

func (e *RespError) Error() string {
	return fmt.Sprintf("MTP operation 0x%04X failed: %s", e.Op, respName(e.Code))
}

func respName(code uint16) string {
	switch code {
	case RespGeneralError:
		return "general error"
	case RespSessionNotOpen:
		return "session not open"
	case RespOperationNotSupp:
		return "operation not supported"
	case RespInvalidObjectHndl:
		return "invalid object handle"
	case RespStoreFull:
		return "storage full"
	case RespAccessDenied:
		return "access denied"
	case RespDeviceBusy:
		return "device busy"
	case RespSessionAlreadyOpen:
		return "session already open"
	}
	return fmt.Sprintf("response 0x%04X", code)
}
