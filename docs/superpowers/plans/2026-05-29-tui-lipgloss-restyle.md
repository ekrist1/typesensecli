# TUI lipgloss Restyle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restyle the clisense TUI with lipgloss — titled rounded-border panels, an app header, styled status/footers, and a single adaptive color theme — applied across all screens.

**Architecture:** A new dependency-free `internal/tui/theme` package owns an adaptive palette, shared styles, and pure render helpers (`Panel`, `KeyHints`, `Status`). Both `tui` (app chrome) and `screens` import it (no import cycle, since `theme` imports only lipgloss). Content panes that today are raw `list`/`JSONView` strings get wrapped in `theme.Panel`; the focused pane gets an accent border. Work is phased: foundation → app chrome → Resource → Cluster → Collections/search → wizards/setup. Each phase ends with a green build/test and a committed, working app.

**Tech Stack:** Go 1.25, charmbracelet lipgloss / bubbles / bubbletea.

**Reference (spec):** `docs/superpowers/specs/2026-05-29-tui-lipgloss-restyle-design.md`

---

## Conventions used throughout

- `theme.Panel{Title, Width, Height, Focused}.Render(content)` returns a rounded box of **exactly** `Width × Height`; the inner content area is `Width-2 × Height-2` (`InnerWidth()/InnerHeight()`). Screens size their inner `list`/`JSONView` to the inner dims and render the panel at the outer dims.
- `theme.KeyHints([2]string{"s","search"}, ...)` renders `s search · d delete` with key glyphs in accent, labels muted.
- `theme.Status(kind, msg)` colors a status segment: `"ok"`→green, `"err"`→red, else muted. `statusKind(s)` (in `screens/layout.go`) classifies the existing free-text status strings.
- Existing modal/editor/confirm views keep rendering full-screen (not panel-wrapped), per spec.

---

# Phase 1 — theme foundation

## File structure (Phase 1)
- Create: `internal/tui/theme/theme.go` — palette + shared styles
- Create: `internal/tui/theme/panel.go` — `Panel` render helper
- Create: `internal/tui/theme/panel_test.go`
- Create: `internal/tui/theme/status.go` — `KeyHints` + `Status`
- Create: `internal/tui/theme/status_test.go`

### Task 1.1: Palette and shared styles

**Files:**
- Create: `internal/tui/theme/theme.go`

- [ ] **Step 1: Create the palette file**

Create `internal/tui/theme/theme.go`:

```go
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
	PanelTitle  = lipgloss.NewStyle().Bold(true).Foreground(Accent)
	StatusBar   = lipgloss.NewStyle().Foreground(Muted)
	StatusOK    = lipgloss.NewStyle().Foreground(Success)
	StatusErr   = lipgloss.NewStyle().Foreground(Error)
)
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/tui/theme/`
Expected: builds with no output (constants only — no test needed for data).

- [ ] **Step 3: Commit**

```bash
git add internal/tui/theme/theme.go
git commit -m "feat(theme): adaptive palette and shared styles"
```

### Task 1.2: Panel render helper

**Files:**
- Create: `internal/tui/theme/panel.go`
- Test: `internal/tui/theme/panel_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/tui/theme/panel_test.go`:

```go
package theme

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/theme/ -run TestPanel -v`
Expected: FAIL — `undefined: Panel`.

- [ ] **Step 3: Implement the Panel helper**

Create `internal/tui/theme/panel.go`:

```go
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
	if p.Width < 2 || p.Height < 2 {
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
			r := []rune(p.Title)
			keep := max(0, inner-3)
			if keep > len(r) {
				keep = len(r)
			}
			label = " " + string(r[:keep]) + " "
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/theme/ -run TestPanel -v`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/tui/theme/panel.go internal/tui/theme/panel_test.go
git commit -m "feat(theme): titled bordered Panel render helper"
```

### Task 1.3: KeyHints and Status helpers

**Files:**
- Create: `internal/tui/theme/status.go`
- Test: `internal/tui/theme/status_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/tui/theme/status_test.go`:

```go
package theme

import (
	"strings"
	"testing"
)

func TestKeyHintsContainsKeysAndLabels(t *testing.T) {
	out := KeyHints([2]string{"s", "search"}, [2]string{"d", "delete"})
	for _, want := range []string{"s", "search", "d", "delete", "·"} {
		if !strings.Contains(out, want) {
			t.Fatalf("KeyHints output missing %q: %q", want, out)
		}
	}
}

