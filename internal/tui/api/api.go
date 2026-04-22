package api

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/client"
)

// SuccessMsg is emitted when an API call returns a 2xx status.
type SuccessMsg struct {
	Tag    string // screen-provided identifier so the right screen reacts
	Status int
	Body   []byte
}

// ErrorMsg is emitted on non-2xx status or transport failure.
type ErrorMsg struct {
	Tag    string
	Status int    // 0 if transport failure
	Body   []byte // may be empty
	Err    error  // nil for HTTP status errors, non-nil for transport
}

// DoRequest builds a tea.Cmd that performs an API call and returns either
// SuccessMsg or ErrorMsg (tagged so only the originating screen reacts).
func DoRequest(c *client.Client, tag, method, path string, body []byte) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		respBody, status, err := c.Do(ctx, method, path, body)
		if err != nil {
			return ErrorMsg{Tag: tag, Status: 0, Err: err}
		}
		if status >= 200 && status < 300 {
			return SuccessMsg{Tag: tag, Status: status, Body: respBody}
		}
		return ErrorMsg{Tag: tag, Status: status, Body: respBody}
	}
}
