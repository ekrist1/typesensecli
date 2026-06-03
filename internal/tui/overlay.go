package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func placeOverlay(x, y int, fg, bg string) string {
	fgLines, fgWidth := overlayLines(fg)
	bgLines, bgWidth := overlayLines(bg)
	fgHeight := len(fgLines)
	bgHeight := len(bgLines)
	if fgWidth == 0 || fgHeight == 0 {
		return bg
	}
	if bgWidth == 0 || bgHeight == 0 {
		return fg
	}

	x = max(0, min(x, max(0, bgWidth-fgWidth)))
	y = max(0, min(y, max(0, bgHeight-fgHeight)))

	var b strings.Builder
	for i, bgLine := range bgLines {
		if i > 0 {
			b.WriteByte('\n')
		}
		if i < y || i >= y+fgHeight {
			b.WriteString(padRight(bgLine, bgWidth))
			continue
		}

		left := ansi.Truncate(bgLine, x, "")
		b.WriteString(padRight(left, x))
		b.WriteString(fgLines[i-y])

		pos := x + ansi.StringWidth(fgLines[i-y])
		right := ansi.TruncateLeft(bgLine, pos, "")
		rightWidth := ansi.StringWidth(right)
		if rightWidth < bgWidth-pos {
			b.WriteString(strings.Repeat(" ", bgWidth-pos-rightWidth))
		}
		b.WriteString(right)
	}
	return b.String()
}

func overlayLines(s string) ([]string, int) {
	lines := strings.Split(s, "\n")
	width := 0
	for _, line := range lines {
		lineWidth := ansi.StringWidth(line)
		width = max(width, lineWidth)
	}
	return lines, width
}

func padRight(s string, width int) string {
	if pad := width - ansi.StringWidth(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}
