package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type TabBar struct {
	Titles []string
	Active int
	Style  struct {
		Active   lipgloss.Style
		Inactive lipgloss.Style
	}
}

func (t TabBar) View() string {
	var parts []string
	for i, title := range t.Titles {
		label := title
		if i == t.Active {
			parts = append(parts, t.Style.Active.Render(label))
		} else {
			parts = append(parts, t.Style.Inactive.Render(label))
		}
	}
	return strings.Join(parts, " ")
}
