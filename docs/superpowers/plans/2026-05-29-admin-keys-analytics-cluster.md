# Admin Features: API Keys, Analytics Rules & Cluster Operations — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add three new admin tabs to the clisense TUI — **API Keys**, **Analytics Rules**, and **Cluster Operations** — covering the corresponding Typesense v30.2 API surface.

**Architecture:** Two of the three features (Analytics Rules, API Keys) reuse the existing generic `screens.Resource` CRUD engine via a `ResourceStrategy`, exactly like `NL Models`, `Stopwords`, and `Conversations` do today. API Keys adds a thin composition wrapper (like `Conversations`) to handle two API-Keys-specific quirks: keys are immutable (no edit) and the full key value is only returned once, on creation. Cluster Operations is a bespoke read-mostly dashboard screen (list of panels + JSON detail pane) plus confirm-gated POST operations, because it doesn't map onto CRUD. Each phase ends with the new tab wired into `app.go` and is independently shippable.

**Tech Stack:** Go 1.25, Bubble Tea / Bubbles / Lipgloss. HTTP via the existing thin `client.Client` + `(method, path)` endpoint helpers. Async calls via `api.DoRequest` returning tagged `SuccessMsg`/`ErrorMsg`.

---

## Background: conventions this plan follows

These are established in the existing codebase — match them exactly:

- **Endpoint helpers** live in `internal/client/endpoints.go` and return `(method, path string)`. They are pure (no I/O) and unit-tested in `endpoints_test.go` by asserting the returned method/path.
- **JSON templates** are embedded files under `internal/templates/`, registered in `templates.go` via `//go:embed` and added to the `All()` map. `templates_test.go` asserts every entry in `All()` is valid JSON.
- **Generic CRUD screens** are built with `screens.NewResource(client, ResourceStrategy{...}, w, h)`. The strategy supplies `List/Create/Update/Delete` endpoint funcs plus `ExtractItems`/`ExtractDetail` parsers. Tags are `"<TabName>:list"`, `"<TabName>:save"`, `"<TabName>:delete"`.
  - `Create` is the POST endpoint used for new items when `CreateNamePrompt == false`.
  - `Update(id)` is used both for "edit existing" (`e` key) and for upsert-by-name (`CreateNamePrompt == true`, `n` key).
  - Set `Create: nil` + `CreateNamePrompt: true` for PUT-by-name resources (e.g. Stopwords); set `Create: <POST>` + `CreateNamePrompt: false` for POST resources (e.g. NL Models).
- **Composition wrappers** (see `screens/conversations.go`) embed a `Resource` as `inner` and intercept specific keys / tagged messages *before* delegating `msg` to `s.inner.Update(msg)`.
- **App wiring** in `internal/tui/app.go`: every screen is a struct field on `App`, constructed in `buildTabs()`, resized in the `tea.WindowSizeMsg` case and (for `Resource`-based screens) registered in `activateTab`, `Update`'s `switch a.active`, `View`'s `switch a.active`, `routeTaggedAPIMessage`, and `activeHasModal`.

### Tab ordering decision

New tabs are inserted **before** `Settings` (Settings must stay last because `settingsTabIndex()` returns `len(tabs)-1` and the Esc handling depends on it). Final order:

```
0 Collections  1 Aliases  2 NL Models  3 Curations  4 Synonyms
5 Stopwords    6 Presets  7 Conversations  8 API Keys  9 Analytics
10 Cluster     11 Settings
```

Digit shortcuts `1`–`9` map to indices `0`–`8` (so `9` → API Keys). Analytics (9), Cluster (10) and Settings (11) are reachable via `Tab`/`Shift+Tab` cycling. This is an accepted limitation; a follow-up could add `0` → Settings. **Do not** change the existing digit handlers in this plan.

> **API verification note:** Endpoint paths below were taken from the Typesense v30.2 docs (API Keys, Cluster Operations, Analytics). Two items could not be 100% confirmed from the docs and are flagged inline: (a) whether a single-rule `GET /analytics/rules/:name` exists, and (b) the exact JSON envelope key for the rules list (`rules` vs bare array). Both are handled defensively by parsing array-or-wrapped-object, mirroring `decodeStopwords`. If a live server is available, confirm with `curl` before implementing the extractor.

---

# Phase 1 — API Keys

**Outcome:** An "API Keys" tab listing existing keys (id, description, prefix, actions, collections), creating new keys via the JSON editor, showing the generated secret **once** in a dismissible modal, and deleting keys. Editing is intentionally disabled (keys are immutable in Typesense).

**Typesense endpoints (v30.2):**
| Method | Path | Use |
| --- | --- | --- |
| GET | `/keys` | list (returns `{"keys":[...]}`) |
| GET | `/keys/:id` | get one |
| POST | `/keys` | create (response includes full `value`, returned only here) |
| DELETE | `/keys/:id` | delete |

Key objects use a **numeric** `id`. List/get responses expose only `value_prefix`, not the secret.

## File structure (Phase 1)

- Modify: `internal/client/endpoints.go` — add 4 key endpoint helpers
- Modify: `internal/client/endpoints_test.go` — add table cases
- Create: `internal/templates/apikey.json` — create-key skeleton
- Modify: `internal/templates/templates.go` — embed + register
- Create: `internal/tui/screens/keys.go` — `Keys` wrapper screen
- Create: `internal/tui/screens/keys_test.go` — extractor + generated-key capture tests
- Modify: `internal/tui/app.go` — wire the tab

### Task 1.1: Add API Keys endpoint helpers

**Files:**
- Modify: `internal/client/endpoints.go` (append after the Presets helpers, before the Conversation helpers)
- Test: `internal/client/endpoints_test.go`