func TestStatusColorsDiffer(t *testing.T) {
	ok := Status("ok", "done")
	err := Status("err", "done")
	if ok == err {
		t.Fatal("ok and err statuses render identically")
	}
	if !strings.Contains(ok, "done") || !strings.Contains(err, "done") {
		t.Fatal("status text not preserved")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/theme/ -run "TestKeyHints|TestStatus" -v`
Expected: FAIL — `undefined: KeyHints`, `Status`.

- [ ] **Step 3: Implement the helpers**

Create `internal/tui/theme/status.go`:

```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/theme/ -run "TestKeyHints|TestStatus" -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/tui/theme/status.go internal/tui/theme/status_test.go
git commit -m "feat(theme): KeyHints and Status helpers"
```

---

# Phase 2 — App chrome

## File structure (Phase 2)
- Modify: `internal/tui/app.go` — header line, theme tab styles, `-3`→`-4` body offset
- Delete: `internal/tui/styles.go` — dead styles replaced by `theme`

### Task 2.1: Header bar, theme tab styles, body offset

**Files:**
- Modify: `internal/tui/app.go`
- Delete: `internal/tui/styles.go`

- [ ] **Step 1: Add the theme import**

In `internal/tui/app.go`, add to the import block (after `"clisense/internal/tui/components"`):

```go
	"clisense/internal/tui/theme"
```

- [ ] **Step 2: Switch tab styles to theme and add the header in View()**

In `internal/tui/app.go`, replace the start of `View()`:

```go
	bar := components.TabBar{Titles: a.tabs, Active: a.active, Width: a.width}
	bar.Style.Active = TabActive
	bar.Style.Inactive = TabInactive
```

with:

```go
	bar := components.TabBar{Titles: a.tabs, Active: a.active, Width: a.width}
	bar.Style.Active = theme.TabActive
	bar.Style.Inactive = theme.TabInactive
	header := theme.HeaderTitle.Render("clisense") + theme.HeaderMeta.Render(" — "+a.cfg.URL)
```

- [ ] **Step 3: Prepend the header to the rendered output**

In `internal/tui/app.go`, change the final return of `View()`:

```go
	return bar.View() + "\n\n" + body
```

to:

```go
	return header + "\n" + bar.View() + "\n\n" + body
```

- [ ] **Step 4: Increase the body height offset from 3 to 4**

The header adds one row. In `internal/tui/app.go`, change every `a.height-3` to `a.height-4`. These occur in `buildTabs()` (11 constructor calls) and in the `tea.WindowSizeMsg` case (11 `SetSize` calls). Use a global replace:

```bash
sed -i 's/a\.height-3/a.height-4/g' internal/tui/app.go
```

Verify exactly 22 sites changed:

```bash
grep -c "a.height-4" internal/tui/app.go   # expect 22
grep -c "a.height-3" internal/tui/app.go   # expect 0
```

- [ ] **Step 5: Delete the dead styles file**

`internal/tui/styles.go` defined `TabActive`/`TabInactive` (now in `theme`) and `Border`/`Footer`/`ErrorLine`/`OKLine`/`Color*` (unused). Remove it:

```bash
git rm internal/tui/styles.go
```

- [ ] **Step 6: Build and verify**

Run: `go build ./... && go test ./...`
Expected: clean build (no `undefined: TabActive` — `View()` now uses `theme.TabActive`), all tests pass.

- [ ] **Step 7: Visual check**

Run:

```bash
cat > /tmp/chromecheck.go <<'EOF'
package main

import (
	"fmt"
	"clisense/internal/tui/theme"
)

func main() {
	fmt.Println(theme.HeaderTitle.Render("clisense") + theme.HeaderMeta.Render(" — http://localhost:8108"))
	fmt.Println(theme.KeyHints([2]string{"s", "search"}, [2]string{"d", "delete"}, [2]string{"q", "quit"}))
}
EOF
cp /tmp/chromecheck.go ./chromecheck_main.go && go run ./chromecheck_main.go; rm -f ./chromecheck_main.go /tmp/chromecheck.go
```

Expected: a colored header line and a colored key-hint line print without error.

- [ ] **Step 8: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat(tui): app header bar and theme-driven tab styles"
```

---

# Phase 3 — Resource screen panels

Covers Aliases, NL Models, Stopwords, Presets, Analytics, API Keys (and, indirectly, the inner Resource of Curations/Synonyms/Conversations).

## File structure (Phase 3)
- Modify: `internal/tui/screens/resource.go` — `FooterHints` field, panel fields, panel-wrapped `View`, sized inner views
- Modify: `internal/tui/screens/layout.go` — add `statusKind`
- Create: `internal/tui/screens/layout_test.go` — `statusKind` tests

### Task 3.1: statusKind helper

**Files:**
- Modify: `internal/tui/screens/layout.go`
- Test: `internal/tui/screens/layout_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/tui/screens/layout_test.go`:

```go
package screens

import "testing"

func TestStatusKind(t *testing.T) {
	cases := map[string]string{
		"":                   "",
		"OK":                 "ok",
		"5 items":            "",
		"parse error: bad":   "err",
		"network error: x":   "err",
		"HTTP 422: bad body": "err",
	}
	for in, want := range cases {
		if got := statusKind(in); got != want {
			t.Fatalf("statusKind(%q) = %q, want %q", in, got, want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/screens/ -run TestStatusKind -v`
Expected: FAIL — `undefined: statusKind`.

- [ ] **Step 3: Implement statusKind**

In `internal/tui/screens/layout.go`, add the `strings` import and the function:

```go
import "strings"

// statusKind classifies a free-text status string for theme.Status coloring.
func statusKind(s string) string {
	switch {
	case s == "":
		return ""
	case strings.HasPrefix(s, "OK"):
		return "ok"
	case strings.Contains(strings.ToLower(s), "error") || strings.HasPrefix(s, "HTTP "):
		return "err"
	default:
		return ""
	}
}
```

(The file currently has no import block — add `import "strings"` near the top, below `package screens`.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/screens/ -run TestStatusKind -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/screens/layout.go internal/tui/screens/layout_test.go
git commit -m "feat(screens): statusKind classifier for status coloring"
```

### Task 3.2: FooterHints strategy field + panel-wrapped Resource

**Files:**
- Modify: `internal/tui/screens/resource.go`

- [ ] **Step 1: Add the theme import and FooterHints field**

In `internal/tui/screens/resource.go`, add `"clisense/internal/tui/theme"` to the imports. Then add a field to `ResourceStrategy` (after `Template []byte`):

```go
	// FooterHints overrides the default footer key hints. Each entry is
	// [2]string{key, label}. When nil, a default CRUD hint set is used.
	FooterHints [][2]string
```

- [ ] **Step 2: Add panel dimension fields to Resource**

In the `Resource` struct, add (after `rawList []byte`):

```go
	listW, detailW, paneH int
```

- [ ] **Step 3: Size inner views and store outer dims in NewResource**

In `NewResource`, replace:

```go
	listWidth, detailWidth, paneHeight := splitPaneSizes(w, h)
	l := list.New(nil, list.NewDefaultDelegate(), listWidth, paneHeight)
	l.Title = s.TabName
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	detail := components.NewJSONView(detailWidth, paneHeight)
	detail.SetContent([]byte("No item selected."))
	return Resource{
		c:      c,
		strat:  s,
		list:   l,
		detail: detail,
		width:  w, height: h,
	}
```

with:

```go
	listWidth, detailWidth, paneHeight := splitPaneSizes(w, h)
	l := list.New(nil, list.NewDefaultDelegate(), max(1, listWidth-2), max(1, paneHeight-2))
	l.Title = s.TabName
	l.SetShowTitle(false) // the panel border shows the title instead
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	detail := components.NewJSONView(max(1, detailWidth-2), max(1, paneHeight-2))
	detail.SetContent([]byte("No item selected."))
	return Resource{
		c:      c,
		strat:  s,
		list:   l,
		detail: detail,
		width:  w, height: h,
		listW: listWidth, detailW: detailWidth, paneH: paneHeight,
	}
```

- [ ] **Step 4: Size inner views and store outer dims in SetSize**

In `internal/tui/screens/resource.go`, replace the body of `SetSize`:

```go
func (r *Resource) SetSize(w, h int) {
	r.width, r.height = w, h
	listWidth, detailWidth, paneHeight := splitPaneSizes(w, h)
	r.list.SetSize(listWidth, paneHeight)
	r.detail.SetSize(detailWidth, paneHeight)
	if r.editor != nil {
		r.editor.SetSize(w, h)
	}
}
```

with:

```go
func (r *Resource) SetSize(w, h int) {
	r.width, r.height = w, h
	listWidth, detailWidth, paneHeight := splitPaneSizes(w, h)
	r.listW, r.detailW, r.paneH = listWidth, detailWidth, paneHeight
	r.list.SetSize(max(1, listWidth-2), max(1, paneHeight-2))
	r.detail.SetSize(max(1, detailWidth-2), max(1, paneHeight-2))
	if r.editor != nil {
		r.editor.SetSize(w, h)
	}
}
```

- [ ] **Step 5: Rewrite View to use panels, KeyHints, and Status**

In `internal/tui/screens/resource.go`, replace the bottom portion of `View()` (the part after the `confirm` check):

```go
	footer := "/ filter · n new · e edit · d delete · PgUp/PgDn scroll JSON · r refresh · q quit"
	if r.status != "" {
		footer = r.status + " · " + footer
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, r.list.View(), "  ", r.detail.View())
	return body + "\n" + footer
}
```

with:

```go
	hints := r.strat.FooterHints
	if hints == nil {
		hints = [][2]string{
			{"/", "filter"}, {"n", "new"}, {"e", "edit"}, {"d", "delete"},
			{"PgUp/PgDn", "scroll"}, {"r", "refresh"}, {"q", "quit"},
		}
	}
	footer := theme.KeyHints(hints...)
	if r.status != "" {
		footer = theme.Status(statusKind(r.status), r.status) + theme.StatusBar.Render(" · ") + footer
	}
	listPanel := theme.Panel{Title: r.strat.TabName, Width: r.listW, Height: r.paneH, Focused: true}
	detailPanel := theme.Panel{Title: "Detail", Width: r.detailW, Height: r.paneH}
	body := lipgloss.JoinHorizontal(lipgloss.Top, listPanel.Render(r.list.View()), " ", detailPanel.Render(r.detail.View()))
	return body + "\n" + footer
}
```

- [ ] **Step 6: Build, test, and visually verify**

Run: `go build ./... && go test ./...`
Expected: clean build, all tests pass (including the existing `TestResourceKeepsEditorOpenOnSaveError`).

Visual check:

```bash
cat > /tmp/rescheck.go <<'EOF'
package main

import (
	"fmt"
	"clisense/internal/tui/screens"
	"clisense/internal/tui/api"
)

func main() {
	r := screens.NewAnalytics(nil, 100, 30)
	r, _ = r.Update(api.SuccessMsg{Tag: "Analytics:list", Status: 200, Body: []byte(`[{"name":"top","type":"popular_queries"},{"name":"misses","type":"nohits_queries"}]`)})
	fmt.Println(r.View())
}
EOF
cp /tmp/rescheck.go ./rescheck_main.go && go run ./rescheck_main.go; rm -f ./rescheck_main.go /tmp/rescheck.go
```

Expected: two bordered panels side by side ("Analytics" list focused/accent, "Detail" subtle) and a colored footer; output fits 100 columns.

- [ ] **Step 7: Commit**

```bash
git add internal/tui/screens/resource.go
git commit -m "feat(screens): bordered panels and styled footer for Resource"
```

---

# Phase 4 — Cluster screen panels

## File structure (Phase 4)
- Modify: `internal/tui/screens/cluster.go` — panel dims, panel-wrapped `View`, sized inner views, styled footer

### Task 4.1: Panel-wrap the Cluster dashboard

**Files:**
- Modify: `internal/tui/screens/cluster.go`

- [ ] **Step 1: Add theme import and panel dimension fields**

In `internal/tui/screens/cluster.go`, add `"clisense/internal/tui/theme"` to the imports. Add fields to the `Cluster` struct (after `width, height int`):

```go
	listW, detailW, paneH int
```

- [ ] **Step 2: Size inner views and store outer dims in NewCluster**

In `NewCluster`, replace:

```go
	listWidth, detailWidth, paneHeight := splitPaneSizes(w, h)
	items := make([]list.Item, 0, len(panels))
	for _, p := range panels {
		items = append(items, clusterPanelItem{title: p.title})
	}
	l := list.New(items, list.NewDefaultDelegate(), listWidth, paneHeight)
	l.Title = "Cluster"
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)

	detail := components.NewJSONView(detailWidth, paneHeight)
	detail.SetContent([]byte("Press r to load cluster info."))

	return Cluster{
		c:        c,
		list:     l,
		detail:   detail,
		panels:   panels,
		ops:      ops,
		payloads: map[string][]byte{},
		width:    w, height: h,
	}
