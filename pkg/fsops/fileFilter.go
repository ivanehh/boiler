package fsops

import (
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

type FileFilterOption func(*FileFilter) error

type FileFilter struct {
	pattern string
	maxAge  time.Duration
	dir     []string
	matches []string
	drill   bool
}

func WithGlobPattern(p string) FileFilterOption {
	return func(ff *FileFilter) error {
		// check if the provided pattern is valid
		if _, err := fs.Glob(os.DirFS(""), p); err != nil {
			return err
		}
		ff.pattern = p
		return nil
	}
}

func WithFileAge(d time.Duration) FileFilterOption {
	return func(ff *FileFilter) error {
		ff.maxAge = d
		return nil
	}
}

func SetLoc(loc []string) FileFilterOption {
	return func(ff *FileFilter) error {
		ff.dir = loc
		return nil
	}
}

// WARN: Not implemented; has no effect on behavior
func Drill() FileFilterOption {
	return func(ff *FileFilter) error {
		ff.drill = true
		return nil
	}
}

func NewFileFilter(opts ...FileFilterOption) (*FileFilter, error) {
	ff := new(FileFilter)
	for _, opt := range opts {
		err := opt(ff)
		if err != nil {
			return nil, err
		}
	}
	return ff, nil
}

func (ff *FileFilter) SetDirs(d []string) {
	ff.dir = d
}

// Filter filters the files in the provided directories and returns a list of absolute file paths
func (ff FileFilter) Filter() ([]string, error) {
	for _, d := range ff.dir {
		matches, err := fs.Glob(os.DirFS(d), ff.pattern)
		if err != nil {
			return nil, err
		}
		for idx := range matches {
			matches[idx] = filepath.Join(d, matches[idx])
		}
		if ff.maxAge != 0 {
			for _, m := range matches {
				f, err := os.Open(m)
				if err != nil {
					return nil, err
				}

				finfo, _ := f.Stat()
				if finfo.ModTime().After(time.Now().Add(-ff.maxAge)) {
					ff.matches = append(ff.matches, m)
				}
				f.Close()
			}
			continue
		}
		ff.matches = append(ff.matches, matches...)
	}
	return ff.matches, nil
}
