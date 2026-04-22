package components

import (
	"encoding/json"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

type JSONEditorSubmitMsg struct {
	Body []byte
}

type JSONEditorCancelMsg struct{}

type JSONEditor struct {
	ta    textarea.Model
	err   string
	title string
}

func NewJSONEditor(title string, initial []byte, width, height int) JSONEditor {
	ta := textarea.New()
	ta.SetValue(string(initial))
	ta.SetWidth(width)
	ta.SetHeight(height - 4) // leave room for title + footer
	ta.Focus()
	return JSONEditor{ta: ta, title: title}
}

func (e *JSONEditor) SetSize(w, h int) {
	e.ta.SetWidth(w)
	e.ta.SetHeight(h - 4)
}

func (e *JSONEditor) SetError(msg string) { e.err = msg }

// Update handles key input. Returns a tea.Cmd if a submit or cancel occurs
// (caller receives JSONEditorSubmitMsg / JSONEditorCancelMsg as tea.Msg).
func (e *JSONEditor) Update(msg tea.Msg) tea.Cmd {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			return func() tea.Msg { return JSONEditorCancelMsg{} }
		case "ctrl+s":
			body := []byte(e.ta.Value())
			var v any
			if err := json.Unmarshal(body, &v); err != nil {
				e.err = "invalid JSON: " + err.Error()
				return nil
			}
			e.err = ""
			return func() tea.Msg { return JSONEditorSubmitMsg{Body: body} }
		}
	}
	var cmd tea.Cmd
	e.ta, cmd = e.ta.Update(msg)
	return cmd
}

func (e *JSONEditor) View() string {
	footer := "Ctrl+S save · Esc cancel"
	if e.err != "" {
		footer = e.err
	}
	return e.title + "\n" + e.ta.View() + "\n" + footer
}