```

with:

```go
	listWidth, detailWidth, paneHeight := splitPaneSizes(w, h)
	items := make([]list.Item, 0, len(panels))
	for _, p := range panels {
		items = append(items, clusterPanelItem{title: p.title})
	}
	l := list.New(items, list.NewDefaultDelegate(), max(1, listWidth-2), max(1, paneHeight-2))
	l.Title = "Cluster"
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)

	detail := components.NewJSONView(max(1, detailWidth-2), max(1, paneHeight-2))
	detail.SetContent([]byte("Press r to load cluster info."))

	return Cluster{
		c:        c,
		list:     l,
		detail:   detail,
		panels:   panels,
		ops:      ops,
		payloads: map[string][]byte{},
		width:    w, height: h,
		listW: listWidth, detailW: detailWidth, paneH: paneHeight,
	}
```

- [ ] **Step 3: Size inner views and store outer dims in SetSize**

In `internal/tui/screens/cluster.go`, replace the body of `SetSize`:

```go
func (s *Cluster) SetSize(w, h int) {
	s.width, s.height = w, h
	listWidth, detailWidth, paneHeight := splitPaneSizes(w, h)
	s.list.SetSize(listWidth, paneHeight)
	s.detail.SetSize(detailWidth, paneHeight)
}
```

with:

```go
func (s *Cluster) SetSize(w, h int) {
	s.width, s.height = w, h
	listWidth, detailWidth, paneHeight := splitPaneSizes(w, h)
	s.listW, s.detailW, s.paneH = listWidth, detailWidth, paneHeight
	s.list.SetSize(max(1, listWidth-2), max(1, paneHeight-2))
	s.detail.SetSize(max(1, detailWidth-2), max(1, paneHeight-2))
}
```

- [ ] **Step 4: Add a helper for the selected panel's title**

In `internal/tui/screens/cluster.go`, add after `selectedPanelTag()`:

```go
func (s Cluster) selectedPanelTitle() string {
	if it, ok := s.list.SelectedItem().(clusterPanelItem); ok {
		return it.title
	}
	return "Detail"
}
```

- [ ] **Step 5: Rewrite View with panels and styled footer**

In `internal/tui/screens/cluster.go`, replace `View()`:

```go
func (s Cluster) View() string {
	if s.confirm != nil {
		return s.confirm.View()
	}
	footer := "r refresh · S snapshot · C compact · V vote · X clear-cache · PgUp/PgDn scroll · q quit"
	if s.status != "" {
		footer = s.status + " · " + footer
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, s.list.View(), "  ", s.detail.View())
	return body + "\n" + footer
}
```

with:

```go
func (s Cluster) View() string {
	if s.confirm != nil {
		return s.confirm.View()
	}
	footer := theme.KeyHints(
		[2]string{"r", "refresh"}, [2]string{"S", "snapshot"}, [2]string{"C", "compact"},
		[2]string{"V", "vote"}, [2]string{"X", "clear-cache"}, [2]string{"PgUp/PgDn", "scroll"}, [2]string{"q", "quit"},
	)
	if s.status != "" {
		footer = theme.Status(statusKind(s.status), s.status) + theme.StatusBar.Render(" · ") + footer
	}
	listPanel := theme.Panel{Title: "Cluster", Width: s.listW, Height: s.paneH, Focused: true}
	detailPanel := theme.Panel{Title: s.selectedPanelTitle(), Width: s.detailW, Height: s.paneH}
	body := lipgloss.JoinHorizontal(lipgloss.Top, listPanel.Render(s.list.View()), " ", detailPanel.Render(s.detail.View()))
	return body + "\n" + footer
}
```

- [ ] **Step 6: Build, test, visual check**

Run: `go build ./... && go test ./...`
Expected: clean, all pass.

Visual check:

```bash
cat > /tmp/clcheck.go <<'EOF'
package main

