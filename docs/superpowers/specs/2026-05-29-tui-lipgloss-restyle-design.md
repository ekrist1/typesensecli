# TUI lipgloss Restyle — Design

**Date:** 2026-05-29
**Status:** Approved (pending spec review)

## Goal

Make the clisense TUI more visually appealing using lipgloss: wrap content panes in titled rounded-border panels, add an app header and styled status bars, and unify styling behind a single adaptive color theme. Apply consistently across **all** screens.

## Decisions (locked during brainstorming)

- **Direction:** Bordered panels + styled status bar (not flat polish, not heavy filled-bar redesign).
- **Palette:** Refined adaptive theme — small named palette using `lipgloss.AdaptiveColor` so it renders well on light and dark terminal backgrounds.
- **Scope:** Everything — all screens restyled for full consistency.
- **Focus cue:** The pane with keyboard focus gets an accent-colored border; other panes use a subtle grey border.

## Problem with the current code

- `internal/tui/styles.go` (package `tui`) defines `Border`, `Footer`, `ErrorLine`, `OKLine` — but they are **unused dead code**. The live UI is flat: panes joined with plain `"  "` spacers, uncolored footers, uncolored status text.
- Screens (`internal/tui/screens`) cannot reuse package `tui`'s styles without an import cycle (`tui` imports `screens`). So `collections.go` reinvents colors inline (`lipgloss.NewStyle().Foreground(lipgloss.Color("39"))`), and other screens have no shared palette.
- Colors are raw 256-color codes (`"39"`, `"244"`) duplicated in several places.

## Architecture

### New package: `internal/tui/theme`

A dependency-free styling package (imports only `lipgloss`) that **both** `tui` and `screens` can import without a cycle. It owns the palette, shared styles, and pure render helpers.

```
internal/tui/theme/
  theme.go    # palette + shared lipgloss styles
  panel.go    # Panel render helper
  status.go   # KeyHints + status formatting helpers
```

`components` stays theme-agnostic: it keeps receiving injected styles (as `TabBar` does today). The theme is consumed by `screens` and `tui`, not by `components`.

#### Palette (`theme.go`)

Adaptive colors (Light/Dark pairs):

| Name | Dark | Light | Use |
| --- | --- | --- | --- |
| `Accent` | `39` | `32` | titles, active tab, focused border, key hints |
| `Muted` | `244` | `245` | secondary text, inactive tabs, status bar |
| `Subtle` | `240` | `250` | inactive panel borders |
| `Success` | `42` | `28` | OK status |
| `Error` | `203` | `160` | error status |
| `Warning` | `214` | `166` | warnings (e.g. running an op) |

Exported styles built from the palette, e.g.: `HeaderTitle` (bold accent), `HeaderMeta` (muted), `TabActive` (bold accent), `TabInactive` (muted), `PanelTitle` (bold accent), `StatusBar` (muted), `StatusOK` (success), `StatusErr` (error).

#### `Panel` helper (`panel.go`)

```go
type Panel struct {
    Title         string
    Width, Height int  // OUTER dimensions, including the 1-cell border on each side
    Focused       bool // accent border when true, subtle border otherwise
}

func (p Panel) Render(content string) string // rounded box, title embedded in top border
func (p Panel) InnerWidth() int  // Width - 2
func (p Panel) InnerHeight() int // Height - 2
```

Rendered by building the box explicitly (top border with embedded title `╭ Title ───╮`, bordered content lines, bottom `╰───╯`) so the output width is exactly `Width` and height exactly `Height`. Content longer/taller than the inner box is truncated; shorter is padded — callers size their inner views to `InnerWidth()/InnerHeight()` so this is rare.

#### Status helpers (`status.go`)

- `KeyHints(pairs ...[2]string) string` → `[s] search · [d] delete` with the key glyphs in `Accent` and labels in `Muted`.
- `Status(state, msg string) string` → colors a leading status segment green (`ok`) or red (`err`), used as the left part of a footer.

