package dbi

import (
	"io/fs"
	"path/filepath"
	"strings"
)

var titleExts = map[string]bool{".nsp": true, ".nsz": true, ".xci": true}

// Title is an installable file found in the titles directory.
type Title struct {
	Name string // name as exposed to DBI (base file name)
	Path string // full path on disk
	Size int64
}

// ScanTitles walks dir recursively and returns all .nsp/.nsz/.xci files.
// DBI only sees base names, so when two files share a name the first one wins.
func ScanTitles(dir string) ([]Title, error) {
	var titles []Title
	seen := map[string]bool{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == dir {
				return err
			}
			return nil // skip unreadable subdirectories
		}
		if d.IsDir() || !titleExts[strings.ToLower(filepath.Ext(d.Name()))] || seen[d.Name()] {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		seen[d.Name()] = true
		titles = append(titles, Title{Name: d.Name(), Path: path, Size: info.Size()})
		return nil
	})
	return titles, err
}
