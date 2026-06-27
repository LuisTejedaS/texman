package collection

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Save serialises c as indented JSON and overwrites filePath atomically via
// a temporary file so a crash mid-write can never corrupt the collection.
func Save(filePath string, c Collection) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if dir := filepath.Dir(filePath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
	}
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", filePath, err)
	}
	return nil
}