import (
	"fmt"
	"clisense/internal/tui/screens"
	"clisense/internal/tui/api"
)

func main() {
	c := screens.NewCluster(nil, 100, 30)
	c, _ = c.Update(api.SuccessMsg{Tag: "Cluster:health", Status: 200, Body: []byte(`{"status":"ok"}`)})
	fmt.Println(c.View())
}
EOF
cp /tmp/clcheck.go ./clcheck_main.go && go run ./clcheck_main.go; rm -f ./clcheck_main.go /tmp/clcheck.go
```

Expected: "Cluster" panel (focused) + "Health" detail panel, colored footer, fits 100 cols.

- [ ] **Step 7: Commit**

```bash
git add internal/tui/screens/cluster.go
git commit -m "feat(screens): bordered panels and styled footer for Cluster"
```

---

# Phase 5 — Collections and document search

## File structure (Phase 5)
- Modify: `internal/tui/screens/collections.go` — 3 panels, theme table styles, styled footer, sized inner views
- Modify: `internal/tui/screens/collection_search.go` — panel-wrapped results/detail, styled footer

### Task 5.1: Panel-wrap the Collections three-pane view

**Files:**
- Modify: `internal/tui/screens/collections.go`

- [ ] **Step 1: Add theme import and panel dimension fields**

In `internal/tui/screens/collections.go`, add `"clisense/internal/tui/theme"` to the imports. Add fields to the `Collections` struct (after `status string`):

```go
	listW, fieldsW, detailW, paneH int
```

- [ ] **Step 2: Size inner views and store outer dims in NewCollections**

In `NewCollections`, replace:

```go
	listWidth, fieldsWidth, detailWidth, paneHeight := collectionPaneSizes(w, h)
	l := list.New(nil, list.NewDefaultDelegate(), listWidth, paneHeight)
	l.Title = "Collections *"
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)

	tbl := newCollectionFieldTable(fieldsWidth, paneHeight)
	detail := components.NewJSONView(detailWidth, paneHeight)
	detail.SetContent([]byte("Select a collection to inspect its schema fields."))

	s := Collections{
		c:      c,
		list:   l,
		fields: tbl,
		detail: detail,
		width:  w,
		height: h,
		focus:  focusCollectionList,
	}
```

with:

```go
	listWidth, fieldsWidth, detailWidth, paneHeight := collectionPaneSizes(w, h)
	l := list.New(nil, list.NewDefaultDelegate(), max(1, listWidth-2), max(1, paneHeight-2))
	l.Title = "Collections"
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)

	tbl := newCollectionFieldTable(max(1, fieldsWidth-2), max(1, paneHeight-2))
	detail := components.NewJSONView(max(1, detailWidth-2), max(1, paneHeight-2))
	detail.SetContent([]byte("Select a collection to inspect its schema fields."))

	s := Collections{
		c:      c,
		list:   l,
		fields: tbl,
		detail: detail,
		width:  w,
		height: h,
		focus:  focusCollectionList,
		listW:  listWidth, fieldsW: fieldsWidth, detailW: detailWidth, paneH: paneHeight,
	}
