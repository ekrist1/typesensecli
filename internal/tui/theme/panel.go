package theme

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Panel renders content inside a rounded border of exactly Width x Height cells,
// with Title embedded in the top border. Focused panels use the Accent border
// color; others use Subtle.
type Panel struct {
	Title         string
	Width, Height int
	Focused       bool
}

func (p Panel) InnerWidth() int  { return max(0, p.Width-2) }
func (p Panel) InnerHeight() int { return max(0, p.Height-2) }

func (p Panel) Render(content string) string {
	if p.Width < 3 || p.Height < 3 {
		return content
	}
	color := lipgloss.TerminalColor(Subtle)
	if p.Focused {
		color = Accent
	}

	inner := lipgloss.NewStyle().
		Width(p.InnerWidth()).
		Height(p.InnerHeight()).
		MaxWidth(p.InnerWidth()).
		MaxHeight(p.InnerHeight()).
		Render(content)

	boxed := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Render(inner)

	lines := strings.Split(boxed, "\n")
	if len(lines) > 0 {
		lines[0] = p.topBorder(color)
	}
	return strings.Join(lines, "\n")
}

// topBorder builds the top edge "╭─ Title ───╮" at exactly InnerWidth cells
// between the corners.
func (p Panel) topBorder(color lipgloss.TerminalColor) string {
	rb := lipgloss.RoundedBorder()
	inner := p.InnerWidth()

	var mid string
	if p.Title == "" || inner < 4 {
		mid = strings.Repeat(rb.Top, inner)
	} else {
		label := " " + p.Title + " "
		if lipgloss.Width(label) > inner-1 {
			// Trim by display width (not rune count) so double-width runes
			// can't overflow the panel. Budget = inner minus the leading "─".
			budget := inner - 1
			r := []rune(p.Title)
			for len(r) > 0 && lipgloss.Width(" "+string(r)+" ") > budget {
				r = r[:len(r)-1]
			}
			label = " " + string(r) + " "
		}
		fill := inner - 1 - lipgloss.Width(label)
		if fill < 0 {
			fill = 0
		}
		mid = rb.Top + label + strings.Repeat(rb.Top, fill)
		if pad := inner - lipgloss.Width(mid); pad > 0 {
			mid += strings.Repeat(rb.Top, pad)
		}
	}
	line := rb.TopLeft + mid + rb.TopRight
	return lipgloss.NewStyle().Foreground(color).Render(line)
}
