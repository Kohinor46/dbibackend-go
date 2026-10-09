package gui

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// remoteFile is a file of a remote folder tree; rel is its slash-separated
// path below the tree's top folder.
type remoteFile struct {
	e   rEntry
	rel string
}

// walkRemote lists the folder dir and everything below it. rel is dir's
// path, "" for the top. Folders are returned too, so that empty ones are
// created locally.
func walkRemote(ctx context.Context, fs remoteFS, dir, rel string) (files []remoteFile, dirs []string, err error) {
	entries, err := fs.List(ctx, dir)
	if err != nil {
		return nil, nil, err
	}
	for _, e := range entries {
		r := path.Join(rel, safeName(e.Name))
		if !e.Dir {
			files = append(files, remoteFile{e: e, rel: r})
			continue
		}
		dirs = append(dirs, r)
		f, d, err := walkRemote(ctx, fs, e.ID, r)
		if err != nil {
			return nil, nil, err
		}
		files, dirs = append(files, f...), append(dirs, d...)
	}
	return files, dirs, nil
}

// safeName makes a remote name usable as a local file name on every
// system: characters Windows forbids become "_", and so do trailing dots
// and spaces.
func safeName(name string) string {
	b := []rune(name)
	for i, r := range b {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			b[i] = '_'
		}
	}
	s := string(b)
	if s == "" || s == "." || s == ".." {
		return "_"
	}
	if t := strings.TrimRight(s, ". "); t != s {
		s = t + strings.Repeat("_", len(s)-len(t))
	}
	return s
}

// treeDownload copies a remote folder into dest (a new or existing local
// folder), replacing local files with the same names.
type treeDownload struct {
	ctx      context.Context
	fs       remoteFS
	progress func(done, total int64)
	onFile   func(rel string, n, total int)
}

// run downloads dir into dest and returns the number of files.
func (t *treeDownload) run(dir, dest string) (int, error) {
	files, dirs, err := walkRemote(t.ctx, t.fs, dir, "")
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return 0, err
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(dest, filepath.FromSlash(d)), 0o755); err != nil {
			return 0, err
		}
	}
	var total, done int64
	for _, f := range files {
		total += f.e.Size
	}
	t.progress(0, total)
	for i, f := range files {
		t.onFile(f.rel, i+1, len(files))
		if err := t.file(f, filepath.Join(dest, filepath.FromSlash(f.rel)), done, total); err != nil {
			return i, err
		}
		done += f.e.Size
		t.progress(done, total)
	}
	return len(files), nil
}

func (t *treeDownload) file(f remoteFile, dest string, done, total int64) error {
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	err = t.fs.Download(t.ctx, f.e, &countingWriter{w: out, report: func(n int64) { t.progress(done+n, total) }})
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(dest) // don't leave a truncated file behind
		return fmt.Errorf("download %s: %w", f.rel, err)
	}
	return nil
}

// resolveDir finds the folder reached from root through names (matched
// case-insensitively), creating the ones that are missing.
func resolveDir(ctx context.Context, fs remoteFS, root string, names []string) (string, error) {
	dir := root
next:
	for _, name := range names {
		entries, err := fs.List(ctx, dir)
		if err != nil {
			return "", err
		}
		for _, e := range entries {
			if e.Dir && strings.EqualFold(e.Name, name) {
				dir = e.ID
				continue next
			}
		}
		if dir, err = fs.MakeDir(ctx, dir, name); err != nil {
			return "", err
		}
	}
	return dir, nil
}
