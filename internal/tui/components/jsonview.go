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
	j.vp.Width = max(0, w)
	j.vp.Height = max(0, h)
	j.vp.SetYOffset(j.vp.YOffset)
	j.vp.SetXOffset(0)
}

// SetContent pretty-prints JSON bytes. Falls back to raw text if invalid.
func (j *JSONView) SetContent(body []byte) {
	content := string(body)
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, body, "", "  "); err == nil {
		content = pretty.String()
	}
	j.vp.SetContent(content)
	j.vp.GotoTop()
	j.vp.SetXOffset(0)
}

func (j *JSONView) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	j.vp, cmd = j.vp.Update(msg)
	return cmd
}

func (j *JSONView) PageDown() { j.vp.PageDown() }

func (j *JSONView) PageUp() { j.vp.PageUp() }

func (j *JSONView) GotoTop() {
	j.vp.GotoTop()
	j.vp.SetXOffset(0)
}

func (j *JSONView) GotoBottom() { j.vp.GotoBottom() }

func (j *JSONView) View() string { return j.vp.View() }
