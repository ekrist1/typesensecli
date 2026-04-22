package tui

import "github.com/charmbracelet/lipgloss"

var (
	ColorAccent    = lipgloss.Color("39")   // cyan-blue
	ColorMuted     = lipgloss.Color("244")  // grey
	ColorError     = lipgloss.Color("203")  // red
	ColorOK        = lipgloss.Color("42")   // green

	TabActive   = lipgloss.NewStyle().Bold(true).Foreground(ColorAccent).Padding(0, 1)
	TabInactive = lipgloss.NewStyle().Foreground(ColorMuted).Padding(0, 1)
	Border      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	Footer      = lipgloss.NewStyle().Foreground(ColorMuted)
	ErrorLine   = lipgloss.NewStyle().Foreground(ColorError)
	OKLine      = lipgloss.NewStyle().Foreground(ColorOK)
)
