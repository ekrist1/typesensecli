package screens

import (
	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/config"
)

type Settings struct {
	setup Setup
}

func NewSettings(cur config.Config) Settings { return Settings{setup: NewSetup(cur)} }

func (s Settings) Init() tea.Cmd { return s.setup.Init() }

func (s Settings) Update(msg tea.Msg) (Settings, tea.Cmd) {
	var cmd tea.Cmd
	s.setup, cmd = s.setup.Update(msg)
	return s, cmd
}

func (s Settings) View() string { return s.setup.View() }