- [ ] **Step 1: Write the failing test**

Add a new test function to `internal/client/endpoints_test.go`:

```go
func TestAPIKeyEndpoints(t *testing.T) {
	tests := []struct {
		name  string
		wantM string
		wantP string
		call  func() (string, string)
	}{
		{name: "list keys", wantM: "GET", wantP: "/keys", call: ListKeys},
		{name: "create key", wantM: "POST", wantP: "/keys", call: CreateKey},
		{name: "get key", wantM: "GET", wantP: "/keys/7", call: func() (string, string) { return GetKey("7") }},
		{name: "delete key", wantM: "DELETE", wantP: "/keys/7", call: func() (string, string) { return DeleteKey("7") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotM, gotP := tt.call()
			if gotM != tt.wantM || gotP != tt.wantP {
				t.Fatalf("got %s %s, want %s %s", gotM, gotP, tt.wantM, tt.wantP)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/client/ -run TestAPIKeyEndpoints -v`
Expected: FAIL — compile error, `undefined: ListKeys` (and the others).

- [ ] **Step 3: Add the endpoint helpers**

In `internal/client/endpoints.go`, add:

```go
func ListKeys() (string, string)   { return "GET", "/keys" }
func CreateKey() (string, string)  { return "POST", "/keys" }
func GetKey(id string) (string, string) {
	return "GET", fmt.Sprintf("/keys/%s", id)
}
func DeleteKey(id string) (string, string) {
	return "DELETE", fmt.Sprintf("/keys/%s", id)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/client/ -run TestAPIKeyEndpoints -v`
Expected: PASS (4 subtests).

- [ ] **Step 5: Commit**

```bash
git add internal/client/endpoints.go internal/client/endpoints_test.go
git commit -m "feat(client): add API Keys endpoint helpers"
```

### Task 1.2: Add the create-key JSON template

**Files:**
- Create: `internal/templates/apikey.json`
- Modify: `internal/templates/templates.go`
- Test: `internal/templates/templates_test.go` (existing `TestAllTemplatesValidJSON` covers it once registered)

- [ ] **Step 1: Create the template file**

Create `internal/templates/apikey.json`:

```json
{
  "description": "Search-only key",
  "actions": ["documents:search"],
  "collections": ["*"]
}
```

- [ ] **Step 2: Register the embed**

In `internal/templates/templates.go`, add the embed directive alongside the others:

```go
//go:embed apikey.json
var APIKey []byte
```

And add the entry to the map returned by `All()`:

```go
		"apikey":       APIKey,
```

- [ ] **Step 3: Run the templates test**

Run: `go test ./internal/templates/ -run TestAllTemplatesValidJSON -v`
Expected: PASS (the new `apikey` template parses as valid JSON).

- [ ] **Step 4: Commit**

```bash
git add internal/templates/apikey.json internal/templates/templates.go
git commit -m "feat(templates): add API key creation template"
```

### Task 1.3: Build the API Keys screen

The `Keys` screen embeds a `Resource` (POST-create, delete, no edit) and adds: (1) swallowing the `e` key so keys can't be edited, and (2) capturing the create response so the generated secret is shown once in a modal. It mirrors the `Conversations` composition pattern.

**Files:**
- Create: `internal/tui/screens/keys.go`
- Test: `internal/tui/screens/keys_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/tui/screens/keys_test.go`:

```go
package screens

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/tui/api"
)

func TestKeysExtractItemsWrappedAndNumericID(t *testing.T) {
	body := []byte(`{"keys":[{"id":0,"description":"admin","value_prefix":"abc"},{"id":7,"description":"search","value_prefix":"xyz"}]}`)
	ids, err := keysExtractItems(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 || ids[0] != "0" || ids[1] != "7" {
		t.Fatalf("got %v, want [0 7]", ids)
	}
}

func TestKeysCapturesGeneratedSecretOnCreate(t *testing.T) {
	k := NewKeys(nil, 80, 24)
	// Simulate a successful create response carrying the full secret.
	saveTag := "API Keys:save"
	k, _ = k.Update(api.SuccessMsg{Tag: saveTag, Status: 201, Body: []byte(`{"id":9,"value":"SECRETVALUE","description":"x"}`)})
	if !k.HasModal() {
		t.Fatal("expected generated-key modal to be open after create")
	}
	if k.generated == nil {
		t.Fatal("generated key payload not captured")
	}
	// Dismissing with esc closes the modal.
	k, _ = k.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if k.generated != nil {
		t.Fatal("generated-key modal still open after esc")
	}
}

func TestKeysSwallowsEditKey(t *testing.T) {
	k := NewKeys(nil, 80, 24)
	// 'e' must not open an editor on the inner resource (keys are immutable).
	k, _ = k.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
	if k.inner.editor != nil {
		t.Fatal("edit must be disabled for API keys")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/screens/ -run TestKeys -v`
Expected: FAIL — `undefined: NewKeys`, `keysExtractItems`.

- [ ] **Step 3: Implement the screen**

Create `internal/tui/screens/keys.go`:

