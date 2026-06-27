package collection

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// LoadAll reads every *.json file in dir and returns the parsed collections
// together with their source file paths (parallel slices, same order).
// Files are processed in lexicographic order for deterministic output.
func LoadAll(dir string) ([]Collection, []string, error) {
	pattern := filepath.Join(dir, "*.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, nil, fmt.Errorf("glob %q: %w", pattern, err)
	}

	sort.Strings(files)

	var cols []Collection
	var paths []string
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, nil, fmt.Errorf("read %s: %w", f, err)
		}
		var c Collection
		if err := json.Unmarshal(data, &c); err != nil {
			return nil, nil, fmt.Errorf("parse %s: %w", f, err)
		}
		cols = append(cols, c)
		paths = append(paths, f)
	}
	return cols, paths, nil
}

// LoadFile reads and parses a single collection JSON file.
func LoadFile(filePath string) (Collection, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return Collection{}, fmt.Errorf("read %s: %w", filePath, err)
	}
	var c Collection
	if err := json.Unmarshal(data, &c); err != nil {
		return Collection{}, fmt.Errorf("parse %s: %w", filePath, err)
	}
	return c, nil
}