### App chrome (`internal/tui/app.go`, package `tui`)

`View()` gains a header line above the tab bar:

```
 clisense — http://localhost:8108
 <scrolling tab bar>

 <screen body>
```

- App name in `theme.HeaderTitle`, URL in `theme.HeaderMeta` (from `a.cfg.URL`).
- Tab bar styles move from `styles.go` to `theme` (`TabActive`/`TabInactive`); `app.go` injects them into `components.TabBar` as it does now.
- `styles.go` is reduced to (or removed in favor of) `theme`; no dead styles remain.
- The header consumes **one additional row**, so the body-height offset increases from `-3` to `-4` in `buildTabs`, the `WindowSizeMsg` handler, and screen constructors.

### Per-screen application

Each pane that is currently a raw `list.View()` / `JSONView.View()` joined with spacers becomes a `theme.Panel(...).Render(view)`. Inner views are sized to the panel's `InnerWidth()/InnerHeight()`; panels are joined horizontally with a single space. Each screen marks `Focused: true` on the panel that currently has keyboard focus.

- **`Resource`** (Aliases, NL Models, Stopwords, Presets, Analytics, API Keys): list panel (titled with `TabName`, focused) + detail panel (titled "Detail", subtle). Footer via `KeyHints`; status via `Status`. Editor/confirm/name-prompt modals keep rendering full-screen as today (no panel wrap).
- **`Cluster`**: panels list (titled "Cluster", focused) + JSON detail panel (titled by selected panel name). Footer via `KeyHints`; op status via `Status`/`Warning`.
- **`Collections`**: its three panes (collections list / schema fields / detail) wrapped in panels; the focused panel follows the existing Left/Right pane switching. Drops inline color literals for `theme` (table header/selected styles).
- **`collection_search`**: query/results panes wrapped in panels; footer styled.
- **`Curations`, `Synonyms`** (wizard screens): primary list/detail panes wrapped where they use the split layout; wizard step headers use `PanelTitle`/`HeaderMeta`; footers via `KeyHints`.
- **`Conversations`**: inner `Resource` inherits the panel treatment; the test modal gets a titled panel header and styled footer.
- **`Setup` / `Settings`**: form gets a titled panel container and styled help/footer text for consistency.

## Sizing details

- Panel borders consume 2 columns and 2 rows per pane. Each screen's size calculation (`splitPaneSizes` and per-screen `SetSize`) subtracts the border so inner views fit; the panel is rendered at the outer pane dimensions. Net horizontal footprint is unchanged (borders replace the old `"  "` spacers’ visual role).
- The app header subtracts one additional row from the body height (offset `-3` → `-4`), applied centrally in `app.go`.

## Components and boundaries

- `theme` — what: palette + shared styles + pure render helpers. Depends on: lipgloss only. Used by: `tui`, `screens`.
- `components` — unchanged contract: dumb widgets receiving injected styles.
- `screens` — consume `theme` for panels, footers, status, titles.
- `tui` (app.go) — composes header + tab bar + active screen body; owns chrome height math.

## Error handling

No new failure modes — this is presentational. Status/error text that screens already produce is now routed through `theme.Status` for coloring. API errors continue to surface as today (`formatAPIError`), just colored.

## Testing

- Unit tests for `theme.Panel.Render`: output `lipgloss.Width == Width`, line count `== Height`, title appears in the first line, focused vs subtle border differ.
- Unit tests for `theme.KeyHints` / `theme.Status`: output contains the expected key glyphs/labels and the correct state coloring is applied (assert via `lipgloss.Width` / substring, not raw ANSI).
- Everything else is visual: verified by rendering representative screens at a couple of widths (the approach used to validate the scrolling tab bar), plus the existing screen logic tests must still pass.

## Out of scope / follow-ups

- Configurable/selectable themes (only one adaptive theme ships).
- Mouse support, animations, or spinners.
- Reflowing layouts for very narrow terminals beyond what panels already handle (panels truncate content gracefully).
