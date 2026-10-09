package gui

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// localItem is a file or folder to upload; rel is its slash-separated path
// below the folder being uploaded into, starting with the selected item's name.
type localItem struct {
	path string
	rel  string
	dir  bool
	size int64
}

// isJunk reports files the OS leaves behind that don't belong on the Switch.
func isJunk(name string) bool {
	return name == ".DS_Store" || strings.HasPrefix(name, "._") ||
		strings.EqualFold(name, "Thumbs.db") || strings.EqualFold(name, "desktop.ini")
}

// collectLocal expands the selected paths into files and folders, parents
// before their contents. Symlinks and OS junk files are left out.
func collectLocal(paths []string) (items []localItem, total int64) {
	for _, p := range paths {
		p = filepath.Clean(p)
		base := filepath.Dir(p)
		filepath.WalkDir(p, func(cur string, d fs.DirEntry, err error) error {
			if err != nil || d.Type()&fs.ModeSymlink != 0 || isJunk(d.Name()) {
				if d != nil && d.IsDir() && cur != p {
					return fs.SkipDir
				}
				return nil
			}
			rel, _ := filepath.Rel(base, cur)
			it := localItem{path: cur, rel: filepath.ToSlash(rel), dir: d.IsDir()}
			if !it.dir {
				info, err := d.Info()
				if err != nil || !info.Mode().IsRegular() {
					return nil
				}
				it.size = info.Size()
				total += it.size
			}
			items = append(items, it)
			return nil
		})
	}
	return items, total
}

type uploadStats struct {
	uploaded int // files sent (including replaced ones)
	replaced int // files that replaced one with the same name
	skipped  int // files or folders not sent because of a file/folder name clash
}

// uploader merges local files and folders into a folder on the device:
// existing folders are reused, files with the same name are replaced, and
// nothing else on the device is touched. Names match case-insensitively,
// like on the Switch's FAT32/exFAT storage.
type uploader struct {
	ctx      context.Context
	fs       remoteFS
	flat     bool // upload all files straight into root, without folders
	log      *slog.Logger
	progress func(done int64)
	onFile   func(name string, n, total int)

	folders  map[string]string            // rel folder path → remote folder ID
	children map[string]map[string]rEntry // remote folder ID → lowercased name → entry
	stats    uploadStats
	done     int64
}

func newUploader(ctx context.Context, fs remoteFS, root string, flat bool, log *slog.Logger) *uploader {
	return &uploader{
		ctx: ctx, fs: fs, flat: flat, log: log,
		progress: func(int64) {}, onFile: func(string, int, int) {},
		folders:  map[string]string{"": root},
		children: map[string]map[string]rEntry{},
	}
}

// list returns the remote folder's children, read once.
func (u *uploader) list(folder string) (map[string]rEntry, error) {
	if m, ok := u.children[folder]; ok {
		return m, nil
	}
	entries, err := u.fs.List(u.ctx, folder)
	if err != nil {
		return nil, err
	}
	m := make(map[string]rEntry, len(entries))
	for _, e := range entries {
		m[strings.ToLower(e.Name)] = e
	}
	u.children[folder] = m
	return m, nil
}

func (u *uploader) run(items []localItem) error {
	files := 0
	for _, it := range items {
		if !it.dir {
			files++
		}
	}
	n := 0
	for _, it := range items {
		name := path.Base(it.rel)
		parentRel := path.Dir(it.rel)
		if parentRel == "." {
			parentRel = ""
		}
		if u.flat {
			if it.dir {
				continue
			}
			parentRel = ""
		}
		parent, ok := u.folders[parentRel]
		if !ok { // its folder was skipped because of a name clash
			u.stats.skipped++
			continue
		}
		children, err := u.list(parent)
		if err != nil {
			return err
		}
		existing, exists := children[strings.ToLower(name)]

		if it.dir {
			switch {
			case exists && existing.Dir:
				u.folders[it.rel] = existing.ID // merge into it
			case exists:
				u.log.Warn("Skipped folder: a file with this name exists on the Switch", "name", it.rel)
				u.stats.skipped++
			default:
				id, err := u.fs.MakeDir(u.ctx, parent, name)
				if err != nil {
					return fmt.Errorf("create folder %s: %w", it.rel, err)
				}
				u.folders[it.rel] = id
				children[strings.ToLower(name)] = rEntry{ID: id, Name: name, Dir: true}
				u.children[id] = map[string]rEntry{} // new and empty: no need to list it
			}
			continue
		}

		n++
		u.onFile(it.rel, n, files)
		if exists && existing.Dir {
			u.log.Warn("Skipped file: a folder with this name exists on the Switch", "name", it.rel)
			u.stats.skipped++
			u.done += it.size
			u.progress(u.done)
			continue
		}
		if exists {
			// Neither MTP nor DBI's FTP overwrite in place reliably: remove
			// the old file, then send the new one.
			if err := u.fs.Delete(u.ctx, existing); err != nil {
				return fmt.Errorf("replace %s: %w", it.rel, err)
			}
			delete(children, strings.ToLower(name))
			u.stats.replaced++
		}
		id, err := u.send(it, parent, name)
		if err != nil {
			return err
		}
		children[strings.ToLower(name)] = rEntry{ID: id, Name: name, Size: it.size}
		u.stats.uploaded++
		u.log.Info("Uploaded", "file", it.rel, "size", it.size, "replaced", exists)
	}
	return nil
}

func (u *uploader) send(it localItem, parent, name string) (string, error) {
	f, err := os.Open(it.path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	id, err := u.fs.Upload(u.ctx, parent, name, it.size, &countingReader{r: f, done: u.done, report: u.progress})
	if err != nil {
		return "", fmt.Errorf("upload %s: %w", it.rel, err)
	}
	u.done += it.size
	return id, nil
}
