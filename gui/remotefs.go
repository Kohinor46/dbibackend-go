package gui

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/Kohinor46/dbibackend-go/mtp"
)

// rEntry is a file or folder on the remote side (the Switch over MTP or FTP).
type rEntry struct {
	ID   string // backend-specific: "storage/handle" for MTP, a path for FTP
	Name string
	Dir  bool
	Size int64
}

// rRoot is a top-level location the browser can open: an MTP storage, or
// the FTP server's root.
type rRoot struct {
	ID    string
	Name  string
	Flat  bool // an install target: upload the files of folders without the folders
	Saves bool // DBI's game saves ("7: Saves"), offered for backup
}

// remoteFS is what the file browser needs from a backend. The browser calls
// it from a background goroutine, one call at a time.
type remoteFS interface {
	List(ctx context.Context, dir string) ([]rEntry, error)
	MakeDir(ctx context.Context, parent, name string) (string, error)
	Delete(ctx context.Context, e rEntry) error
	Upload(ctx context.Context, parent, name string, size int64, r io.Reader) (string, error)
	Download(ctx context.Context, e rEntry, w io.Writer) error
}

// mtpFS adapts an MTP client to remoteFS.
type mtpFS struct{ c mtpClient }

func mtpID(storage, handle uint32) string {
	return strconv.FormatUint(uint64(storage), 16) + "/" + strconv.FormatUint(uint64(handle), 10)
}

func parseMTPID(id string) (storage, handle uint32, err error) {
	s, h, ok := strings.Cut(id, "/")
	st, err1 := strconv.ParseUint(s, 16, 32)
	hd, err2 := strconv.ParseUint(h, 10, 32)
	if !ok || err1 != nil || err2 != nil {
		return 0, 0, fmt.Errorf("bad MTP object id %q", id)
	}
	return uint32(st), uint32(hd), nil
}

// mtpRoots turns storages into browser roots; DBI's "install" storages are
// flat, its "Saves" storage holds the game saves.
func mtpRoots(storages []mtp.Storage) []rRoot {
	roots := make([]rRoot, len(storages))
	for i, s := range storages {
		desc := strings.ToLower(s.Info.Description)
		roots[i] = rRoot{
			ID:    mtpID(s.ID, mtp.ParentRoot),
			Name:  storageName(s),
			Flat:  strings.Contains(desc, "install"),
			Saves: strings.Contains(desc, "save"),
		}
	}
	return roots
}

func (f mtpFS) List(ctx context.Context, dir string) ([]rEntry, error) {
	storage, parent, err := parseMTPID(dir)
	if err != nil {
		return nil, err
	}
	objs, err := f.c.List(ctx, storage, parent)
	if err != nil {
		return nil, err
	}
	entries := make([]rEntry, len(objs))
	for i, o := range objs {
		entries[i] = rEntry{ID: mtpID(storage, o.Handle), Name: o.Name(), Dir: o.IsFolder(), Size: int64(o.Size)}
	}
	return entries, nil
}

func (f mtpFS) MakeDir(ctx context.Context, parent, name string) (string, error) {
	storage, p, err := parseMTPID(parent)
	if err != nil {
		return "", err
	}
	h, err := f.c.MakeFolder(ctx, storage, p, name)
	return mtpID(storage, h), err
}

func (f mtpFS) Delete(ctx context.Context, e rEntry) error {
	_, h, err := parseMTPID(e.ID)
	if err != nil {
		return err
	}
	return f.c.Delete(ctx, h)
}

func (f mtpFS) Upload(ctx context.Context, parent, name string, size int64, r io.Reader) (string, error) {
	storage, p, err := parseMTPID(parent)
	if err != nil {
		return "", err
	}
	h, err := f.c.Upload(ctx, storage, p, name, uint64(size), r)
	return mtpID(storage, h), err
}

func (f mtpFS) Download(ctx context.Context, e rEntry, w io.Writer) error {
	_, h, err := parseMTPID(e.ID)
	if err != nil {
		return err
	}
	return f.c.Download(ctx, mtp.Object{Handle: h, Size: uint64(e.Size)}, w)
}
