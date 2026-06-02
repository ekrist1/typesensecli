// Package screens contains the tab-level Bubble Tea models.
package screens

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/config"
	"clisense/internal/tui/theme"
)

type SetupDoneMsg struct {
	Cfg config.Config
}

type Setup struct {
	cfg     config.Config
	cluster textinput.Model
	url     textinput.Model
	apiKey  textinput.Model
	focus   int // 0 = cluster, 1 = url, 2 = apiKey
	err     string
}

func NewSetup(initial config.Config) Setup {
	active := initial.ActiveClusterName()
	if active == "" {
		active = "default"
	}
	cluster := textinput.New()
	cluster.Placeholder = "default"
	cluster.SetValue(active)

	url := textinput.New()
	url.Placeholder = "http://localhost:8108"
	url.SetValue(initial.URL())

	key := textinput.New()
	key.Placeholder = "API key"
	key.SetValue(initial.APIKey())
	key.EchoMode = textinput.EchoPassword

	focus := 1
	if len(initial.ClusterNames()) > 1 {
		focus = 0
	}
	s := Setup{cfg: initial, cluster: cluster, url: url, apiKey: key, focus: focus}
	s.focusInput()
	return s
}

func (s Setup) Init() tea.Cmd { return textinput.Blink }

func (s *Setup) SetError(msg string) { s.err = msg }

func (s Setup) Update(msg tea.Msg) (Setup, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "tab", "down":
			s.focus = (s.focus + 1) % 3
			s.focusInput()
			return s, nil
		case "shift+tab", "up":
			s.focus = (s.focus + 2) % 3
			s.focusInput()
			return s, nil
		case "enter":
			if s.cluster.Value() == "" || s.url.Value() == "" || s.apiKey.Value() == "" {
				s.err = "cluster, URL, and API key are required"
				return s, nil
			}
			cfg := s.cfg
			cfg.SetConnection(s.cluster.Value(), s.url.Value(), s.apiKey.Value())
			return s, func() tea.Msg { return SetupDoneMsg{Cfg: cfg} }
		}
	}
	var cmd tea.Cmd
	switch s.focus {
	case 0:
		s.cluster, cmd = s.cluster.Update(msg)
	case 1:
		s.url, cmd = s.url.Update(msg)
	case 2:
		s.apiKey, cmd = s.apiKey.Update(msg)
	}
	return s, cmd
}

func (s *Setup) focusInput() {
	s.cluster.Blur()
	s.url.Blur()
	s.apiKey.Blur()
	switch s.focus {
	case 0:
		s.cluster.Focus()
	case 1:
		s.url.Focus()
	case 2:
		s.apiKey.Focus()
	}
}

func (s Setup) View() string {
	v := theme.HeaderTitle.Render("clisense") + theme.HeaderMeta.Render(" — Typesense connection") + "\n\n"
	v += theme.HeaderMeta.Render("Cluster: ") + s.cluster.View() + "\n"
	v += theme.HeaderMeta.Render("URL:     ") + s.url.View() + "\n"
	v += theme.HeaderMeta.Render("API key: ") + s.apiKey.View() + "\n\n"
	if s.err != "" {
		v += theme.StatusErr.Render(s.err) + "\n"
	}
	v += theme.KeyHints(
		[2]string{"Tab/↑↓", "field"}, [2]string{"Enter", "save"}, [2]string{"Ctrl+C", "quit"},
	)
	return v
}
