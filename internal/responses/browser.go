package responses

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// File represents one saved response snapshot on disk.
type File struct {
	Name    string
	Path    string
	ModTime time.Time
	Size    int64
}

// List returns saved response files from dir, newest first.
func List(dir string) ([]File, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read responses dir: %w", err)
	}

	files := make([]File, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, fmt.Errorf("stat response file %q: %w", entry.Name(), err)
		}
		files = append(files, File{
			Name:    entry.Name(),
			Path:    filepath.Join(dir, entry.Name()),
			ModTime: info.ModTime(),
			Size:    info.Size(),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		if files[i].ModTime.Equal(files[j].ModTime) {
			return files[i].Name > files[j].Name
		}
		return files[i].ModTime.After(files[j].ModTime)
	})

	return files, nil
}

// Read loads a saved response snapshot.
func Read(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read response file: %w", err)
	}
	return string(data), nil
}

// Delete removes a saved response snapshot from disk.
func Delete(path string) error {
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete response file: %w", err)
	}
	return nil
}
