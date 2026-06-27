package httpclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/luisalfredotejeda/texman/internal/collection"
)

// Response holds the result of an executed HTTP request.
type Response struct {
	StatusCode int
	Status     string
	Headers    http.Header
	Body       string
	Duration   time.Duration
}

// ResponseMsg is the Bubble Tea message delivered when a request completes.
type ResponseMsg struct {
	Resp *Response
}

// ErrMsg is the Bubble Tea message delivered when a request fails.
type ErrMsg struct {
	Err error
}

// Execute returns a Bubble Tea Cmd that fires an HTTP request in a goroutine.
// On success it delivers ResponseMsg; on failure, ErrMsg.
func Execute(req collection.Request) tea.Cmd {
	return func() tea.Msg {
		start := time.Now()

		var bodyReader io.Reader
		if req.Body != "" {
			bodyReader = strings.NewReader(req.Body)
		}

		httpReq, err := http.NewRequest(req.Method, req.URL, bodyReader)
		if err != nil {
			return ErrMsg{Err: fmt.Errorf("build request: %w", err)}
		}

		for k, v := range req.Headers {
			httpReq.Header.Set(k, v)
		}

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(httpReq)
		if err != nil {
			return ErrMsg{Err: fmt.Errorf("execute: %w", err)}
		}
		defer resp.Body.Close()

		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			return ErrMsg{Err: fmt.Errorf("read body: %w", err)}
		}

		return ResponseMsg{
			Resp: &Response{
				StatusCode: resp.StatusCode,
				Status:     resp.Status,
				Headers:    resp.Header,
				Body:       prettyJSON(raw),
				Duration:   time.Since(start),
			},
		}
	}
}

// prettyJSON returns pretty-printed JSON if data is valid JSON, otherwise
// returns the raw string. An empty slice returns the literal "(empty)".
func prettyJSON(data []byte) string {
	if len(data) == 0 {
		return "(empty)"
	}
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return string(data)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return string(data)
	}
	return string(out)
}