```go
package screens

import (
	"encoding/json"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/client"
	"clisense/internal/templates"
	"clisense/internal/tui/api"
	"clisense/internal/tui/components"
)

// Keys is the API Keys screen. It wraps the generic Resource CRUD engine but
// (1) disables editing — Typesense API keys are immutable — and (2) captures the
// create response so the generated secret (returned only once) can be shown in a
// dismissible modal.
type Keys struct {
	inner     Resource
	generated *components.JSONView // non-nil while the generated-secret modal is shown
}

const keysSaveTag = "API Keys:save"

func NewKeys(c *client.Client, w, h int) Keys {
	inner := NewResource(c, ResourceStrategy{
		TabName:      "API Keys",
		Template:     templates.APIKey,
		List:         client.ListKeys,
		Create:       client.CreateKey,
		Update:       nil, // editing disabled; the wrapper swallows the "e" key
		Delete:       client.DeleteKey,
		ExtractItems: keysExtractItems,
		ExtractDetail: keysExtractDetail,
	}, w, h)
	return Keys{inner: inner}
}

func (s *Keys) SetSize(w, h int) {
	s.inner.SetSize(w, h)
	if s.generated != nil {
		s.generated.SetSize(w-4, h-6)
	}
}

func (s Keys) Init() tea.Cmd { return s.inner.Init() }

// HasModal reports the generated-secret modal or any inner Resource modal.
func (s Keys) HasModal() bool { return s.generated != nil || s.inner.HasModal() }

func (s Keys) Update(msg tea.Msg) (Keys, tea.Cmd) {
	// Generated-secret modal open — swallow keys until dismissed.
	if s.generated != nil {
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "esc", "enter", "q":
				s.generated = nil
				return s, nil
			case "pgup":
				s.generated.PageUp()
				return s, nil
			case "pgdown":
				s.generated.PageDown()
				return s, nil
			}
		}
		return s, nil
	}

	// Capture the secret from a successful create before the inner resource
	// consumes the message (it would otherwise just refresh the list).
	if m, ok := msg.(api.SuccessMsg); ok && m.Tag == keysSaveTag {
		view := components.NewJSONView(s.inner.width-4, s.inner.height-6)
		view.SetContent(m.Body)
		s.generated = &view
		// Still let inner refresh its list.
		var cmd tea.Cmd
		s.inner, cmd = s.inner.Update(msg)
		return s, cmd
	}

	// Disable edit: swallow "e" unless an inner modal owns the key.
	if km, ok := msg.(tea.KeyMsg); ok && km.String() == "e" && !s.inner.HasModal() {
		return s, nil
	}

	var cmd tea.Cmd
	s.inner, cmd = s.inner.Update(msg)
	return s, cmd
}

func (s Keys) View() string {
	if s.generated != nil {
		return "Generated API key (copy it now — it will not be shown again)\n\n" +
			s.generated.View() +
			"\n\nEsc/Enter dismiss · PgUp/PgDn scroll"
	}
	return s.inner.View()
}

// keysExtractItems parses the wrapped {"keys":[...]} list (also tolerates a bare
// array) and returns string ids derived from the numeric "id" field.
func keysExtractItems(body []byte) ([]string, error) {
	keys, err := decodeKeys(body)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(keys))
	for _, k := range keys {
		ids = append(ids, keyID(k))
	}
	return ids, nil
}

func keysExtractDetail(body []byte, id string) []byte {
	keys, err := decodeKeys(body)
	if err != nil {
		return nil
	}
	for _, k := range keys {
		if keyID(k) == id {
			b, _ := json.MarshalIndent(k, "", "  ")
			return b
		}
	}
	return nil
}

func decodeKeys(body []byte) ([]map[string]any, error) {
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err == nil {
		return arr, nil
	}
	var obj struct {
		Keys []map[string]any `json:"keys"`
	}
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil, err
	}
	return obj.Keys, nil
}

// keyID renders the numeric "id" (decoded as float64 by encoding/json) as a
// string so it can be used as a list id and in the /keys/:id path.
func keyID(k map[string]any) string {
	switch v := k["id"].(type) {
	case float64:
		return fmt.Sprintf("%d", int64(v))
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/screens/ -run TestKeys -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/tui/screens/keys.go internal/tui/screens/keys_test.go
git commit -m "feat(tui): API Keys screen with one-time secret display"
```

### Task 1.4: Wire the API Keys tab into the app

**Files:**
- Modify: `internal/tui/app.go`

- [ ] **Step 1: Add the struct field**

In the `App` struct (after `conversations screens.Conversations`), add:

```go
	keys          screens.Keys
```

- [ ] **Step 2: Add the tab title**

In `NewApp`, change the `tabs` slice to insert `"API Keys"` before `"Settings"`:

```go
		tabs: []string{"Collections", "Aliases", "NL Models", "Curations", "Synonyms", "Stopwords", "Presets", "Conversations", "API Keys", "Settings"},
```

- [ ] **Step 3: Construct it in buildTabs**

In `buildTabs()`, after the `conversations` line:

```go
	a.keys = screens.NewKeys(a.c, a.width, a.height-3)
```

- [ ] **Step 4: Resize, activate, update, view, route, modal-gate**

In `Update`'s `tea.WindowSizeMsg` case, add (with the other `SetSize` calls):

```go
		a.keys.SetSize(a.width, a.height-3)
```

API Keys is now index 8 and Settings is index 9. Update `activateTab`'s switch so case `8` returns `a.keys.Init()` and add case `9` returning `a.settings.Init()`:

```go
	case 8:
		return a.keys.Init()
	case 9:
		return a.settings.Init()
```

In `Update`'s `switch a.active`, add case `8` for keys and move settings to `9`:

```go
	case 8:
		a.keys, cmd = a.keys.Update(msg)
	case 9:
		a.settings, cmd = a.settings.Update(msg)
```

In `View`'s `switch a.active`, mirror it:

```go
	case 8:
		body = a.keys.View()
	case 9:
		body = a.settings.View()
```

In `routeTaggedAPIMessage`, add a case (note the generated-secret save tag is `"API Keys:save"`, which already matches the `HasPrefix(tag, "API Keys:")` form):

