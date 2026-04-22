package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/client"
)

// DoRequest builds a tea.Cmd that performs an API call and returns either
// APISuccessMsg or APIErrorMsg (tagged so only the originating screen reacts).
func DoRequest(c *client.Client, tag, method, path string, body []byte) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		respBody, status, err := c.Do(ctx, method, path, body)
		if err != nil {
			return APIErrorMsg{Tag: tag, Status: 0, Err: err}
		}
		if status >= 200 && status < 300 {
			return APISuccessMsg{Tag: tag, Status: status, Body: respBody}
		}
		return APIErrorMsg{Tag: tag, Status: status, Body: respBody}
	}
}
