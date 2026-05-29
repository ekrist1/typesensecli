package theme

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// KeyHints renders "key label · key label" with key glyphs in Accent and labels
// in Muted. Each pair is [2]string{key, label}.
func KeyHints(pairs ...[2]string) string {
	keyStyle := lipgloss.NewStyle().Foreground(Accent)
	parts := make([]string, 0, len(pairs))
	for _, p := range pairs {
		parts = append(parts, keyStyle.Render(p[0])+" "+StatusBar.Render(p[1]))
	}
	return strings.Join(parts, StatusBar.Render(" · "))
}

// Status colors a status segment: "ok" green, "err" red, anything else muted.
func Status(kind, msg string) string {
	switch kind {
	case "ok":
		return StatusOK.Render(msg)
	case "err":
		return StatusErr.Render(msg)
	default:
		return StatusBar.Render(msg)
	}
}