```

- [ ] **Step 3: Size inner views and store outer dims in SetSize**

In `internal/tui/screens/collections.go`, replace the body of `SetSize`:

```go
func (s *Collections) SetSize(w, h int) {
	s.width, s.height = w, h
	listWidth, fieldsWidth, detailWidth, paneHeight := collectionPaneSizes(w, h)
	s.list.SetSize(listWidth, paneHeight)
	s.fields.SetColumns(collectionFieldColumns(fieldsWidth))
	s.fields.SetWidth(fieldsWidth)
	s.fields.SetHeight(paneHeight)
	s.detail.SetSize(detailWidth, paneHeight)
	if s.search != nil {
		s.search.SetSize(w, h)
	}
}
```

with:

```go
func (s *Collections) SetSize(w, h int) {
	s.width, s.height = w, h
	listWidth, fieldsWidth, detailWidth, paneHeight := collectionPaneSizes(w, h)
	s.listW, s.fieldsW, s.detailW, s.paneH = listWidth, fieldsWidth, detailWidth, paneHeight
	s.list.SetSize(max(1, listWidth-2), max(1, paneHeight-2))
	s.fields.SetColumns(collectionFieldColumns(max(1, fieldsWidth-2)))
	s.fields.SetWidth(max(1, fieldsWidth-2))
	s.fields.SetHeight(max(1, paneHeight-2))
	s.detail.SetSize(max(1, detailWidth-2), max(1, paneHeight-2))
	if s.search != nil {
		s.search.SetSize(w, h)
	}
}
```

- [ ] **Step 4: Replace inline table colors with theme**

In `internal/tui/screens/collections.go`, replace `collectionFieldTableStyles`:

```go
func collectionFieldTableStyles(focused bool) table.Styles {
	styles := table.DefaultStyles()
	styles.Header = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	styles.Cell = lipgloss.NewStyle()
	if focused {
		styles.Selected = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	} else {
		styles.Selected = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("244"))
	}
	return styles
}
```

with:

```go
func collectionFieldTableStyles(focused bool) table.Styles {
	styles := table.DefaultStyles()
	styles.Header = lipgloss.NewStyle().Bold(true).Foreground(theme.Accent)
	styles.Cell = lipgloss.NewStyle()
	if focused {
		styles.Selected = lipgloss.NewStyle().Bold(true).Foreground(theme.Accent)
	} else {
		styles.Selected = lipgloss.NewStyle().Bold(true).Foreground(theme.Muted)
	}
	return styles
}
```

- [ ] **Step 5: Rewrite the default View body with panels and styled footer**

In `internal/tui/screens/collections.go`, replace the footer/body portion of `View()` (the code after the `confirm` check, shown earlier at lines 382–393):

```go
	footer := "←/→ switch pane · / filter collections · Ctrl+B/Ctrl+F scroll config · r refresh · q quit"
	if s.focus == focusFieldTable {
		footer = "focus fields · " + footer
	} else {
		footer = "focus collections · " + footer
	}
	footer = "s search documents · d delete collection · " + footer
	if s.status != "" {
		footer = s.status + " · " + footer
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, s.list.View(), "  ", s.fields.View(), "  ", s.detail.View())
	return body + "\n" + footer
}
```

with:

```go
	footer := theme.KeyHints(
		[2]string{"s", "search"}, [2]string{"d", "delete"}, [2]string{"←/→", "switch pane"},
		[2]string{"/", "filter"}, [2]string{"Ctrl+B/Ctrl+F", "scroll"}, [2]string{"r", "refresh"}, [2]string{"q", "quit"},
	)
	if s.status != "" {
		footer = theme.Status(statusKind(s.status), s.status) + theme.StatusBar.Render(" · ") + footer
	}
	listFocused := s.focus == focusCollectionList
	fieldsFocused := s.focus == focusFieldTable
	listPanel := theme.Panel{Title: "Collections", Width: s.listW, Height: s.paneH, Focused: listFocused}
	fieldsPanel := theme.Panel{Title: "Schema", Width: s.fieldsW, Height: s.paneH, Focused: fieldsFocused}
	detailPanel := theme.Panel{Title: "Field", Width: s.detailW, Height: s.paneH}
	body := lipgloss.JoinHorizontal(lipgloss.Top,
		listPanel.Render(s.list.View()), " ",
		fieldsPanel.Render(s.fields.View()), " ",
		detailPanel.Render(s.detail.View()),
	)
	return body + "\n" + footer
}
```

- [ ] **Step 6: Build, test, visual check**

Run: `go build ./... && go test ./...`
Expected: clean, all pass.

Visual check:

```bash
cat > /tmp/colcheck.go <<'EOF'
package main

import (
	"fmt"
	"clisense/internal/tui/screens"
	"clisense/internal/tui/api"
)

func main() {
	s := screens.NewCollections(nil, 120, 30)
	s, _ = s.Update(api.SuccessMsg{Tag: "collections", Status: 200, Body: []byte(`["products","users"]`)})
	fmt.Println(s.View())
}
EOF
cp /tmp/colcheck.go ./colcheck_main.go && go run ./colcheck_main.go; rm -f ./colcheck_main.go /tmp/colcheck.go
```

Expected: three bordered panels ("Collections" focused, "Schema", "Field"), colored footer, fits 120 cols.

- [ ] **Step 7: Commit**

```bash
git add internal/tui/screens/collections.go
git commit -m "feat(screens): bordered panels and theme styling for Collections"
```

### Task 5.2: Panel-wrap the document search modal

**Files:**
- Modify: `internal/tui/screens/collection_search.go`

- [ ] **Step 1: Add theme import and panel dimension fields**

In `internal/tui/screens/collection_search.go`, add `"clisense/internal/tui/theme"` to the imports. Add fields to `collectionSearchModal` (after `width, height int`):

```go
	leftW, rightW, resultsH, detailH int
```

- [ ] **Step 2: Size inner views and store outer dims in the constructor**

In `newCollectionSearchModal`, replace:

```go
	results := list.New(nil, list.NewDefaultDelegate(), leftWidth, resultsHeight)
	results.Title = "Results"
	results.SetShowStatusBar(false)
	results.SetShowHelp(false)
	results.SetFilteringEnabled(false)

	detail := components.NewJSONView(rightWidth, detailHeight)
	detail.SetContent([]byte("Run a search to inspect matching documents."))

	return collectionSearchModal{
		c:          c,
		collection: collection,
		queryBy:    queryBy,
		queryText:  queryText,
		searchable: append([]string(nil), searchable...),
		results:    results,
		detail:     detail,
		width:      w,
		height:     h,
	}
