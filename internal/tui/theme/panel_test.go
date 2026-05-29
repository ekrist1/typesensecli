package theme

import (
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestMain(m *testing.M) {
	// Force color output so border-color tests pass without a real TTY.
	lipgloss.SetColorProfile(termenv.TrueColor)
	os.Exit(m.Run())
}

func TestPanelExactWidthAndHeight(t *testing.T) {
	out := Panel{Title: "Schema", Width: 30, Height: 6}.Render("hello\nworld")
	if w := lipgloss.Width(out); w != 30 {
		t.Fatalf("width = %d, want 30", w)
	}
	if lines := strings.Count(out, "\n") + 1; lines != 6 {
		t.Fatalf("height = %d lines, want 6", lines)
	}
}

func TestPanelShowsTitle(t *testing.T) {
	out := Panel{Title: "Schema", Width: 30, Height: 4}.Render("x")
	first := strings.SplitN(out, "\n", 2)[0]
	if !strings.Contains(first, "Schema") {
		t.Fatalf("top border %q missing title", first)
	}
}

func TestPanelFocusedDiffersFromSubtle(t *testing.T) {
	focused := Panel{Title: "T", Width: 20, Height: 4, Focused: true}.Render("x")
	subtle := Panel{Title: "T", Width: 20, Height: 4, Focused: false}.Render("x")
	if focused == subtle {
		t.Fatal("focused and unfocused panels render identically (border color not applied)")
	}
}

func TestPanelDegenerateSizeReturnsContent(t *testing.T) {
	if got := (Panel{Width: 1, Height: 1}).Render("abc"); got != "abc" {
		t.Fatalf("degenerate panel = %q, want raw content", got)
	}
}
