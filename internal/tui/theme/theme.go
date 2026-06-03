// Package theme holds the adaptive color palette, shared lipgloss styles, and
// pure render helpers used across the TUI. It depends only on lipgloss so both
// the tui and screens packages can import it without an import cycle.
package theme

import "github.com/charmbracelet/lipgloss"

// Adaptive palette — Light/Dark pairs so the UI reads well on any terminal.
var (
	Accent  = lipgloss.AdaptiveColor{Light: "32", Dark: "39"}
	Muted   = lipgloss.AdaptiveColor{Light: "245", Dark: "244"}
	Subtle  = lipgloss.AdaptiveColor{Light: "250", Dark: "240"}
	Success = lipgloss.AdaptiveColor{Light: "28", Dark: "42"}
	Error   = lipgloss.AdaptiveColor{Light: "160", Dark: "203"}
	Warning = lipgloss.AdaptiveColor{Light: "166", Dark: "214"}
)

// Shared styles.
var (
	HeaderTitle = lipgloss.NewStyle().Bold(true).Foreground(Accent)
	HeaderMeta  = lipgloss.NewStyle().Foreground(Muted)
	TabActive   = lipgloss.NewStyle().Bold(true).Foreground(Accent).Padding(0, 1)
	TabInactive = lipgloss.NewStyle().Foreground(Muted).Padding(0, 1)
	TabNumber   = lipgloss.NewStyle().Foreground(Subtle)
	PanelTitle  = lipgloss.NewStyle().Bold(true).Foreground(Accent)
	StatusBar   = lipgloss.NewStyle().Foreground(Muted)
	StatusOK    = lipgloss.NewStyle().Foreground(Success)
	StatusErr   = lipgloss.NewStyle().Foreground(Error)
)