```

with:

```go
	results := list.New(nil, list.NewDefaultDelegate(), max(1, leftWidth-2), max(1, resultsHeight-2))
	results.Title = "Results"
	results.SetShowTitle(false)
	results.SetShowStatusBar(false)
	results.SetShowHelp(false)
	results.SetFilteringEnabled(false)

	detail := components.NewJSONView(max(1, rightWidth-2), max(1, detailHeight-2))
	detail.SetContent([]byte("Run a search to inspect matching documents."))

	return collectionSearchModal{
		c:          c,
		collection: collection,
		queryBy:    queryBy,
		queryText:  queryText,
		searchable: append([]string(nil), searchable...),
		results:    results,
		detail:     detail,
		width:      w,
		height:     h,
		leftW:      leftWidth, rightW: rightWidth, resultsH: resultsHeight, detailH: detailHeight,
	}
```

- [ ] **Step 3: Size inner views and store outer dims in SetSize**

In `internal/tui/screens/collection_search.go`, replace the body of `SetSize`:

```go
func (m *collectionSearchModal) SetSize(w, h int) {
	m.width, m.height = w, h
	leftWidth, rightWidth, resultsHeight, detailHeight := collectionSearchSizes(w, h)
	m.queryBy.Width = max(20, leftWidth-2)
	m.queryText.Width = max(20, leftWidth-2)
	m.results.SetSize(leftWidth, resultsHeight)
	m.detail.SetSize(rightWidth, detailHeight)
}
```

with:

```go
func (m *collectionSearchModal) SetSize(w, h int) {
	m.width, m.height = w, h
	leftWidth, rightWidth, resultsHeight, detailHeight := collectionSearchSizes(w, h)
	m.leftW, m.rightW, m.resultsH, m.detailH = leftWidth, rightWidth, resultsHeight, detailHeight
	m.queryBy.Width = max(20, leftWidth-4)
	m.queryText.Width = max(20, leftWidth-4)
	m.results.SetSize(max(1, leftWidth-2), max(1, resultsHeight-2))
	m.detail.SetSize(max(1, rightWidth-2), max(1, detailHeight-2))
}
```

- [ ] **Step 4: Rewrite View with panels and styled header/footer**

In `internal/tui/screens/collection_search.go`, replace `View()`:

```go
func (m collectionSearchModal) View() string {
	header := fmt.Sprintf("Search collection %q", m.collection)
	if m.status != "" {
		header += " · " + m.status
	}

	queryHint := "Available fields: "
	if len(m.searchable) == 0 {
		queryHint += "none detected from schema; enter query_by manually"
	} else {
		queryHint += truncateCollectionSearchString(strings.Join(m.searchable, ", "), 100)
	}

	leftLines := []string{
		header,
		"",
		"Search fields: " + m.queryBy.View(),
		lipgloss.NewStyle().Faint(true).Render(queryHint),
		"",
		"Search text:   " + m.queryText.View(),
		"",
		m.results.View(),
	}

	left := strings.Join(leftLines, "\n")
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", m.detail.View())
	footer := "Tab switch · Enter search · Ctrl+B/Ctrl+F scroll JSON · Esc close"
	return body + "\n" + footer
}
```

with:

```go
func (m collectionSearchModal) View() string {
	header := theme.HeaderTitle.Render(fmt.Sprintf("Search %q", m.collection))
	if m.status != "" {
		header += theme.StatusBar.Render(" · ") + theme.Status(statusKind(m.status), m.status)
	}

	queryHint := "Available fields: "
	if len(m.searchable) == 0 {
		queryHint += "none detected from schema; enter query_by manually"
	} else {
		queryHint += truncateCollectionSearchString(strings.Join(m.searchable, ", "), 100)
	}

	leftLines := []string{
		header,
		"",
		"Search fields: " + m.queryBy.View(),
		theme.HeaderMeta.Render(queryHint),
		"",
		"Search text:   " + m.queryText.View(),
		"",
		theme.Panel{Title: "Results", Width: m.leftW, Height: m.resultsH, Focused: m.focus == 2}.Render(m.results.View()),
	}

	left := strings.Join(leftLines, "\n")
	detailPanel := theme.Panel{Title: "Document", Width: m.rightW, Height: m.detailH}
	body := lipgloss.JoinHorizontal(lipgloss.Top, left, " ", detailPanel.Render(m.detail.View()))
	footer := theme.KeyHints(
		[2]string{"Tab", "switch"}, [2]string{"Enter", "search"},
		[2]string{"Ctrl+B/Ctrl+F", "scroll"}, [2]string{"Esc", "close"},
	)
	return body + "\n" + footer
}
```

- [ ] **Step 5: Build, test, visual check**

Run: `go build ./... && go test ./...`
Expected: clean, all pass.

Visual check:

```bash
cat > /tmp/scheck.go <<'EOF'
package main

import (
	"fmt"
	"clisense/internal/tui/screens"
)

func main() {
	// Render an empty collections screen's search modal via the public API is
	// internal; instead just confirm the package builds and Collections renders.
	s := screens.NewCollections(nil, 120, 30)
	fmt.Println(len(s.View()) > 0)
}
EOF
cp /tmp/scheck.go ./scheck_main.go && go run ./scheck_main.go; rm -f ./scheck_main.go /tmp/scheck.go
```

Expected: prints `true` (the search modal is exercised live in the running app — open Collections, press `s`).

- [ ] **Step 6: Commit**

```bash
git add internal/tui/screens/collection_search.go
git commit -m "feat(screens): bordered panels and styled chrome for document search"
```

---

# Phase 6 — Wizards, Conversations, and Setup

## File structure (Phase 6)
- Modify: `internal/tui/screens/curations.go` — `FooterHints`, drop footer string-replace, styled prompt headers
- Modify: `internal/tui/screens/synonyms.go` — same
- Modify: `internal/tui/screens/conversations.go` — styled test modal header/footer + panel for result
- Modify: `internal/tui/screens/setup.go` — styled title/footer

### Task 6.1: Curations — FooterHints and styled prompts

**Files:**
- Modify: `internal/tui/screens/curations.go`

- [ ] **Step 1: Add theme import**

In `internal/tui/screens/curations.go`, add `"clisense/internal/tui/theme"` to the imports.

- [ ] **Step 2: Set FooterHints in the inner Resource strategy**

In `NewCurations`, locate the `ResourceStrategy{...}` literal passed to `NewResource` and add this field to it:

```go
		FooterHints: [][2]string{
			{"/", "filter"}, {"n", "wizard new"}, {"N", "raw new"}, {"e", "wizard edit"},
			{"E", "raw edit"}, {"d", "delete"}, {"PgUp/PgDn", "scroll"}, {"r", "refresh"}, {"q", "quit"},
		},