```go
	case strings.HasPrefix(tag, "API Keys:"):
		a.keys, cmd = a.keys.Update(msg)
```

In `activeHasModal`, add case `8`:

```go
	case 8:
		return a.keys.HasModal()
```

> Note: `activeHasModal`'s `case 8` previously returned the Settings form `true`. Because Settings is now index 9 and `settingsTabIndex()` is computed dynamically, the existing `if a.active == a.settingsTabIndex()` Esc handling in the `tea.KeyMsg` block keeps working unchanged. The hard-coded `case 8: return true` for settings inside `activeHasModal` must be **replaced** by the keys case above, and a `case 9: return true` added for settings:

```go
	case 9:
		// Settings is a form — always treat as modal.
		return true
```

- [ ] **Step 5: Build, test, and manually verify**

Run: `go build ./... && go test ./...`
Expected: builds clean, all tests pass.

Manual check (requires a running Typesense): launch `./clisense`, press `9` to reach API Keys, confirm the list loads, press `n` to create a search-only key, submit, and confirm the generated-secret modal appears with a `value` field. Press `d` to delete a key.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat(tui): wire API Keys tab into app"
```

---

# Phase 2 — Analytics Rules

**Outcome:** An "Analytics" tab listing analytics rules, creating/editing them via the JSON editor (POST upserts by `name`), and deleting them by name.

**Typesense endpoints (v30.2):**
| Method | Path | Use |
| --- | --- | --- |
| GET | `/analytics/rules` | list |
| POST | `/analytics/rules` | create/upsert (single object or array; `name` is in the body) |
| DELETE | `/analytics/rules/:name` | delete |

A rule body looks like: `{"name","type" (popular_queries|nohits_queries|counter|log),"collection","event_type","params":{...}}`. Because POST upserts by `name`, both "new" and "edit" flows POST to `/analytics/rules`, so this screen reuses `Resource` directly (no wrapper needed) by pointing both `Create` and `Update` at the POST endpoint.

> **Verification note:** The rules list envelope (`{"rules":[...]}` vs a bare array) and the existence of `GET /analytics/rules/:name` were not fully confirmed from docs. The extractor below parses array-or-`{"rules":[...]}` defensively, mirroring `decodeStopwords`. No single-rule GET is needed because the list response embeds full rule objects (same assumption the other screens make).

## File structure (Phase 2)

- Modify: `internal/client/endpoints.go` — analytics endpoint helpers
- Modify: `internal/client/endpoints_test.go` — table cases
- Create: `internal/templates/analytics_rule.json` — rule skeleton
- Modify: `internal/templates/templates.go` — embed + register
- Create: `internal/tui/screens/analytics.go` — `NewAnalytics` (returns `Resource`)
- Create: `internal/tui/screens/analytics_test.go` — extractor tests
- Modify: `internal/tui/app.go` — wire the tab

### Task 2.1: Add Analytics endpoint helpers

**Files:**
- Modify: `internal/client/endpoints.go`
- Test: `internal/client/endpoints_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/client/endpoints_test.go`:

```go
func TestAnalyticsEndpoints(t *testing.T) {
	tests := []struct {
		name  string
		wantM string
		wantP string
		call  func() (string, string)
	}{
		{name: "list rules", wantM: "GET", wantP: "/analytics/rules", call: ListAnalyticsRules},
		{name: "create rule", wantM: "POST", wantP: "/analytics/rules", call: UpsertAnalyticsRule},
		{name: "delete rule", wantM: "DELETE", wantP: "/analytics/rules/top_queries", call: func() (string, string) { return DeleteAnalyticsRule("top_queries") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotM, gotP := tt.call()
			if gotM != tt.wantM || gotP != tt.wantP {
				t.Fatalf("got %s %s, want %s %s", gotM, gotP, tt.wantM, tt.wantP)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/client/ -run TestAnalyticsEndpoints -v`
Expected: FAIL — `undefined: ListAnalyticsRules` etc.

- [ ] **Step 3: Add the endpoint helpers**

In `internal/client/endpoints.go`:

```go
func ListAnalyticsRules() (string, string)   { return "GET", "/analytics/rules" }
func UpsertAnalyticsRule() (string, string)  { return "POST", "/analytics/rules" }
func DeleteAnalyticsRule(name string) (string, string) {
	return "DELETE", fmt.Sprintf("/analytics/rules/%s", name)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/client/ -run TestAnalyticsEndpoints -v`
Expected: PASS (3 subtests).

- [ ] **Step 5: Commit**

```bash
git add internal/client/endpoints.go internal/client/endpoints_test.go
git commit -m "feat(client): add Analytics rules endpoint helpers"
```

### Task 2.2: Add the analytics rule template

**Files:**
- Create: `internal/templates/analytics_rule.json`
- Modify: `internal/templates/templates.go`

- [ ] **Step 1: Create the template file**

Create `internal/templates/analytics_rule.json`:

```json
{
  "name": "popular_queries",
  "type": "popular_queries",
  "collection": "products",
  "event_type": "search",
  "params": {
    "destination_collection": "product_queries",
    "limit": 1000,
    "expand_query": false,
    "capture_search_requests": true
  }
}
```

- [ ] **Step 2: Register the embed**

In `internal/templates/templates.go`:

```go
//go:embed analytics_rule.json
var AnalyticsRule []byte
```

Add to `All()`:

```go
		"analytics_rule": AnalyticsRule,
```

- [ ] **Step 3: Run the templates test**

Run: `go test ./internal/templates/ -run TestAllTemplatesValidJSON -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/templates/analytics_rule.json internal/templates/templates.go
git commit -m "feat(templates): add analytics rule template"
```

### Task 2.3: Build the Analytics screen

**Files:**
- Create: `internal/tui/screens/analytics.go`
- Test: `internal/tui/screens/analytics_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/tui/screens/analytics_test.go`:

```go
package screens

import "testing"

func TestAnalyticsExtractItemsWrappedAndBare(t *testing.T) {
	wrapped := []byte(`{"rules":[{"name":"top","type":"popular_queries"},{"name":"misses","type":"nohits_queries"}]}`)
	ids, err := analyticsExtractItems(wrapped)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 || ids[0] != "top" || ids[1] != "misses" {
		t.Fatalf("got %v, want [top misses]", ids)
	}

	bare := []byte(`[{"name":"only","type":"counter"}]`)
	ids, err = analyticsExtractItems(bare)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 1 || ids[0] != "only" {
		t.Fatalf("got %v, want [only]", ids)
	}
}

func TestAnalyticsExtractDetail(t *testing.T) {
	body := []byte(`{"rules":[{"name":"top","type":"popular_queries"}]}`)
	d := analyticsExtractDetail(body, "top")
	if d == nil {
		t.Fatal("expected detail for rule 'top'")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/screens/ -run TestAnalytics -v`
Expected: FAIL — `undefined: analyticsExtractItems`, `analyticsExtractDetail`.

- [ ] **Step 3: Implement the screen**

Create `internal/tui/screens/analytics.go`:

```go
package screens

import (
	"encoding/json"

	"clisense/internal/client"
	"clisense/internal/templates"
)

// NewAnalytics returns a Resource screen for analytics rules. Both create and
// edit POST to /analytics/rules (POST upserts by the rule's "name" field), so
// the generic engine handles the full lifecycle without a wrapper.
func NewAnalytics(c *client.Client, w, h int) Resource {
	return NewResource(c, ResourceStrategy{
		TabName:       "Analytics",
		Template:      templates.AnalyticsRule,
		List:          client.ListAnalyticsRules,
		Create:        client.UpsertAnalyticsRule,
		Update:        func(string) (string, string) { return client.UpsertAnalyticsRule() },
		Delete:        client.DeleteAnalyticsRule,
		ExtractItems:  analyticsExtractItems,
		ExtractDetail: analyticsExtractDetail,
	}, w, h)
}

func analyticsExtractItems(body []byte) ([]string, error) {
	rules, err := decodeAnalyticsRules(body)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(rules))
	for _, r := range rules {
		if name, ok := r["name"].(string); ok {
			ids = append(ids, name)
		}
	}
	return ids, nil
}

func analyticsExtractDetail(body []byte, id string) []byte {
	rules, err := decodeAnalyticsRules(body)
	if err != nil {
		return nil
	}
	for _, r := range rules {
		if r["name"] == id {
			b, _ := json.MarshalIndent(r, "", "  ")
			return b
		}
	}
	return nil
}

func decodeAnalyticsRules(body []byte) ([]map[string]any, error) {
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err == nil {
		return arr, nil
	}
	var obj struct {
		Rules []map[string]any `json:"rules"`
	}
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil, err
	}
	return obj.Rules, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/screens/ -run TestAnalytics -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/tui/screens/analytics.go internal/tui/screens/analytics_test.go
git commit -m "feat(tui): Analytics rules screen"
```

### Task 2.4: Wire the Analytics tab into the app

Analytics becomes index 9; Settings moves to index 10.

**Files:**
- Modify: `internal/tui/app.go`

- [ ] **Step 1: Add the struct field**

In the `App` struct, after `keys screens.Keys`:

```go
	analytics     screens.Resource
```

- [ ] **Step 2: Add the tab title**

Insert `"Analytics"` before `"Settings"` in the `tabs` slice:

```go
		tabs: []string{"Collections", "Aliases", "NL Models", "Curations", "Synonyms", "Stopwords", "Presets", "Conversations", "API Keys", "Analytics", "Settings"},
```

- [ ] **Step 3: Construct it in buildTabs**

```go
	a.analytics = screens.NewAnalytics(a.c, a.width, a.height-3)
```

- [ ] **Step 4: Resize / activate / update / view / route / modal-gate**

In the `tea.WindowSizeMsg` case:

```go
		a.analytics.SetSize(a.width, a.height-3)
```

In `activateTab` (Analytics = 9, Settings = 10):

```go
	case 9:
		return a.analytics.Init()
	case 10:
		return a.settings.Init()
```

In `Update`'s `switch a.active`:

```go
	case 9:
		a.analytics, cmd = a.analytics.Update(msg)
	case 10:
		a.settings, cmd = a.settings.Update(msg)
```

In `View`'s `switch a.active`:

```go
	case 9:
		body = a.analytics.View()
	case 10:
		body = a.settings.View()
```

In `routeTaggedAPIMessage`:

```go
	case strings.HasPrefix(tag, "Analytics:"):
		a.analytics, cmd = a.analytics.Update(msg)
```

In `activeHasModal`, add case 9 and move the settings `return true` to case 10:

```go
	case 9:
		return a.analytics.HasModal()
	case 10:
		// Settings is a form — always treat as modal.
		return true
```

- [ ] **Step 5: Build, test, manual verify**

Run: `go build ./... && go test ./...`
Expected: clean build, all tests pass.

Manual (running Typesense): launch, `Tab` to "Analytics", create a `popular_queries` rule via `n`, confirm it appears in the list, edit with `e`, delete with `d`.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat(tui): wire Analytics tab into app"
```

---

# Phase 3 — Cluster Operations

**Outcome:** A "Cluster" tab presenting a left-hand list of read-only panels (Health, Stats, Metrics, Debug) with a JSON detail pane on the right, refreshable with `r`, plus four confirm-gated POST operations (snapshot, compact DB, re-elect leader / vote, clear cache).

**Typesense endpoints (v30.2):**
| Method | Path | Use |
| --- | --- | --- |
| GET | `/health` | node health |
| GET | `/stats.json` | API endpoint stats |
| GET | `/metrics.json` | RAM/CPU/disk/network metrics |
| GET | `/debug` | version + raft state |
| POST | `/operations/snapshot?snapshot_path=…` | point-in-time snapshot |
| POST | `/operations/db/compact` | compact on-disk DB |
| POST | `/operations/vote` | trigger leader re-election |
| POST | `/operations/cache/clear` | clear search cache |

Operations are confirm-gated because three of them have real operational impact. The snapshot path defaults to a constant; making it editable is an explicit non-goal here (YAGNI) — note it as a follow-up.

## File structure (Phase 3)

- Modify: `internal/client/endpoints.go` — cluster endpoint helpers
- Modify: `internal/client/endpoints_test.go` — table cases
- Create: `internal/tui/screens/cluster.go` — bespoke `Cluster` screen
- Create: `internal/tui/screens/cluster_test.go` — panel routing + confirm tests
- Modify: `internal/tui/app.go` — wire the tab

### Task 3.1: Add Cluster endpoint helpers

**Files:**
- Modify: `internal/client/endpoints.go`
- Test: `internal/client/endpoints_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/client/endpoints_test.go`:

```go
func TestClusterEndpoints(t *testing.T) {
	tests := []struct {
		name  string
		wantM string
		wantP string
		call  func() (string, string)
	}{
		{name: "health", wantM: "GET", wantP: "/health", call: Health},
		{name: "stats", wantM: "GET", wantP: "/stats.json", call: Stats},
		{name: "metrics", wantM: "GET", wantP: "/metrics.json", call: Metrics},
		{name: "debug", wantM: "GET", wantP: "/debug", call: Debug},
		{name: "compact", wantM: "POST", wantP: "/operations/db/compact", call: CompactDB},
		{name: "vote", wantM: "POST", wantP: "/operations/vote", call: Vote},
		{name: "clear cache", wantM: "POST", wantP: "/operations/cache/clear", call: ClearCache},
		{name: "snapshot", wantM: "POST", wantP: "/operations/snapshot?snapshot_path=%2Ftmp%2Ftypesense-snapshot", call: func() (string, string) { return Snapshot("/tmp/typesense-snapshot") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotM, gotP := tt.call()
			if gotM != tt.wantM || gotP != tt.wantP {
				t.Fatalf("got %s %s, want %s %s", gotM, gotP, tt.wantM, tt.wantP)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/client/ -run TestClusterEndpoints -v`
Expected: FAIL — `undefined: Health` etc.

- [ ] **Step 3: Add the endpoint helpers**

In `internal/client/endpoints.go` (the `net/url` import is already present):

```go
func Health() (string, string)     { return "GET", "/health" }
func Stats() (string, string)      { return "GET", "/stats.json" }
func Metrics() (string, string)    { return "GET", "/metrics.json" }
func Debug() (string, string)      { return "GET", "/debug" }
func CompactDB() (string, string)  { return "POST", "/operations/db/compact" }
func Vote() (string, string)       { return "POST", "/operations/vote" }
func ClearCache() (string, string) { return "POST", "/operations/cache/clear" }

func Snapshot(path string) (string, string) {
	v := url.Values{}
	v.Set("snapshot_path", path)
	return "POST", fmt.Sprintf("/operations/snapshot?%s", v.Encode())
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/client/ -run TestClusterEndpoints -v`
Expected: PASS (8 subtests).

- [ ] **Step 5: Commit**

```bash
git add internal/client/endpoints.go internal/client/endpoints_test.go
git commit -m "feat(client): add cluster operations endpoint helpers"
```

### Task 3.2: Build the Cluster screen

This is a bespoke screen. Left: a `list.Model` of four panels. Right: a `components.JSONView` showing the selected panel's last-fetched payload. `r` refreshes all four reads. Operation keys (`S`/`C`/`V`/`X`) open a `components.Confirm`; on confirmation it POSTs and surfaces the response into the detail pane. It mirrors the layout helpers and confirm pattern used elsewhere.

**Files:**
- Create: `internal/tui/screens/cluster.go`
- Test: `internal/tui/screens/cluster_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/tui/screens/cluster_test.go`:

```go
package screens

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/tui/api"
	"clisense/internal/tui/components"
)

func TestClusterStoresPanelPayload(t *testing.T) {
	c := NewCluster(nil, 100, 30)
	c, _ = c.Update(api.SuccessMsg{Tag: "Cluster:health", Status: 200, Body: []byte(`{"ok":true}`)})
	if got := string(c.payloads["Cluster:health"]); got != `{"ok":true}` {
		t.Fatalf("payload not stored, got %q", got)
	}
}

func TestClusterOperationOpensConfirm(t *testing.T) {
	c := NewCluster(nil, 100, 30)
	// "C" requests DB compaction and must open a confirm dialog, not fire immediately.
	c, cmd := c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("C")})
	if !c.HasModal() {
		t.Fatal("expected confirm dialog to open for compact operation")
	}
	if cmd != nil {
		t.Fatal("operation must not fire before confirmation")
	}
}

func TestClusterConfirmedOperationFires(t *testing.T) {
	c := NewCluster(nil, 100, 30)
	c, _ = c.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("X")}) // clear cache
	if !c.HasModal() {
		t.Fatal("expected confirm dialog")
	}
	_, cmd := c.Update(components.ConfirmResultMsg{Tag: "Cluster:op:clear-cache", Confirmed: true})
	if cmd == nil {
		t.Fatal("expected the clear-cache request command after confirmation")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/screens/ -run TestCluster -v`
Expected: FAIL — `undefined: NewCluster`.

- [ ] **Step 3: Implement the screen**

Create `internal/tui/screens/cluster.go`:

```go
package screens

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"clisense/internal/client"
	"clisense/internal/tui/api"
	"clisense/internal/tui/components"
)

// panel describes a read-only cluster endpoint shown in the left list.
type panel struct {
	title string
	tag   string
	call  func() (string, string)
}

// op describes a confirm-gated POST operation.
type op struct {
	key     string // trigger key
	tag     string
	label   string // confirm prompt
	call    func() (string, string)
}

type clusterPanelItem struct{ title string }

func (i clusterPanelItem) Title() string       { return i.title }
func (i clusterPanelItem) Description() string { return "" }
func (i clusterPanelItem) FilterValue() string { return i.title }

const snapshotPath = "/tmp/typesense-snapshot"

// Cluster is the cluster-operations dashboard: read-only panels on the left, a
// JSON detail pane on the right, plus confirm-gated POST operations.
type Cluster struct {
	c       *client.Client
	list    list.Model
	detail  components.JSONView
	confirm *components.Confirm

	panels   []panel
	ops      []op
	payloads map[string][]byte // tag -> last response body

	status        string
	width, height int
}

func NewCluster(c *client.Client, w, h int) Cluster {
	panels := []panel{
		{title: "Health", tag: "Cluster:health", call: client.Health},
		{title: "Stats", tag: "Cluster:stats", call: client.Stats},
		{title: "Metrics", tag: "Cluster:metrics", call: client.Metrics},
		{title: "Debug", tag: "Cluster:debug", call: client.Debug},
	}
	ops := []op{
		{key: "S", tag: "Cluster:op:snapshot", label: "Create a snapshot at " + snapshotPath + "?", call: func() (string, string) { return client.Snapshot(snapshotPath) }},
		{key: "C", tag: "Cluster:op:compact", label: "Compact the on-disk database?", call: client.CompactDB},
		{key: "V", tag: "Cluster:op:vote", label: "Trigger leader re-election (vote)?", call: client.Vote},
		{key: "X", tag: "Cluster:op:clear-cache", label: "Clear the search cache?", call: client.ClearCache},
	}

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
}

func (s *Cluster) SetSize(w, h int) {
	s.width, s.height = w, h
	listWidth, detailWidth, paneHeight := splitPaneSizes(w, h)
	s.list.SetSize(listWidth, paneHeight)
	s.detail.SetSize(detailWidth, paneHeight)
}

func (s Cluster) HasModal() bool { return s.confirm != nil }

// Init fetches all read-only panels once on activation.
func (s Cluster) Init() tea.Cmd { return s.refreshAll() }

func (s Cluster) refreshAll() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(s.panels))
	for _, p := range s.panels {
		m, path := p.call()
		cmds = append(cmds, api.DoRequest(s.c, p.tag, m, path, nil))
	}
	return tea.Batch(cmds...)
}

func (s Cluster) selectedPanelTag() string {
	if it, ok := s.list.SelectedItem().(clusterPanelItem); ok {
		for _, p := range s.panels {
			if p.title == it.title {
				return p.tag
			}
		}
	}
	return ""
}

func (s *Cluster) showSelectedPayload() {
	if body, ok := s.payloads[s.selectedPanelTag()]; ok && body != nil {
		s.detail.SetContent(body)
	}
}

func (s Cluster) Update(msg tea.Msg) (Cluster, tea.Cmd) {
	// Confirm modal owns input until resolved.
	if s.confirm != nil {
		cmd := s.confirm.Update(msg)
		if res, ok := msg.(components.ConfirmResultMsg); ok {
			for _, o := range s.ops {
				if res.Tag == o.tag {
					s.confirm = nil
					if res.Confirmed {
						m, path := o.call()
						s.status = "running " + o.label
						return s, api.DoRequest(s.c, o.tag, m, path, nil)
					}
					return s, nil
				}
			}
		}
		return s, cmd
	}

	switch m := msg.(type) {
	case api.SuccessMsg:
		// Panel read result.
		for _, p := range s.panels {
			if m.Tag == p.tag {
				s.payloads[p.tag] = m.Body
				if s.selectedPanelTag() == p.tag {
					s.detail.SetContent(m.Body)
				}
				return s, nil
			}
		}
		// Operation result — show response in the detail pane.
		for _, o := range s.ops {
			if m.Tag == o.tag {
				s.status = "OK: " + o.label
				s.detail.SetContent(m.Body)
				return s, nil
			}
		}
	case api.ErrorMsg:
		s.status = formatAPIError(m)
		return s, nil
	case tea.KeyMsg:
		switch m.String() {
		case "r":
			return s, s.refreshAll()
		case "pgup":
			s.detail.PageUp()
			return s, nil
		case "pgdown":
			s.detail.PageDown()
			return s, nil
		}
		// Operation trigger keys open a confirm dialog.
		for _, o := range s.ops {
			if m.String() == o.key {
				cf := components.NewConfirm(o.label, o.tag)
				s.confirm = &cf
				return s, nil
			}
		}
	}

	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)
	s.showSelectedPayload()
	return s, cmd
}

func (s Cluster) View() string {
	if s.confirm != nil {
		return s.confirm.View()
	}
	footer := "r refresh · S snapshot · C compact · V vote · X clear-cache · PgUp/PgDn scroll · q quit"
	if s.status != "" {
		footer = s.status + " · " + footer
	}
	body := lipgloss.JoinHorizontal(lipgloss.Top, s.list.View(), "  ", s.detail.View())
	return body + "\n" + fmt.Sprint(footer)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/screens/ -run TestCluster -v`
Expected: PASS (3 tests).

> If `components.NewConfirm`, `components.ConfirmResultMsg`, or `components.JSONView`'s `PageUp/PageDown/SetContent/SetSize` signatures differ from those used above, open `internal/tui/components/confirm.go` and `internal/tui/components/jsonview.go` and adjust the calls — the `Resource` screen in `resource.go` uses all of them and is the reference.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/screens/cluster.go internal/tui/screens/cluster_test.go
git commit -m "feat(tui): cluster operations dashboard screen"
```

### Task 3.3: Wire the Cluster tab into the app

Cluster becomes index 10; Settings moves to index 11.

**Files:**
- Modify: `internal/tui/app.go`

- [ ] **Step 1: Add the struct field**

In the `App` struct, after `analytics screens.Resource`:

```go
	cluster       screens.Cluster
```

- [ ] **Step 2: Add the tab title**

Insert `"Cluster"` before `"Settings"`:

```go
		tabs: []string{"Collections", "Aliases", "NL Models", "Curations", "Synonyms", "Stopwords", "Presets", "Conversations", "API Keys", "Analytics", "Cluster", "Settings"},
```

- [ ] **Step 3: Construct it in buildTabs**

```go
	a.cluster = screens.NewCluster(a.c, a.width, a.height-3)
```

- [ ] **Step 4: Resize / activate / update / view / route / modal-gate**

In the `tea.WindowSizeMsg` case:

```go
		a.cluster.SetSize(a.width, a.height-3)
```

In `activateTab` (Cluster = 10, Settings = 11):

```go
	case 10:
		return a.cluster.Init()
	case 11:
		return a.settings.Init()
```

In `Update`'s `switch a.active`:

```go
	case 10:
		a.cluster, cmd = a.cluster.Update(msg)
	case 11:
		a.settings, cmd = a.settings.Update(msg)
```

In `View`'s `switch a.active`:

```go
	case 10:
		body = a.cluster.View()
	case 11:
		body = a.settings.View()
```

In `routeTaggedAPIMessage` (handles both `Cluster:<panel>` reads and `Cluster:op:<name>` operations via the single prefix):

```go
	case strings.HasPrefix(tag, "Cluster:"):
		a.cluster, cmd = a.cluster.Update(msg)
```

In `activeHasModal`, add case 10 and move the settings `return true` to case 11:

```go
	case 10:
		return a.cluster.HasModal()
	case 11:
		// Settings is a form — always treat as modal.
		return true
```

- [ ] **Step 5: Build, full test, manual verify**

Run: `go build ./... && go test ./...`
Expected: clean build, all tests pass.

Manual (running Typesense): launch, `Tab` to "Cluster", confirm Health/Stats/Metrics/Debug populate (use arrow keys to switch panels, PgUp/PgDn to scroll), press `X` and confirm the clear-cache dialog appears and the response shows on confirm. **Do not** run `S`/`C`/`V` against a production cluster during testing.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat(tui): wire Cluster operations tab into app"
```

---

## Final verification (all phases)

- [ ] Run the full suite: `go test ./...` — all green.
- [ ] `go vet ./...` — no findings.
- [ ] `go build -o clisense .` — binary builds.
- [ ] Update `README.md`: add API Keys, Analytics, and Cluster to the feature list and note the new tab count / that tabs beyond 9 are reached via `Tab`.
- [ ] Commit the README update: `git commit -am "docs: document API Keys, Analytics, Cluster tabs"`.

## Follow-ups (out of scope, note in commit/PR body)

- Editable snapshot path for the Cluster snapshot operation (currently a constant).
- `0` → Settings digit shortcut now that there are >9 tabs.
- Analytics **events** endpoints (`POST/GET /analytics/events`) and a per-rule events viewer (`log`-type rules only).
- API key `expires_at`/`autodelete` helper fields surfaced in the template with inline guidance.
- Confirm whether `GET /analytics/rules/:name` exists; if list responses ever omit full rule bodies, add a per-item GET in `ExtractDetail`.

---

## Self-review notes

- **Spec coverage:** API Keys (list/create/delete + one-time secret, edit disabled) ✓; Analytics rules (list/create/upsert/delete) ✓; Cluster ops (health/stats/metrics/debug reads + snapshot/compact/vote/clear-cache confirmed POSTs) ✓. Analytics *events* explicitly deferred to follow-ups.
- **Type consistency:** Tag strings are consistent between screen and `routeTaggedAPIMessage` (`"API Keys:"`, `"Analytics:"`, `"Cluster:"` prefixes). `keysSaveTag` = `"API Keys:save"` matches `Resource.tag("save")` for `TabName: "API Keys"`. Cluster panel/op tags all share the `"Cluster:"` prefix so a single route case suffices. Endpoint helper names match between `endpoints.go`, the tests, and the screen constructors.
- **Index bookkeeping:** Each phase re-numbers the Settings tab (9 → 10 → 11). Implementers executing phases out of order must use whichever index Settings currently occupies; `settingsTabIndex()` stays dynamic so Esc handling is unaffected. The hard-coded digit handlers (`1`–`9`) are intentionally left unchanged.
- **Placeholders:** none — every code step contains full code; every run step has an exact command + expected result.
