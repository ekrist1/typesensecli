// Package components holds reusable Bubble Tea subcomponents.
package components

import (
	"bytes"
	"encoding/json"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type JSONView struct {
	vp viewport.Model
}

func NewJSONView(width, height int) JSONView {
	return JSONView{vp: viewport.New(width, height)}
}

func (j *JSONView) SetSize(w, h int) {
	j.vp.Width = w
	j.vp.Height = h
}

// SetContent pretty-prints JSON bytes. Falls back to raw text if invalid.
func (j *JSONView) SetContent(body []byte) {
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, body, "", "  "); err == nil {
		j.vp.SetContent(pretty.String())
	} else {
		j.vp.SetContent(string(body))
	}
}

func (j *JSONView) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	j.vp, cmd = j.vp.Update(msg)
	return cmd
}

func (j *JSONView) View() string { return j.vp.View() }
