package responses

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/luisalfredotejeda/texman/internal/httpclient"
)

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// Save writes resp to dir/<timestamp>_<slugified-reqName>.txt.
// It creates dir if it does not exist and returns the full path written.
func Save(dir, reqName string, resp *httpclient.Response) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create responses dir: %w", err)
	}

	ts := time.Now().Format("2006-01-02_15-04-05")
	slug := slugify(reqName)
	name := ts + "_" + slug + ".txt"
	path := filepath.Join(dir, name)

	content := format(reqName, resp)

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write response file: %w", err)
	}
	return path, nil
}

// format builds the human-readable file content.
func format(reqName string, resp *httpclient.Response) string {
	var sb strings.Builder

	sb.WriteString("Request:  " + reqName + "\n")
	sb.WriteString("Status:   " + resp.Status + "\n")
	sb.WriteString("Duration: " + fmt.Sprintf("%dms", resp.Duration.Milliseconds()) + "\n")
	sb.WriteString("\n")

	if len(resp.Headers) > 0 {
		sb.WriteString("Headers:\n")
		for _, k := range sortedKeys(resp.Headers) {
			sb.WriteString("  " + k + ": " + strings.Join(resp.Headers[k], ", ") + "\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Body:\n")
	sb.WriteString(resp.Body + "\n")

	return sb.String()
}

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonAlnum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "response"
	}
	return s
}

func sortedKeys(h http.Header) []string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
