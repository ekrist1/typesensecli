package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	tabGap   = 1 // cells between adjacent tabs (the Join separator)
	tabArrow = 2 // cells reserved for a "‹ " or " ›" overflow indicator
)

type TabBar struct {
	Titles []string
	Active int
	Width  int // total cells available; 0 means render every tab unbounded
	Style  struct {
		Active   lipgloss.Style
		Inactive lipgloss.Style
	}
}

func (t TabBar) View() string {
	n := len(t.Titles)
	if n == 0 {
		return ""
	}

	render := func(i int) string {
		if i == t.Active {
			return t.Style.Active.Render(t.Titles[i])
		}
		return t.Style.Inactive.Render(t.Titles[i])
	}

	// Unbounded: keep the original behaviour.
	if t.Width <= 0 {
		parts := make([]string, n)
		for i := range t.Titles {
			parts[i] = render(i)
		}
		return strings.Join(parts, " ")
	}

	widths := make([]int, n)
	for i := range t.Titles {
		widths[i] = lipgloss.Width(t.Style.Inactive.Render(t.Titles[i]))
	}

	lo, hi := tabWindow(widths, t.Active, t.Width)

	parts := make([]string, 0, hi-lo+1)
	for i := lo; i <= hi; i++ {
		parts = append(parts, render(i))
	}
	out := strings.Join(parts, " ")

	// Muted, padding-free arrows borrow the inactive colour.
	arrow := t.Style.Inactive.Padding(0, 0)
	if lo > 0 {
		out = arrow.Render("‹") + " " + out
	}
	if hi < n-1 {
		out = out + " " + arrow.Render("›")
	}
	return out
}

// tabWindow returns the inclusive [lo,hi] range of tab indices to display so
// that the active tab is always visible and the rendered row (including overflow
// arrows) fits within totalWidth. It expands outward from the active tab,
// balancing left/right growth, and stops when neither side can add a tab.
func tabWindow(widths []int, active, totalWidth int) (lo, hi int) {
	n := len(widths)
	lo, hi = active, active
	used := widths[active]

	for {
		leftHidden := lo > 0
		rightHidden := hi < n-1
		reserve := 0
		if leftHidden {
			reserve += tabArrow
		}
		if rightHidden {
			reserve += tabArrow
		}
		avail := totalWidth - reserve - used

		canRight := rightHidden && tabGap+widths[hi+1] <= avail
		canLeft := leftHidden && tabGap+widths[lo-1] <= avail
		if !canRight && !canLeft {
			break
		}

		// Prefer the side closer to the active tab to keep it roughly centred;
		// fall back to whichever side can still grow.
		growRight := canRight && (!canLeft || (hi-active) <= (active-lo))
		if growRight {
			hi++
			used += tabGap + widths[hi]
		} else {
			lo--
			used += tabGap + widths[lo]
		}
	}
	return lo, hi
}

// windowUsedWidth reports the rendered width of the [lo,hi] window over a slice
// of n total tabs, including gaps and any overflow arrows.
func windowUsedWidth(widths []int, lo, hi, n int) int {
	used := 0
	for i := lo; i <= hi; i++ {
		used += widths[i]
		if i > lo {
			used += tabGap
		}
	}
	if lo > 0 {
		used += tabArrow
	}
	if hi < n-1 {
		used += tabArrow
	}
	return used
}
