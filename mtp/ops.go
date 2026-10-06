package mtp

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
)

// Storage is one storage of the device (DBI exposes the SD card, NAND,
// "install" targets and so on as separate storages).
type Storage struct {
	ID   uint32
	Info StorageInfo
}

// Object is a file or folder.
type Object struct {
	Handle uint32
	Info   ObjectInfo
	Size   uint64 // full size, also for files of 4 GB and more
}

// Name returns the object's file name.
func (o *Object) Name() string { return o.Info.Filename }

// IsFolder reports whether the object is a folder.
func (o *Object) IsFolder() bool { return o.Info.IsFolder() }

// Storages lists the device's storages.
func (c *Client) Storages(ctx context.Context) ([]Storage, error) {
	var ids byteSink
	if _, err := c.run(ctx, OpGetStorageIDs, nil, nil, &ids, -1); err != nil {
		return nil, err
	}
	d := &decoder{b: ids.b}
	list := d.u32s()
	if d.err != nil {
		return nil, fmt.Errorf("storage IDs: %w", d.err)
	}
	storages := make([]Storage, 0, len(list))
	for _, id := range list {
		var data byteSink
		if _, err := c.run(ctx, OpGetStorageInfo, []uint32{id}, nil, &data, -1); err != nil {
			return nil, err
		}
		info, err := unmarshalStorageInfo(data.b)
		if err != nil {
			return nil, fmt.Errorf("storage 0x%08X info: %w", id, err)
		}
		storages = append(storages, Storage{ID: id, Info: *info})
	}
	return storages, nil
}

// List returns the objects in a folder; parent ParentRoot lists the storage root.
func (c *Client) List(ctx context.Context, storage, parent uint32) ([]Object, error) {
	var handles byteSink
	if _, err := c.run(ctx, OpGetObjectHandles, []uint32{storage, 0, parent}, nil, &handles, -1); err != nil {
		return nil, err
	}
	d := &decoder{b: handles.b}
	list := d.u32s()
	if d.err != nil {
		return nil, fmt.Errorf("object handles: %w", d.err)
	}
	objects := make([]Object, 0, len(list))
	for _, h := range list {
		o, err := c.Object(ctx, h)
		if err != nil {
			return nil, err
		}
		objects = append(objects, o)
	}
	return objects, nil
}

// Object reads one object's info and full size.
func (c *Client) Object(ctx context.Context, handle uint32) (Object, error) {
	var data byteSink
	if _, err := c.run(ctx, OpGetObjectInfo, []uint32{handle}, nil, &data, -1); err != nil {
		return Object{}, err
	}
	info, err := unmarshalObjectInfo(data.b)
	if err != nil {
		return Object{}, fmt.Errorf("object 0x%08X info: %w", handle, err)
	}
	o := Object{Handle: handle, Info: *info, Size: uint64(info.CompressedSize)}
	if info.CompressedSize == maxU32Length && !info.IsFolder() {
		// 4 GB or more: the real size is only in the 64-bit ObjectSize property.
		if o.Size, err = c.objectSize(ctx, handle); err != nil {
			return Object{}, err
		}
	}
	return o, nil
}

func (c *Client) objectSize(ctx context.Context, handle uint32) (uint64, error) {
	if info := c.Info(); info == nil || !info.Supports(OpGetObjectPropValue) {
		return 0, fmt.Errorf("object 0x%08X is 4 GB or larger, but the device can't report its size", handle)
	}
	var data byteSink
	if _, err := c.run(ctx, OpGetObjectPropValue, []uint32{handle, uint32(PropObjectSize)}, nil, &data, -1); err != nil {
		return 0, err
	}
	if len(data.b) < 8 {
		return 0, fmt.Errorf("object 0x%08X size: %w", handle, errShortData)
	}
	return binary.LittleEndian.Uint64(data.b), nil
}

// Download writes object o's contents to w.
func (c *Client) Download(ctx context.Context, o Object, w io.Writer) error {
	_, err := c.run(ctx, OpGetObject, []uint32{o.Handle}, nil, w, int64(o.Size))
	return err
}

// Upload creates a file named name of the given size in parent (ParentRoot
// for the storage root) and sends size bytes from r. It returns the new
// object's handle.
//
// Files of 4 GB and more don't fit the 32-bit size field of ObjectInfo, so
// when the device supports it the object is announced with
// SendObjectPropList, which carries a 64-bit size (as Windows does).
func (c *Client) Upload(ctx context.Context, storage, parent uint32, name string, size uint64, r io.Reader) (uint32, error) {
	var resp []uint32
	var err error
	if info := c.Info(); info != nil && info.Supports(OpSendObjectPropList) {
		params := []uint32{storage, parent, uint32(FormatUndefined), uint32(size >> 32), uint32(size)}
		props := objectPropList(name, size)
		resp, err = c.run(ctx, OpSendObjectPropList, params, &dataOut{r: bytes.NewReader(props), size: uint64(len(props))}, nil, -1)
	} else {
		oi := ObjectInfo{StorageID: storage, Format: FormatUndefined, CompressedSize: uint32(min(size, maxU32Length)), Filename: name}
		if parent != ParentRoot {
			oi.Parent = parent
		}
		b := oi.marshal()
		resp, err = c.run(ctx, OpSendObjectInfo, []uint32{storage, parent}, &dataOut{r: bytes.NewReader(b), size: uint64(len(b))}, nil, -1)
	}
	if err != nil {
		return 0, err
	}
	if len(resp) < 3 {
		return 0, fmt.Errorf("upload: device returned no object handle")
	}
	handle := resp[2]
	if _, err := c.run(ctx, OpSendObject, nil, &dataOut{r: r, size: size}, nil, -1); err != nil {
		return 0, err
	}
	return handle, nil
}

// MakeFolder creates a folder in parent and returns its handle.
func (c *Client) MakeFolder(ctx context.Context, storage, parent uint32, name string) (uint32, error) {
	oi := ObjectInfo{StorageID: storage, Format: FormatAssociation, AssociationType: associationGenericFolder, Filename: name}
	if parent != ParentRoot {
		oi.Parent = parent
	}
	b := oi.marshal()
	resp, err := c.run(ctx, OpSendObjectInfo, []uint32{storage, parent}, &dataOut{r: bytes.NewReader(b), size: uint64(len(b))}, nil, -1)
	if err != nil {
		return 0, err
	}
	if len(resp) < 3 {
		return 0, fmt.Errorf("make folder: device returned no object handle")
	}
	return resp[2], nil
}

// Delete removes a file or folder.
func (c *Client) Delete(ctx context.Context, handle uint32) error {
	_, err := c.run(ctx, OpDeleteObject, []uint32{handle, 0}, nil, nil, -1)
	return err
}