```

- [ ] **Step 3: Replace the fragile footer string-replace and style the prompt headers**

In `internal/tui/screens/curations.go`, replace `View()`:

```go
func (s Curations) View() string {
	switch {
	case s.namePrompt != nil:
		body := "Curation set name:\n\n" + s.namePrompt.View() + "\n\nEnter continue · Esc cancel"
		if s.createMode == curationCreateRaw {
			body = "Curation set name (raw JSON):\n\n" + s.namePrompt.View() + "\n\nEnter continue · Esc cancel"
		}
		if s.promptErr != "" {
			body += "\n\n" + s.promptErr
		}
		return body
	case s.linkPrompt != nil:
		body := "Collections to link this curation set to:\n\n" + s.linkPrompt.View() +
			"\n\nComma-separated names · Enter continue · Esc cancel"
		if s.promptErr != "" {
			body += "\n\n" + s.promptErr
		}
		return body
	case s.wizard != nil:
		return s.wizard.View()
	default:
		view := s.inner.View()
		return strings.Replace(view,
			"/ filter · n new · e edit · d delete · PgUp/PgDn scroll JSON · r refresh · q quit",
			"/ filter · n wizard new · N raw new · e wizard edit · E raw edit · d delete · PgUp/PgDn scroll JSON · r refresh · q quit",
			1,
		)
	}
}
```

with:

```go
func (s Curations) View() string {
	switch {
	case s.namePrompt != nil:
		title := "Curation set name"
		if s.createMode == curationCreateRaw {
			title = "Curation set name (raw JSON)"
		}
		body := theme.PanelTitle.Render(title) + "\n\n" + s.namePrompt.View() + "\n\n" +
			theme.KeyHints([2]string{"Enter", "continue"}, [2]string{"Esc", "cancel"})
		if s.promptErr != "" {
			body += "\n\n" + theme.StatusErr.Render(s.promptErr)
		}
		return body
	case s.linkPrompt != nil:
		body := theme.PanelTitle.Render("Collections to link this curation set to") + "\n\n" +
			s.linkPrompt.View() + "\n\n" +
			theme.KeyHints([2]string{"Enter", "continue"}, [2]string{"Esc", "cancel"}) +
			theme.StatusBar.Render("  (comma-separated names)")
		if s.promptErr != "" {
			body += "\n\n" + theme.StatusErr.Render(s.promptErr)
		}
		return body
	case s.wizard != nil:
		return s.wizard.View()
	default:
		return s.inner.View()
	}
}
```

- [ ] **Step 4: Remove the now-unused `strings` import if needed**

Run: `go build ./internal/tui/screens/ 2>&1 | head`
If it reports `"strings" imported and not used`, remove the `strings` import line from `curations.go`. If `strings` is still used elsewhere in the file, leave it.

- [ ] **Step 5: Build and test**

Run: `go build ./... && go test ./...`
Expected: clean, all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/screens/curations.go
git commit -m "feat(screens): theme styling and FooterHints for Curations"
```

### Task 6.2: Synonyms — FooterHints and styled prompts

**Files:**
- Modify: `internal/tui/screens/synonyms.go`

- [ ] **Step 1: Add theme import**

In `internal/tui/screens/synonyms.go`, add `"clisense/internal/tui/theme"` to the imports.

- [ ] **Step 2: Set FooterHints in the inner Resource strategy**

In `NewSynonyms`, add to the `ResourceStrategy{...}` literal:

```go
		FooterHints: [][2]string{
			{"/", "filter"}, {"n", "wizard new"}, {"N", "raw new"}, {"e", "wizard edit"},
			{"E", "raw edit"}, {"d", "delete"}, {"PgUp/PgDn", "scroll"}, {"r", "refresh"}, {"q", "quit"},
		},
```

- [ ] **Step 3: Replace the footer string-replace and style prompts**

In `internal/tui/screens/synonyms.go`, replace `View()`:

```go
func (s Synonyms) View() string {
	switch {
	case s.namePrompt != nil:
		body := "Synonym set name:\n\n" + s.namePrompt.View() + "\n\nEnter continue · Esc cancel"
		if s.createMode == synonymCreateRaw {
			body = "Synonym set name (raw JSON):\n\n" + s.namePrompt.View() + "\n\nEnter continue · Esc cancel"
		}
		if s.promptErr != "" {
			body += "\n\n" + s.promptErr
		}
		return body
	case s.linkPrompt != nil:
		body := "Collections to link this synonym set to:\n\n" + s.linkPrompt.View() +
			"\n\nComma-separated names · Enter continue · Esc cancel"
		if s.promptErr != "" {
			body += "\n\n" + s.promptErr
		}
		return body
	case s.wizard != nil:
		return s.wizard.View()
	default:
		view := s.inner.View()
		return strings.Replace(view,
			"/ filter · n new · e edit · d delete · PgUp/PgDn scroll JSON · r refresh · q quit",
			"/ filter · n wizard new · N raw new · e wizard edit · E raw edit · d delete · PgUp/PgDn scroll JSON · r refresh · q quit",
			1,
		)
	}
}
```

with:

```go
func (s Synonyms) View() string {
	switch {
	case s.namePrompt != nil:
		title := "Synonym set name"
		if s.createMode == synonymCreateRaw {
			title = "Synonym set name (raw JSON)"
		}
		body := theme.PanelTitle.Render(title) + "\n\n" + s.namePrompt.View() + "\n\n" +
			theme.KeyHints([2]string{"Enter", "continue"}, [2]string{"Esc", "cancel"})
		if s.promptErr != "" {
			body += "\n\n" + theme.StatusErr.Render(s.promptErr)
		}
		return body
	case s.linkPrompt != nil:
		body := theme.PanelTitle.Render("Collections to link this synonym set to") + "\n\n" +
			s.linkPrompt.View() + "\n\n" +
			theme.KeyHints([2]string{"Enter", "continue"}, [2]string{"Esc", "cancel"}) +
			theme.StatusBar.Render("  (comma-separated names)")
		if s.promptErr != "" {
			body += "\n\n" + theme.StatusErr.Render(s.promptErr)
		}
		return body
	case s.wizard != nil:
		return s.wizard.View()
	default:
		return s.inner.View()
	}
}
```

- [ ] **Step 4: Remove unused `strings` import if needed**

