// Package screens contains the tab-level Bubble Tea models.
package screens

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/config"
)

type SetupDoneMsg struct {
	Cfg config.Config
}

type Setup struct {
	url    textinput.Model
	apiKey textinput.Model
	focus  int // 0 = url, 1 = apiKey
	err    string
}

func NewSetup(initial config.Config) Setup {
	url := textinput.New()
	url.Placeholder = "http://localhost:8108"
	url.SetValue(initial.URL)
	url.Focus()

	key := textinput.New()
	key.Placeholder = "API key"
	key.SetValue(initial.APIKey)
	key.EchoMode = textinput.EchoPassword

	return Setup{url: url, apiKey: key, focus: 0}
}

func (s Setup) Init() tea.Cmd { return textinput.Blink }

func (s Setup) Update(msg tea.Msg) (Setup, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "tab", "shift+tab":
			s.focus = 1 - s.focus
			if s.focus == 0 {
				s.url.Focus()
				s.apiKey.Blur()
			} else {
				s.apiKey.Focus()
				s.url.Blur()
			}
			return s, nil
		case "enter":
			if s.url.Value() == "" || s.apiKey.Value() == "" {
				s.err = "both fields are required"
				return s, nil
			}
			cfg := config.Config{URL: s.url.Value(), APIKey: s.apiKey.Value()}
			return s, func() tea.Msg { return SetupDoneMsg{Cfg: cfg} }
		}
	}
	var cmd tea.Cmd
	if s.focus == 0 {
		s.url, cmd = s.url.Update(msg)
	} else {
		s.apiKey, cmd = s.apiKey.Update(msg)
	}
	return s, cmd
}

func (s Setup) View() string {
	v := "clisense — Typesense connection\n\n"
	v += "URL:     " + s.url.View() + "\n"
	v += "API key: " + s.apiKey.View() + "\n\n"
	if s.err != "" {
		v += s.err + "\n"
	}
	v += "Tab switch · Enter save · Ctrl+C quit"
	return v
}