Run: `go build ./internal/tui/screens/ 2>&1 | head`
If it reports `"strings" imported and not used` for `synonyms.go`, remove that import line; otherwise leave it.

- [ ] **Step 5: Build and test**

Run: `go build ./... && go test ./...`
Expected: clean, all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/screens/synonyms.go
git commit -m "feat(screens): theme styling and FooterHints for Synonyms"
```

### Task 6.3: Conversations test modal styling

**Files:**
- Modify: `internal/tui/screens/conversations.go`

- [ ] **Step 1: Add theme import**

In `internal/tui/screens/conversations.go`, add `"clisense/internal/tui/theme"` to the imports.

- [ ] **Step 2: Style the test modal header and footer**

In `internal/tui/screens/conversations.go`, replace the test-modal branch of `View()`:

```go
func (s Conversations) View() string {
	if s.testQuery != nil {
		v := "Test conversation model\n\n"
		v += "Collection: " + s.testCollection.View() + "\n\n"
		v += "Query:\n" + s.testQuery.View() + "\n\n"
		if s.testResult != nil {
			v += "Response:\n" + s.testResult.View() + "\n"
		}
		v += "\nCtrl+S send · Tab switch · Esc close"
		return v
	}
	return s.inner.View()
}
```

with:

```go
func (s Conversations) View() string {
	if s.testQuery != nil {
		v := theme.HeaderTitle.Render("Test conversation model") + "\n\n"
		v += theme.HeaderMeta.Render("Collection: ") + s.testCollection.View() + "\n\n"
		v += theme.HeaderMeta.Render("Query:") + "\n" + s.testQuery.View() + "\n\n"
		if s.testResult != nil {
			v += theme.HeaderMeta.Render("Response:") + "\n" + s.testResult.View() + "\n"
		}
		v += "\n" + theme.KeyHints(
			[2]string{"Ctrl+S", "send"}, [2]string{"Tab", "switch"}, [2]string{"Esc", "close"},
		)
		return v
	}
	return s.inner.View()
}
```

- [ ] **Step 3: Build and test**

Run: `go build ./... && go test ./...`
Expected: clean, all pass.

- [ ] **Step 4: Commit**

```bash
git add internal/tui/screens/conversations.go
git commit -m "feat(screens): theme styling for Conversations test modal"
```

### Task 6.4: Setup/Settings form styling

**Files:**
- Modify: `internal/tui/screens/setup.go`

- [ ] **Step 1: Add theme import**

In `internal/tui/screens/setup.go`, add `"clisense/internal/tui/theme"` to the imports.

- [ ] **Step 2: Style the form title, error, and footer**

In `internal/tui/screens/setup.go`, replace `View()`:

```go
func (s Setup) View() string {
	v := "clisense — Typesense connection\n\n"
	v += "URL:     " + s.url.View() + "\n"
	v += "API key: " + s.apiKey.View() + "\n\n"
	if s.err != "" {
		v += s.err + "\n"
	}
	v += "Tab/↑↓ switch · Enter save · Ctrl+C quit"
	return v
}
```

with:

```go
func (s Setup) View() string {
	v := theme.HeaderTitle.Render("clisense") + theme.HeaderMeta.Render(" — Typesense connection") + "\n\n"
	v += theme.HeaderMeta.Render("URL:     ") + s.url.View() + "\n"
	v += theme.HeaderMeta.Render("API key: ") + s.apiKey.View() + "\n\n"
	if s.err != "" {
		v += theme.StatusErr.Render(s.err) + "\n"
	}
	v += theme.KeyHints(
		[2]string{"Tab/↑↓", "switch"}, [2]string{"Enter", "save"}, [2]string{"Ctrl+C", "quit"},
	)
	return v
}
```

- [ ] **Step 3: Build and test**

Run: `go build ./... && go test ./...`
Expected: clean, all pass.

- [ ] **Step 4: Commit**

```bash
git add internal/tui/screens/setup.go
git commit -m "feat(screens): theme styling for setup/settings form"
```

---

## Final verification (all phases)

- [ ] `go test ./...` — all green.
- [ ] `go vet ./...` — no findings.
- [ ] `gofmt -l internal/ main.go` — empty (run `gofmt -w` on any listed file, then re-check).
- [ ] `go build -o clisense .` — binary builds.
- [ ] Manual: launch `./clisense` against a running Typesense and tab through every screen. Confirm: header shows `clisense — <url>`; each pane is bordered with a title; the focused pane has an accent border; footers show colored key hints; OK/error statuses are green/red. Verify nothing is clipped at 80 and 120 columns.
- [ ] Update `README.md` feature list to mention the bordered panel UI (one line under Features).
- [ ] Commit the README update: `git commit -am "docs: note the bordered-panel TUI styling"`.

---

## Self-review notes

- **Spec coverage:** theme package (palette/styles/Panel/KeyHints/Status) → Tasks 1.1–1.3 ✓; app chrome (header, theme tabs, dead-styles removal, height offset) → Task 2.1 ✓; Resource panels + focus border + footer/status → Task 3.2 ✓; Cluster → 4.1 ✓; Collections + collection_search → 5.1/5.2 ✓; Curations/Synonyms/Conversations/Setup → 6.1–6.4 ✓. Focused-pane accent border realized via `Panel.Focused` (Resource list, Cluster list, Collections follows `s.focus`, search results when `focus==2`). Adaptive palette via `lipgloss.AdaptiveColor` ✓.
- **Cross-task consistency:** `theme.Panel`, `theme.KeyHints`, `theme.Status` signatures are identical across every screen task. `statusKind` (Task 3.1) is used by Resource, Cluster, Collections, and search. The `FooterHints [][2]string` field (Task 3.2) is consumed by Curations (6.1) and Synonyms (6.2), which is why those tasks drop the fragile `strings.Replace`. The `-3`→`-4` offset (Task 2.1) is the only body-height change; per-screen border insets are absorbed by sizing inner views to `outer-2`.
- **Placeholder scan:** every code step shows complete before/after code; every run step has an exact command and expected result. No TBD/TODO.
- **Risk note:** exact vertical fit depends on terminal height; sizing is conservative (panels render at `splitPaneSizes` height, which already reserves margin), but the final manual check at 80/120 columns is the gate for any off-by-one row adjustment.
```
