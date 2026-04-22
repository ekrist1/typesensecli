# clisense — Typesense TUI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Bubble Tea TUI for managing a single Typesense instance — Collections (list/view), NL Search Models (CRUD), Curation Sets (CRUD), Conversation Models (CRUD + one-shot test).

**Architecture:** Thin `net/http` client returning raw bytes → Bubble Tea tab-bar UI with 2-pane screens (list + JSON detail) and a full-screen JSON editor overlay for create/update. Config persisted to `~/.config/clisense/config.yaml` (mode 0600).

**Tech Stack:** Go 1.25, Bubble Tea, Bubbles (textinput/textarea/list/viewport/spinner), Lip Gloss, `gopkg.in/yaml.v3`, stdlib `net/http` + `encoding/json`.

**Reference:** Typesense API docs at https://typesense.org/docs/30.2/api/ — consult for exact endpoint paths, request bodies, and response shapes. Where endpoint paths appear in this plan they should be verified against the docs during Task 4.

---

## File Structure

```
clisense/
├── main.go
├── go.mod / go.sum
├── internal/
│   ├── config/
│   │   ├── config.go              # Load, Save, Path, sentinel errors
│   │   └── config_test.go
│   ├── client/
│   │   ├── client.go              # Client struct, Do()
│   │   ├── client_test.go
│   │   └── endpoints.go           # named path/method helpers
│   ├── tui/
│   │   ├── app.go                 # root Model, routes keys to active tab
│   │   ├── styles.go              # lipgloss styles shared across screens
│   │   ├── messages.go            # shared tea.Msg types (apiSuccessMsg, apiErrorMsg)
│   │   ├── cmds.go                # doRequest tea.Cmd factory
│   │   ├── components/
│   │   │   ├── tabbar.go          # tab bar renderer
│   │   │   ├── jsonview.go        # read-only pretty-printed JSON pane
│   │   │   ├── jsoneditor.go      # full-screen JSON editor overlay
│   │   │   └── confirm.go         # yes/no confirmation modal
│   │   └── screens/
│   │       ├── setup.go           # first-run config form
│   │       ├── collections.go     # list + schema view (read-only)
│   │       ├── nlmodels.go        # CRUD
│   │       ├── curations.go       # CRUD
│   │       ├── conversations.go   # CRUD + test modal
│   │       └── settings.go        # edit URL / API key
│   └── templates/
│       ├── templates.go           # go:embed + accessors
│       ├── templates_test.go
│       ├── nlmodel.json
│       ├── curation.json
│       └── conversation.json
└── docs/superpowers/...           # spec + plan
```

---

## Task 1: Scaffold dependencies and directories

**Files:**
- Modify: `go.mod`
- Create: `internal/config/`, `internal/client/`, `internal/tui/components/`, `internal/tui/screens/`, `internal/templates/` (empty dirs via placeholder files below)

- [ ] **Step 1: Add dependencies**

Run:
```bash
cd /home/espen/godev/typesensecli
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/bubbles@latest
go get github.com/charmbracelet/lipgloss@latest
go get gopkg.in/yaml.v3@latest
```

Expected: `go.sum` is created; `go.mod` lists all four as direct dependencies.

- [ ] **Step 2: Verify module builds**

Run: `go build ./...`
Expected: no output, exit 0 (nothing to build yet — stdlib check).

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "scaffold: add Bubble Tea + YAML deps"
```

---

## Task 2: Config package (TDD)

**Files:**
- Create: `internal/config/config.go`
- Test: `internal/config/config_test.go`

Config file path resolution: `$XDG_CONFIG_HOME/clisense/config.yaml` if set, else `$HOME/.config/clisense/config.yaml`. For tests, allow override via a function parameter.

- [ ] **Step 1: Write failing tests**

Create `internal/config/config_test.go`:

```go
package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	in := Config{URL: "http://localhost:8108", APIKey: "xyz"}
	if err := Save(path, in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	out, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if out != in {
		t.Errorf("got %+v, want %+v", out, in)
	}
}

func TestLoadMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.yaml")
	_, err := Load(path)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestLoadCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corrupt.yaml")
	if err := os.WriteFile(path, []byte("this: is: not: yaml: ["), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if !errors.Is(err, ErrCorrupt) {
		t.Errorf("got %v, want ErrCorrupt", err)
	}
}

func TestSavePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Save(path, Config{URL: "u", APIKey: "k"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("got perm %o, want 0600", info.Mode().Perm())
	}
}
```

- [ ] **Step 2: Run tests, verify they fail**

Run: `go test ./internal/config/...`
Expected: FAIL — undefined `Config`, `Save`, `Load`, `ErrNotFound`, `ErrCorrupt`.

- [ ] **Step 3: Implement `internal/config/config.go`**

```go
// Package config reads and writes the clisense on-disk configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

var (
	ErrNotFound = errors.New("config not found")
	ErrCorrupt  = errors.New("config corrupt")
)

type Config struct {
	URL    string `yaml:"url"`
	APIKey string `yaml:"api_key"`
}

// DefaultPath returns the standard config path, honoring $XDG_CONFIG_HOME.
func DefaultPath() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "clisense", "config.yaml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "clisense", "config.yaml"), nil
}

func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, ErrNotFound
		}
		return Config{}, err
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return Config{}, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	return c, nil
}

func Save(path string, c Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0600)
}
```

- [ ] **Step 4: Run tests, verify they pass**

Run: `go test ./internal/config/... -v`
Expected: PASS on all four tests.

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): load/save YAML config with 0600 perms"
```

---

## Task 3: HTTP client (TDD)

**Files:**
- Create: `internal/client/client.go`
- Test: `internal/client/client_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/client/client_test.go`:

```go
package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDoSendsHeadersAndBody(t *testing.T) {
	var gotMethod, gotPath, gotAPIKey, gotContentType, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAPIKey = r.Header.Get("X-TYPESENSE-API-KEY")
		gotContentType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(201)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "secret")
	body, status, err := c.Do(context.Background(), "POST", "/collections", []byte(`{"name":"x"}`))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if status != 201 {
		t.Errorf("status=%d", status)
	}
	if string(body) != `{"ok":true}` {
		t.Errorf("body=%s", body)
	}
	if gotMethod != "POST" || gotPath != "/collections" {
		t.Errorf("method/path: %s %s", gotMethod, gotPath)
	}
	if gotAPIKey != "secret" {
		t.Errorf("api key header: %q", gotAPIKey)
	}
	if gotContentType != "application/json" {
		t.Errorf("content-type: %q", gotContentType)
	}
	if gotBody != `{"name":"x"}` {
		t.Errorf("body: %q", gotBody)
	}
}

func TestDoReturnsNon2xxWithoutError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(422)
		_, _ = w.Write([]byte(`{"message":"bad"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	body, status, err := c.Do(context.Background(), "GET", "/x", nil)
	if err != nil {
		t.Fatalf("unexpected error for 4xx: %v", err)
	}
	if status != 422 {
		t.Errorf("status=%d", status)
	}
	if !strings.Contains(string(body), "bad") {
		t.Errorf("body=%s", body)
	}
}

func TestDoTransportErrorReturnsError(t *testing.T) {
	c := New("http://127.0.0.1:1", "k") // port 1, nothing listens
	c.HTTP.Timeout = 200 * time.Millisecond
	_, _, err := c.Do(context.Background(), "GET", "/x", nil)
	if err == nil {
		t.Fatal("expected transport error, got nil")
	}
}

func TestDoRespectsContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	c := New(srv.URL, "k")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := c.Do(ctx, "GET", "/x", nil)
	if err == nil {
		t.Fatal("expected cancel error")
	}
}
```

- [ ] **Step 2: Run tests, verify they fail**

Run: `go test ./internal/client/...`
Expected: FAIL — undefined `New`, `Client.Do`.

- [ ] **Step 3: Implement `internal/client/client.go`**

```go
// Package client is a thin HTTP wrapper around the Typesense API.
package client

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

func New(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Do executes an API request. HTTP error statuses (4xx/5xx) are returned as
// (body, status, nil); only transport/context failures produce a non-nil error.
func (c *Client) Do(ctx context.Context, method, path string, body []byte) ([]byte, int, error) {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, rdr)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("X-TYPESENSE-API-KEY", c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return respBody, resp.StatusCode, nil
}
```

- [ ] **Step 4: Run tests, verify they pass**

Run: `go test ./internal/client/... -v`
Expected: PASS on all four tests.

- [ ] **Step 5: Commit**

```bash
git add internal/client/
git commit -m "feat(client): thin net/http Typesense client"
```

---

## Task 4: Endpoint path helpers

**Files:**
- Create: `internal/client/endpoints.go`

Purpose: centralize method + path strings so screens don't hand-build URLs. Each helper returns `(method, path)`. **Before implementing**, confirm the exact paths against https://typesense.org/docs/30.2/api/ — the values below are best-known but should be verified.

- [ ] **Step 1: Implement path helpers**

Create `internal/client/endpoints.go`:

```go
package client

import "fmt"

// Each helper returns (method, path) for a Typesense endpoint.
// Paths verified against https://typesense.org/docs/30.2/api/.

func ListCollections() (string, string)        { return "GET", "/collections" }
func GetCollection(name string) (string, string) {
	return "GET", fmt.Sprintf("/collections/%s", name)
}

func ListNLModels() (string, string)        { return "GET", "/nl_search_models" }
func CreateNLModel() (string, string)       { return "POST", "/nl_search_models" }
func UpdateNLModel(id string) (string, string) {
	return "PUT", fmt.Sprintf("/nl_search_models/%s", id)
}
func DeleteNLModel(id string) (string, string) {
	return "DELETE", fmt.Sprintf("/nl_search_models/%s", id)
}

func ListCurationSets() (string, string)        { return "GET", "/curation_sets" }
func GetCurationSet(name string) (string, string) {
	return "GET", fmt.Sprintf("/curation_sets/%s", name)
}
func UpsertCurationSet(name string) (string, string) {
	return "PUT", fmt.Sprintf("/curation_sets/%s", name)
}
func DeleteCurationSet(name string) (string, string) {
	return "DELETE", fmt.Sprintf("/curation_sets/%s", name)
}

func ListConversationModels() (string, string)      { return "GET", "/conversations/models" }
func CreateConversationModel() (string, string)     { return "POST", "/conversations/models" }
func UpdateConversationModel(id string) (string, string) {
	return "PUT", fmt.Sprintf("/conversations/models/%s", id)
}
func DeleteConversationModel(id string) (string, string) {
	return "DELETE", fmt.Sprintf("/conversations/models/%s", id)
}

// ConversationTest performs a search with a conversation model attached.
// Path and body shape verified against the Typesense Conversational Search docs.
func ConversationTest() (string, string) { return "POST", "/multi_search" }
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add internal/client/endpoints.go
git commit -m "feat(client): add Typesense endpoint path helpers"
```

---

## Task 5: JSON templates (embedded)

**Files:**
- Create: `internal/templates/templates.go`
- Create: `internal/templates/nlmodel.json`
- Create: `internal/templates/curation.json`
- Create: `internal/templates/conversation.json`
- Test: `internal/templates/templates_test.go`

- [ ] **Step 1: Create template files**

`internal/templates/nlmodel.json`:
```json
{
  "id": "my-nl-model",
  "model_name": "openai/gpt-4o",
  "api_key": "YOUR_OPENAI_KEY",
  "system_prompt": "You are a helpful search assistant.",
  "max_bytes": 16384
}
```

`internal/templates/curation.json`:
```json
{
  "items": [
    {
      "id": "promote-brand-x",
      "rule": { "query": "brand x", "match": "exact" },
      "includes": [ { "id": "product-123", "position": 1 } ],
      "excludes": []
    }
  ]
}
```

`internal/templates/conversation.json`:
```json
{
  "id": "my-conversation-model",
  "model_name": "openai/gpt-4o",
  "api_key": "YOUR_OPENAI_KEY",
  "system_prompt": "You answer questions based on search results.",
  "max_bytes": 16384,
  "history_collection": "conversation_store"
}
```

- [ ] **Step 2: Write failing test**

`internal/templates/templates_test.go`:
```go
package templates

import (
	"encoding/json"
	"testing"
)

func TestAllTemplatesValidJSON(t *testing.T) {
	for name, body := range All() {
		var v any
		if err := json.Unmarshal(body, &v); err != nil {
			t.Errorf("template %q invalid: %v", name, err)
		}
	}
}
```

- [ ] **Step 3: Run test, verify it fails**

Run: `go test ./internal/templates/...`
Expected: FAIL — undefined `All`.

- [ ] **Step 4: Implement `internal/templates/templates.go`**

```go
// Package templates exposes embedded JSON skeletons used by the TUI editor.
package templates

import _ "embed"

//go:embed nlmodel.json
var NLModel []byte

//go:embed curation.json
var Curation []byte

//go:embed conversation.json
var Conversation []byte

// All returns every template keyed by a short identifier.
func All() map[string][]byte {
	return map[string][]byte{
		"nlmodel":      NLModel,
		"curation":     Curation,
		"conversation": Conversation,
	}
}
```

- [ ] **Step 5: Run test, verify it passes**

Run: `go test ./internal/templates/... -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/templates/
git commit -m "feat(templates): embed JSON skeletons for resource creation"
```

---

## Task 6: Shared styles and messages

**Files:**
- Create: `internal/tui/styles.go`
- Create: `internal/tui/messages.go`
- Create: `internal/tui/cmds.go`

- [ ] **Step 1: Implement `internal/tui/styles.go`**

```go
package tui

import "github.com/charmbracelet/lipgloss"

var (
	ColorAccent    = lipgloss.Color("39")   // cyan-blue
	ColorMuted     = lipgloss.Color("244")  // grey
	ColorError     = lipgloss.Color("203")  // red
	ColorOK        = lipgloss.Color("42")   // green

	TabActive   = lipgloss.NewStyle().Bold(true).Foreground(ColorAccent).Padding(0, 1)
	TabInactive = lipgloss.NewStyle().Foreground(ColorMuted).Padding(0, 1)
	Border      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	Footer      = lipgloss.NewStyle().Foreground(ColorMuted)
	ErrorLine   = lipgloss.NewStyle().Foreground(ColorError)
	OKLine      = lipgloss.NewStyle().Foreground(ColorOK)
)
```

- [ ] **Step 2: Implement `internal/tui/messages.go`**

```go
package tui

// APISuccessMsg is emitted when an API call returns a 2xx status.
type APISuccessMsg struct {
	Tag    string // screen-provided identifier so the right screen reacts
	Status int
	Body   []byte
}

// APIErrorMsg is emitted on non-2xx status or transport failure.
type APIErrorMsg struct {
	Tag    string
	Status int    // 0 if transport failure
	Body   []byte // may be empty
	Err    error  // nil for HTTP status errors, non-nil for transport
}
```

- [ ] **Step 3: Implement `internal/tui/cmds.go`**

```go
package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/client"
)

// DoRequest builds a tea.Cmd that performs an API call and returns either
// APISuccessMsg or APIErrorMsg (tagged so only the originating screen reacts).
func DoRequest(c *client.Client, tag, method, path string, body []byte) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		respBody, status, err := c.Do(ctx, method, path, body)
		if err != nil {
			return APIErrorMsg{Tag: tag, Status: 0, Err: err}
		}
		if status >= 200 && status < 300 {
			return APISuccessMsg{Tag: tag, Status: status, Body: respBody}
		}
		return APIErrorMsg{Tag: tag, Status: status, Body: respBody}
	}
}
```

- [ ] **Step 4: Verify build**

Run: `go build ./...`
Expected: exit 0.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/styles.go internal/tui/messages.go internal/tui/cmds.go
git commit -m "feat(tui): shared styles, API messages, and request command"
```

---

## Task 7: JSON view component

**Files:**
- Create: `internal/tui/components/jsonview.go`

A read-only viewport that pretty-prints JSON bytes. Scroll with the keys bubbles `viewport` supports by default (arrows, pgup/pgdn).

- [ ] **Step 1: Implement component**

```go
// Package components holds reusable Bubble Tea subcomponents.
package components

import (
	"bytes"
	"encoding/json"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type JSONView struct {
	vp viewport.Model
}

func NewJSONView(width, height int) JSONView {
	return JSONView{vp: viewport.New(width, height)}
}

func (j *JSONView) SetSize(w, h int) {
	j.vp.Width = w
	j.vp.Height = h
}

// SetContent pretty-prints JSON bytes. Falls back to raw text if invalid.
func (j *JSONView) SetContent(body []byte) {
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, body, "", "  "); err == nil {
		j.vp.SetContent(pretty.String())
	} else {
		j.vp.SetContent(string(body))
	}
}

func (j *JSONView) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	j.vp, cmd = j.vp.Update(msg)
	return cmd
}

func (j *JSONView) View() string { return j.vp.View() }
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/components/jsonview.go
git commit -m "feat(tui): read-only JSON viewport component"
```

---

## Task 8: JSON editor overlay component

**Files:**
- Create: `internal/tui/components/jsoneditor.go`

Full-screen editor. `Ctrl+S` triggers a submit callback (the screen handles sending the request). `Esc` cancels. Shows a footer with the current error message when set.

- [ ] **Step 1: Implement editor**

```go
package components

import (
	"encoding/json"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

type JSONEditorSubmitMsg struct {
	Body []byte
}

type JSONEditorCancelMsg struct{}

type JSONEditor struct {
	ta    textarea.Model
	err   string
	title string
}

func NewJSONEditor(title string, initial []byte, width, height int) JSONEditor {
	ta := textarea.New()
	ta.SetValue(string(initial))
	ta.SetWidth(width)
	ta.SetHeight(height - 4) // leave room for title + footer
	ta.Focus()
	return JSONEditor{ta: ta, title: title}
}

func (e *JSONEditor) SetSize(w, h int) {
	e.ta.SetWidth(w)
	e.ta.SetHeight(h - 4)
}

func (e *JSONEditor) SetError(msg string) { e.err = msg }

// Update handles key input. Returns a tea.Cmd if a submit or cancel occurs
// (caller receives JSONEditorSubmitMsg / JSONEditorCancelMsg as tea.Msg).
func (e *JSONEditor) Update(msg tea.Msg) tea.Cmd {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			return func() tea.Msg { return JSONEditorCancelMsg{} }
		case "ctrl+s":
			body := []byte(e.ta.Value())
			var v any
			if err := json.Unmarshal(body, &v); err != nil {
				e.err = "invalid JSON: " + err.Error()
				return nil
			}
			e.err = ""
			return func() tea.Msg { return JSONEditorSubmitMsg{Body: body} }
		}
	}
	var cmd tea.Cmd
	e.ta, cmd = e.ta.Update(msg)
	return cmd
}

func (e *JSONEditor) View() string {
	footer := "Ctrl+S save · Esc cancel"
	if e.err != "" {
		footer = e.err
	}
	return e.title + "\n" + e.ta.View() + "\n" + footer
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/components/jsoneditor.go
git commit -m "feat(tui): full-screen JSON editor component"
```

---

## Task 9: Confirmation modal component

**Files:**
- Create: `internal/tui/components/confirm.go`

- [ ] **Step 1: Implement**

```go
package components

import tea "github.com/charmbracelet/bubbletea"

type ConfirmResultMsg struct {
	Confirmed bool
	Tag       string // so callers can disambiguate which confirm returned
}

type Confirm struct {
	Prompt string
	Tag    string
}

func NewConfirm(prompt, tag string) Confirm { return Confirm{Prompt: prompt, Tag: tag} }

func (c Confirm) Update(msg tea.Msg) tea.Cmd {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "y", "Y":
			return func() tea.Msg { return ConfirmResultMsg{Confirmed: true, Tag: c.Tag} }
		case "n", "N", "esc":
			return func() tea.Msg { return ConfirmResultMsg{Confirmed: false, Tag: c.Tag} }
		}
	}
	return nil
}

func (c Confirm) View() string {
	return c.Prompt + "\n\n[y] yes   [n] no"
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/components/confirm.go
git commit -m "feat(tui): yes/no confirmation modal"
```

---

## Task 10: Tab bar component

**Files:**
- Create: `internal/tui/components/tabbar.go`

- [ ] **Step 1: Implement**

```go
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
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/components/tabbar.go
git commit -m "feat(tui): tab bar rendering component"
```

---

## Task 11: Setup screen (first-run config form)

**Files:**
- Create: `internal/tui/screens/setup.go`

Shown when `config.Load` returns `ErrNotFound` or `ErrCorrupt`, or when the user opens the Settings tab for editing. Emits a `SetupDoneMsg` with the chosen `Config` on submit.

- [ ] **Step 1: Implement**

```go
// Package screens contains the tab-level Bubble Tea models.
package screens

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/config"
)

type SetupDoneMsg struct {
	Cfg config.Config
}

type Setup struct {
	url    textinput.Model
	apiKey textinput.Model
	focus  int // 0 = url, 1 = apiKey
	err    string
}

func NewSetup(initial config.Config) Setup {
	url := textinput.New()
	url.Placeholder = "http://localhost:8108"
	url.SetValue(initial.URL)
	url.Focus()

	key := textinput.New()
	key.Placeholder = "API key"
	key.SetValue(initial.APIKey)
	key.EchoMode = textinput.EchoPassword

	return Setup{url: url, apiKey: key, focus: 0}
}

func (s Setup) Init() tea.Cmd { return textinput.Blink }

func (s Setup) Update(msg tea.Msg) (Setup, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "tab", "shift+tab":
			s.focus = 1 - s.focus
			if s.focus == 0 {
				s.url.Focus()
				s.apiKey.Blur()
			} else {
				s.apiKey.Focus()
				s.url.Blur()
			}
			return s, nil
		case "enter":
			if s.url.Value() == "" || s.apiKey.Value() == "" {
				s.err = "both fields are required"
				return s, nil
			}
			cfg := config.Config{URL: s.url.Value(), APIKey: s.apiKey.Value()}
			return s, func() tea.Msg { return SetupDoneMsg{Cfg: cfg} }
		}
	}
	var cmd tea.Cmd
	if s.focus == 0 {
		s.url, cmd = s.url.Update(msg)
	} else {
		s.apiKey, cmd = s.apiKey.Update(msg)
	}
	return s, cmd
}

func (s Setup) View() string {
	v := "clisense — Typesense connection\n\n"
	v += "URL:     " + s.url.View() + "\n"
	v += "API key: " + s.apiKey.View() + "\n\n"
	if s.err != "" {
		v += s.err + "\n"
	}
	v += "Tab switch · Enter save · Ctrl+C quit"
	return v
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/screens/setup.go
git commit -m "feat(tui): setup screen for first-run config"
```

---

## Task 12: Collections screen (read-only, list + schema)

**Files:**
- Create: `internal/tui/screens/collections.go`

Uses `bubbles/list` for the left pane and the `JSONView` component for the right. `r` refreshes. No `n`/`e`/`d`.

- [ ] **Step 1: Implement**

```go
package screens

import (
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/client"
	"clisense/internal/tui"
	"clisense/internal/tui/components"
)

const tagCollections = "collections"
const tagCollectionDetail = "collection-detail"

type collectionItem struct{ name string }

func (c collectionItem) Title() string       { return c.name }
func (c collectionItem) Description() string { return "" }
func (c collectionItem) FilterValue() string { return c.name }

type Collections struct {
	c       *client.Client
	list    list.Model
	detail  components.JSONView
	width   int
	height  int
	status  string
}

func NewCollections(c *client.Client, w, h int) Collections {
	l := list.New(nil, list.NewDefaultDelegate(), w/2, h-4)
	l.Title = "Collections"
	l.SetShowStatusBar(false)
	return Collections{c: c, list: l, detail: components.NewJSONView(w/2, h-4), width: w, height: h}
}

func (s Collections) Init() tea.Cmd { return s.fetchList() }

func (s *Collections) SetSize(w, h int) {
	s.width, s.height = w, h
	s.list.SetSize(w/2, h-4)
	s.detail.SetSize(w/2, h-4)
}

func (s Collections) fetchList() tea.Cmd {
	m, p := client.ListCollections()
	return tui.DoRequest(s.c, tagCollections, m, p, nil)
}

func (s Collections) fetchDetail(name string) tea.Cmd {
	m, p := client.GetCollection(name)
	return tui.DoRequest(s.c, tagCollectionDetail, m, p, nil)
}

func (s Collections) Update(msg tea.Msg) (Collections, tea.Cmd) {
	switch m := msg.(type) {
	case tui.APISuccessMsg:
		switch m.Tag {
		case tagCollections:
			var arr []map[string]any
			if err := json.Unmarshal(m.Body, &arr); err != nil {
				s.status = "parse error: " + err.Error()
				return s, nil
			}
			items := make([]list.Item, 0, len(arr))
			for _, c := range arr {
				if name, ok := c["name"].(string); ok {
					items = append(items, collectionItem{name: name})
				}
			}
			s.list.SetItems(items)
			s.status = fmt.Sprintf("%d collections", len(items))
			return s, nil
		case tagCollectionDetail:
			s.detail.SetContent(m.Body)
			return s, nil
		}
	case tui.APIErrorMsg:
		if m.Err != nil {
			s.status = "network error: " + m.Err.Error()
		} else {
			s.status = fmt.Sprintf("HTTP %d: %s", m.Status, string(m.Body))
		}
		return s, nil
	case tea.KeyMsg:
		switch m.String() {
		case "r":
			return s, s.fetchList()
		case "enter":
			if it, ok := s.list.SelectedItem().(collectionItem); ok {
				return s, s.fetchDetail(it.name)
			}
		}
	}
	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)
	return s, cmd
}

func (s Collections) View() string {
	footer := "Enter view schema · r refresh · Esc back"
	if s.status != "" {
		footer = s.status + " · " + footer
	}
	return s.list.View() + "    " + s.detail.View() + "\n" + footer
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/screens/collections.go
git commit -m "feat(tui): collections list + schema view"
```

---

## Task 13: Generic resource CRUD screen

**Files:**
- Create: `internal/tui/screens/resource.go`

The NL Models, Curations, and Conversations tabs share the same pattern (list + detail + `n`/`e`/`d`). A reusable `Resource` type keeps per-tab code tiny. Per-resource behavior is passed in via a `ResourceStrategy`.

Some resources are keyed by an ID the user provides up front (Curation Sets use PUT `/curation_sets/{name}` for both create and update). For those, set `CreateNamePrompt: true` and leave `Create` nil — the screen asks for a name first, then calls `Update(name)` when the editor submits.

- [ ] **Step 1: Implement `internal/tui/screens/resource.go`**

```go
package screens

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/client"
	"clisense/internal/tui"
	"clisense/internal/tui/components"
)

// ResourceStrategy configures how a CRUD resource screen talks to the API.
type ResourceStrategy struct {
	TabName  string
	Template []byte

	List   func() (string, string)
	Create func() (string, string)            // may be nil when CreateNamePrompt is true
	Update func(id string) (string, string)
	Delete func(id string) (string, string)

	// When true, `n` first asks for a name, then calls Update(name) on submit.
	CreateNamePrompt bool

	// ExtractItems parses the list response and returns display IDs.
	ExtractItems func(body []byte) ([]string, error)
	// ExtractDetail selects the JSON payload for a single ID from the list body.
	// Most Typesense list endpoints embed full records, so no per-item GET is needed.
	// Returns nil if not found.
	ExtractDetail func(body []byte, id string) []byte
}

type Resource struct {
	c       *client.Client
	strat   ResourceStrategy
	list    list.Model
	detail  components.JSONView
	editor  *components.JSONEditor
	confirm *components.Confirm

	namePrompt  *textinput.Model // open when user is naming a new upsert-by-name item
	pendingName string           // name captured by namePrompt, used on editor submit

	rawList       []byte
	status        string
	width, height int
	isEditing     bool
}

type resourceItem struct{ id string }

func (r resourceItem) Title() string       { return r.id }
func (r resourceItem) Description() string { return "" }
func (r resourceItem) FilterValue() string { return r.id }

func NewResource(c *client.Client, s ResourceStrategy, w, h int) Resource {
	l := list.New(nil, list.NewDefaultDelegate(), w/2, h-4)
	l.Title = s.TabName
	l.SetShowStatusBar(false)
	return Resource{
		c:      c,
		strat:  s,
		list:   l,
		detail: components.NewJSONView(w/2, h-4),
		width:  w, height: h,
	}
}

func (r *Resource) SetSize(w, h int) {
	r.width, r.height = w, h
	r.list.SetSize(w/2, h-4)
	r.detail.SetSize(w/2, h-4)
	if r.editor != nil {
		r.editor.SetSize(w, h)
	}
}

func (r Resource) tag(op string) string { return r.strat.TabName + ":" + op }

func (r Resource) Init() tea.Cmd { return r.fetchList() }

func (r Resource) fetchList() tea.Cmd {
	m, p := r.strat.List()
	return tui.DoRequest(r.c, r.tag("list"), m, p, nil)
}

func (r Resource) Update(msg tea.Msg) (Resource, tea.Cmd) {
	// Name prompt modal — open when creating an upsert-by-name resource.
	if r.namePrompt != nil {
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "esc":
				r.namePrompt = nil
				return r, nil
			case "enter":
				name := r.namePrompt.Value()
				if name == "" {
					return r, nil
				}
				r.pendingName = name
				r.namePrompt = nil
				ed := components.NewJSONEditor("New "+r.strat.TabName+": "+name, r.strat.Template, r.width, r.height)
				r.editor = &ed
				r.isEditing = false
				return r, nil
			}
		}
		updated, cmd := r.namePrompt.Update(msg)
		r.namePrompt = &updated
		return r, cmd
	}

	// Editor modal.
	if r.editor != nil {
		cmd := r.editor.Update(msg)
		switch m := msg.(type) {
		case components.JSONEditorCancelMsg:
			r.editor = nil
			r.pendingName = ""
			r.isEditing = false
			return r, nil
		case components.JSONEditorSubmitMsg:
			var method, path string
			switch {
			case r.pendingName != "":
				method, path = r.strat.Update(r.pendingName)
				r.pendingName = ""
			case r.isEditing:
				if it, ok := r.list.SelectedItem().(resourceItem); ok {
					method, path = r.strat.Update(it.id)
				}
			default:
				if r.strat.Create == nil {
					r.status = "create not supported (missing name)"
					r.editor = nil
					return r, nil
				}
				method, path = r.strat.Create()
			}
			r.editor = nil
			r.isEditing = false
			return r, tui.DoRequest(r.c, r.tag("save"), method, path, m.Body)
		}
		return r, cmd
	}

	// Confirm modal.
	if r.confirm != nil {
		cmd := r.confirm.Update(msg)
		if res, ok := msg.(components.ConfirmResultMsg); ok && res.Tag == r.tag("delete") {
			r.confirm = nil
			if res.Confirmed {
				if it, ok := r.list.SelectedItem().(resourceItem); ok {
					m, p := r.strat.Delete(it.id)
					return r, tui.DoRequest(r.c, r.tag("delete"), m, p, nil)
				}
			}
			return r, nil
		}
		return r, cmd
	}

	switch m := msg.(type) {
	case tui.APISuccessMsg:
		switch m.Tag {
		case r.tag("list"):
			r.rawList = m.Body
			ids, err := r.strat.ExtractItems(m.Body)
			if err != nil {
				r.status = "parse error: " + err.Error()
				return r, nil
			}
			items := make([]list.Item, 0, len(ids))
			for _, id := range ids {
				items = append(items, resourceItem{id: id})
			}
			r.list.SetItems(items)
			r.status = fmt.Sprintf("%d items", len(items))
			if len(ids) > 0 {
				if d := r.strat.ExtractDetail(r.rawList, ids[0]); d != nil {
					r.detail.SetContent(d)
				}
			}
			return r, nil
		case r.tag("save"), r.tag("delete"):
			r.status = "OK"
			return r, r.fetchList()
		}
	case tui.APIErrorMsg:
		if m.Err != nil {
			r.status = "network error: " + m.Err.Error()
		} else {
			r.status = fmt.Sprintf("HTTP %d: %s", m.Status, string(m.Body))
		}
		return r, nil
	case tea.KeyMsg:
		switch m.String() {
		case "r":
			return r, r.fetchList()
		case "n":
			if r.strat.CreateNamePrompt {
				ti := textinput.New()
				ti.Placeholder = "name"
				ti.Focus()
				r.namePrompt = &ti
				return r, textinput.Blink
			}
			ed := components.NewJSONEditor("New "+r.strat.TabName, r.strat.Template, r.width, r.height)
			r.editor = &ed
			r.isEditing = false
			return r, nil
		case "e":
			if it, ok := r.list.SelectedItem().(resourceItem); ok {
				body := r.strat.ExtractDetail(r.rawList, it.id)
				if body == nil {
					body = r.strat.Template
				}
				ed := components.NewJSONEditor("Edit "+it.id, body, r.width, r.height)
				r.editor = &ed
				r.isEditing = true
			}
			return r, nil
		case "d":
			if it, ok := r.list.SelectedItem().(resourceItem); ok {
				cf := components.NewConfirm("Delete "+it.id+"?", r.tag("delete"))
				r.confirm = &cf
			}
			return r, nil
		case "enter":
			if it, ok := r.list.SelectedItem().(resourceItem); ok {
				if d := r.strat.ExtractDetail(r.rawList, it.id); d != nil {
					r.detail.SetContent(d)
				}
			}
			return r, nil
		}
	}

	var cmd tea.Cmd
	r.list, cmd = r.list.Update(msg)
	if it, ok := r.list.SelectedItem().(resourceItem); ok {
		if d := r.strat.ExtractDetail(r.rawList, it.id); d != nil {
			r.detail.SetContent(d)
		}
	}
	return r, cmd
}

func (r Resource) View() string {
	if r.namePrompt != nil {
		return "Name for new " + r.strat.TabName + ":\n\n" + r.namePrompt.View() + "\n\nEnter confirm · Esc cancel"
	}
	if r.editor != nil {
		return r.editor.View()
	}
	if r.confirm != nil {
		return r.confirm.View()
	}
	footer := "n new · e edit · d delete · r refresh · Esc back"
	if r.status != "" {
		footer = r.status + " · " + footer
	}
	return r.list.View() + "    " + r.detail.View() + "\n" + footer
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/screens/resource.go
git commit -m "feat(tui): generic CRUD resource screen with editor + confirm modals"
```

---

## Task 14: NL Models tab

**Files:**
- Create: `internal/tui/screens/nlmodels.go`

- [ ] **Step 1: Implement**

```go
package screens

import (
	"encoding/json"

	"clisense/internal/client"
	"clisense/internal/templates"
)

// NewNLModels returns a Resource screen configured for NL search models.
// The list endpoint returns an array of model objects with an "id" field.
func NewNLModels(c *client.Client, w, h int) Resource {
	return NewResource(c, ResourceStrategy{
		TabName:  "NL Models",
		Template: templates.NLModel,
		List:     client.ListNLModels,
		Create:   client.CreateNLModel,
		Update:   client.UpdateNLModel,
		Delete:   client.DeleteNLModel,
		ExtractItems: func(body []byte) ([]string, error) {
			var arr []map[string]any
			if err := json.Unmarshal(body, &arr); err != nil {
				return nil, err
			}
			ids := make([]string, 0, len(arr))
			for _, m := range arr {
				if id, ok := m["id"].(string); ok {
					ids = append(ids, id)
				}
			}
			return ids, nil
		},
		ExtractDetail: func(body []byte, id string) []byte {
			var arr []map[string]any
			if err := json.Unmarshal(body, &arr); err != nil {
				return nil
			}
			for _, m := range arr {
				if m["id"] == id {
					b, _ := json.MarshalIndent(m, "", "  ")
					return b
				}
			}
			return nil
		},
	}, w, h)
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/screens/nlmodels.go
git commit -m "feat(tui): NL search models tab"
```

---

## Task 15: Curations tab

**Files:**
- Create: `internal/tui/screens/curations.go`

Curation sets are keyed by `name`. Typesense uses `PUT /curation_sets/{name}` for both create and update, so we use the name-prompt path added in Task 13 (`CreateNamePrompt: true`, `Create: nil`).

- [ ] **Step 1: Implement**

```go
package screens

import (
	"encoding/json"

	"clisense/internal/client"
	"clisense/internal/templates"
)

func NewCurations(c *client.Client, w, h int) Resource {
	return NewResource(c, ResourceStrategy{
		TabName:          "Curations",
		Template:         templates.Curation,
		List:             client.ListCurationSets,
		Create:           nil, // upsert-by-name: name captured before editor opens
		Update:           client.UpsertCurationSet,
		Delete:           client.DeleteCurationSet,
		CreateNamePrompt: true,
		ExtractItems: func(body []byte) ([]string, error) {
			var arr []map[string]any
			if err := json.Unmarshal(body, &arr); err != nil {
				return nil, err
			}
			ids := make([]string, 0, len(arr))
			for _, m := range arr {
				if name, ok := m["name"].(string); ok {
					ids = append(ids, name)
				}
			}
			return ids, nil
		},
		ExtractDetail: func(body []byte, name string) []byte {
			var arr []map[string]any
			if err := json.Unmarshal(body, &arr); err != nil {
				return nil
			}
			for _, m := range arr {
				if m["name"] == name {
					b, _ := json.MarshalIndent(m, "", "  ")
					return b
				}
			}
			return nil
		},
	}, w, h)
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/screens/curations.go
git commit -m "feat(tui): curation sets tab with name-prompt upsert"
```

---

## Task 16: Conversations tab + test modal

**Files:**
- Create: `internal/tui/screens/conversations.go`

Conversation models use `id`, full CRUD via the generic `Resource`. The extra `t` key opens a simple query modal that submits a `/multi_search` request with `conversation_model_id` set.

- [ ] **Step 1: Implement**

```go
package screens

import (
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/client"
	"clisense/internal/templates"
	"clisense/internal/tui"
	"clisense/internal/tui/components"
)

type Conversations struct {
	inner Resource

	testCollection *textinput.Model
	testQuery      *textarea.Model
	testResult     *components.JSONView
	focus          int // 0 = collection, 1 = query
}

func NewConversations(c *client.Client, w, h int) Conversations {
	inner := NewResource(c, ResourceStrategy{
		TabName:  "Conversations",
		Template: templates.Conversation,
		List:     client.ListConversationModels,
		Create:   client.CreateConversationModel,
		Update:   client.UpdateConversationModel,
		Delete:   client.DeleteConversationModel,
		ExtractItems: func(body []byte) ([]string, error) {
			var arr []map[string]any
			if err := json.Unmarshal(body, &arr); err != nil {
				return nil, err
			}
			ids := make([]string, 0, len(arr))
			for _, m := range arr {
				if id, ok := m["id"].(string); ok {
					ids = append(ids, id)
				}
			}
			return ids, nil
		},
		ExtractDetail: func(body []byte, id string) []byte {
			var arr []map[string]any
			if err := json.Unmarshal(body, &arr); err != nil {
				return nil
			}
			for _, m := range arr {
				if m["id"] == id {
					b, _ := json.MarshalIndent(m, "", "  ")
					return b
				}
			}
			return nil
		},
	}, w, h)
	return Conversations{inner: inner}
}

func (s *Conversations) SetSize(w, h int) { s.inner.SetSize(w, h) }

func (s Conversations) Init() tea.Cmd { return s.inner.Init() }

const tagConvTest = "conversations:test"

func (s Conversations) Update(msg tea.Msg) (Conversations, tea.Cmd) {
	// Test modal open — route all keys to it.
	if s.testQuery != nil {
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "esc":
				s.testQuery, s.testCollection, s.testResult, s.focus = nil, nil, nil, 0
				return s, nil
			case "tab", "shift+tab":
				s.focus = 1 - s.focus
				if s.focus == 0 {
					s.testCollection.Focus()
					s.testQuery.Blur()
				} else {
					s.testCollection.Blur()
					s.testQuery.Focus()
				}
				return s, nil
			case "ctrl+s":
				it, ok := s.inner.list.SelectedItem().(resourceItem)
				if !ok {
					return s, nil
				}
				reqBody := map[string]any{
					"searches": []map[string]any{{
						"collection":             s.testCollection.Value(),
						"q":                      s.testQuery.Value(),
						"query_by":               "*",
						"conversation":           true,
						"conversation_model_id":  it.id,
					}},
				}
				b, _ := json.Marshal(reqBody)
				m, p := client.ConversationTest()
				return s, tui.DoRequest(s.inner.c, tagConvTest, m, p, b)
			}
		}
		var cmd tea.Cmd
		if s.focus == 0 {
			*s.testCollection, cmd = s.testCollection.Update(msg)
		} else {
			*s.testQuery, cmd = s.testQuery.Update(msg)
		}
		return s, cmd
	}

	// Intercept the "t" key when inner has no modal open.
	if km, ok := msg.(tea.KeyMsg); ok && km.String() == "t" && s.inner.editor == nil && s.inner.confirm == nil {
		ti := textinput.New()
		ti.Placeholder = "collection name"
		ti.Focus()
		ta := textarea.New()
		ta.Placeholder = "query"
		ta.SetWidth(s.inner.width - 4)
		ta.SetHeight(4)
		rv := components.NewJSONView(s.inner.width-4, s.inner.height-12)
		s.testCollection = &ti
		s.testQuery = &ta
		s.testResult = &rv
		s.focus = 0
		return s, textinput.Blink
	}

	// Surface test response back into the modal viewer.
	switch m := msg.(type) {
	case tui.APISuccessMsg:
		if m.Tag == tagConvTest && s.testResult != nil {
			s.testResult.SetContent(m.Body)
			return s, nil
		}
	case tui.APIErrorMsg:
		if m.Tag == tagConvTest && s.testResult != nil {
			s.testResult.SetContent([]byte(fmt.Sprintf(`{"error":"HTTP %d","body":%q}`, m.Status, string(m.Body))))
			return s, nil
		}
	}

	var cmd tea.Cmd
	s.inner, cmd = s.inner.Update(msg)
	return s, cmd
}

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
	// Append 't' to the footer via reuse of inner.View() — simplest is to
	// render inner.View() and let the user discover the key in help.
	return s.inner.View()
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/screens/conversations.go
git commit -m "feat(tui): conversation models tab with one-shot test modal"
```

---

## Task 17: Settings tab

**Files:**
- Create: `internal/tui/screens/settings.go`

Thin wrapper around the `Setup` screen that edits existing config and returns a `SetupDoneMsg`. Root model reacts by saving to disk and rebuilding the client.

- [ ] **Step 1: Implement**

```go
package screens

import (
	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/config"
)

type Settings struct {
	setup Setup
}

func NewSettings(cur config.Config) Settings { return Settings{setup: NewSetup(cur)} }

func (s Settings) Init() tea.Cmd { return s.setup.Init() }

func (s Settings) Update(msg tea.Msg) (Settings, tea.Cmd) {
	var cmd tea.Cmd
	s.setup, cmd = s.setup.Update(msg)
	return s, cmd
}

func (s Settings) View() string { return s.setup.View() }
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/screens/settings.go
git commit -m "feat(tui): settings tab reusing setup form"
```

---

## Task 18: Root app model (tab routing)

**Files:**
- Create: `internal/tui/app.go`

- [ ] **Step 1: Implement**

```go
package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/client"
	"clisense/internal/config"
	"clisense/internal/tui/components"
	"clisense/internal/tui/screens"
)

type App struct {
	cfg        config.Config
	cfgPath    string
	c          *client.Client
	inSetup    bool
	setup      screens.Setup
	active     int
	width      int
	height     int

	collections   screens.Collections
	nlModels      screens.Resource
	curations     screens.Resource
	conversations screens.Conversations
	settings      screens.Settings

	tabs []string
}

func NewApp(cfg config.Config, cfgPath string, inSetup bool) App {
	a := App{
		cfg:     cfg,
		cfgPath: cfgPath,
		inSetup: inSetup,
		setup:   screens.NewSetup(cfg),
		tabs:    []string{"Collections", "NL Models", "Curations", "Conversations", "Settings"},
		width:   100,
		height:  30,
	}
	if !inSetup {
		a.c = client.New(cfg.URL, cfg.APIKey)
		a.buildTabs()
	}
	return a
}

func (a *App) buildTabs() {
	a.collections = screens.NewCollections(a.c, a.width, a.height-3)
	a.nlModels = screens.NewNLModels(a.c, a.width, a.height-3)
	a.curations = screens.NewCurations(a.c, a.width, a.height-3)
	a.conversations = screens.NewConversations(a.c, a.width, a.height-3)
	a.settings = screens.NewSettings(a.cfg)
}

func (a App) Init() tea.Cmd {
	if a.inSetup {
		return a.setup.Init()
	}
	return a.collections.Init()
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = m.Width, m.Height
		a.collections.SetSize(a.width, a.height-3)
		a.nlModels.SetSize(a.width, a.height-3)
		a.curations.SetSize(a.width, a.height-3)
		a.conversations.SetSize(a.width, a.height-3)
		return a, nil
	case screens.SetupDoneMsg:
		a.cfg = m.Cfg
		if err := config.Save(a.cfgPath, a.cfg); err != nil {
			// Surfacing errors in setup screen is out of scope for v1; log to stderr.
			return a, nil
		}
		a.c = client.New(a.cfg.URL, a.cfg.APIKey)
		a.inSetup = false
		a.buildTabs()
		return a, a.collections.Init()
	case tea.KeyMsg:
		if a.inSetup {
			var cmd tea.Cmd
			a.setup, cmd = a.setup.Update(msg)
			return a, cmd
		}
		switch m.String() {
		case "ctrl+c", "q":
			return a, tea.Quit
		case "1":
			a.active = 0
			return a, a.collections.Init()
		case "2":
			a.active = 1
			return a, a.nlModels.Init()
		case "3":
			a.active = 2
			return a, a.curations.Init()
		case "4":
			a.active = 3
			return a, a.conversations.Init()
		case "5":
			a.active = 4
			return a, a.settings.Init()
		case "tab":
			a.active = (a.active + 1) % len(a.tabs)
			return a, nil
		case "shift+tab":
			a.active = (a.active - 1 + len(a.tabs)) % len(a.tabs)
			return a, nil
		}
	}

	if a.inSetup {
		var cmd tea.Cmd
		a.setup, cmd = a.setup.Update(msg)
		return a, cmd
	}

	var cmd tea.Cmd
	switch a.active {
	case 0:
		a.collections, cmd = a.collections.Update(msg)
	case 1:
		a.nlModels, cmd = a.nlModels.Update(msg)
	case 2:
		a.curations, cmd = a.curations.Update(msg)
	case 3:
		a.conversations, cmd = a.conversations.Update(msg)
	case 4:
		a.settings, cmd = a.settings.Update(msg)
	}
	return a, cmd
}

func (a App) View() string {
	if a.inSetup {
		return a.setup.View()
	}
	bar := components.TabBar{Titles: a.tabs, Active: a.active}
	bar.Style.Active = TabActive
	bar.Style.Inactive = TabInactive
	body := ""
	switch a.active {
	case 0:
		body = a.collections.View()
	case 1:
		body = a.nlModels.View()
	case 2:
		body = a.curations.View()
	case 3:
		body = a.conversations.View()
	case 4:
		body = a.settings.View()
	}
	return bar.View() + "\n\n" + body
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: exit 0.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat(tui): root app model with tab routing"
```

---

## Task 19: main.go wire-up

**Files:**
- Create: `main.go`

- [ ] **Step 1: Implement**

```go
// Command clisense is a terminal UI for managing a Typesense instance.
package main

import (
	"errors"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/config"
	"clisense/internal/tui"
)

func main() {
	path, err := config.DefaultPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config path error:", err)
		os.Exit(1)
	}
	cfg, err := config.Load(path)
	inSetup := false
	switch {
	case err == nil:
		// ok
	case errors.Is(err, config.ErrNotFound), errors.Is(err, config.ErrCorrupt):
		inSetup = true
	default:
		fmt.Fprintln(os.Stderr, "config load error:", err)
		os.Exit(1)
	}

	app := tui.NewApp(cfg, path, inSetup)
	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "tui error:", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Build and run a quick sanity check**

Run: `go build -o clisense . && ls -la clisense`
Expected: binary exists.

Run: `./clisense` in a terminal — the setup screen should appear when no config is present. Enter bogus values and verify it lands on the Collections tab (which will show a network error against a non-existent server — that's fine for now, just confirm no crash).

- [ ] **Step 3: Commit**

```bash
git add main.go
git commit -m "feat: wire main entrypoint to TUI app"
```

---

## Task 20: Manual smoke test against a live Typesense

This task is not code — it's the acceptance checklist from the spec. Run against a local or remote Typesense instance with real credentials.

- [ ] **1. Fresh config flow**
  - Remove `~/.config/clisense/config.yaml`.
  - Run `./clisense` — setup screen appears.
  - Enter URL + API key, press Enter — Collections tab loads with real collections.
  - Confirm `ls -la ~/.config/clisense/config.yaml` shows mode `-rw-------`.

- [ ] **2. Collections tab**
  - Press `1`. List populates. `Enter` on a row shows the schema JSON in the right pane.
  - Press `r` — list refreshes.

- [ ] **3. NL Models tab (CRUD)**
  - Press `2`. Create with `n`, edit with `e`, delete with `d` — each round-trips against the server.

- [ ] **4. Curations tab (CRUD)**
  - Press `3`. Press `n`, enter a name, edit the JSON, save — verify the new set appears.
  - Press `e` on an existing set, modify, save — verify change persists.
  - Press `d`, confirm — verify removal.

- [ ] **5. Conversations tab (CRUD + test)**
  - Press `4`. CRUD the same way.
  - Select a model, press `t`. Enter collection name + query. `Ctrl+S`. A JSON response renders.
  - `Esc` closes the modal.

- [ ] **6. Error surfacing**
  - In Settings (tab `5`), change the API key to an invalid one and save.
  - Return to any resource tab — the error HTTP status is visible in the footer, app does not crash.

- [ ] **7. Commit**

No code changes to commit; if any bug fixes were needed during smoke test, commit them as follow-ups named `fix(smoke): ...`.

---

## Self-review notes

The spec sections mapped to tasks:

- Config (spec §Connection & configuration) → Task 2.
- HTTP client + endpoints (spec §HTTP client, §Architecture) → Tasks 3–4.
- Templates (spec §JSON editor overlay) → Task 5.
- Styles, messages, commands (spec §Data flow) → Task 6.
- JSON view + editor + confirm + tabbar components → Tasks 7–10.
- Setup screen (spec §UX → Setup) → Task 11.
- Collections read-only (spec §Collections tab) → Task 12.
- Generic CRUD + name-prompt variant (spec §Common resource-tab layout, §NL/Curations/Conversations) → Task 13.
- NL Models / Curations / Conversations / Settings tabs → Tasks 14–17.
- Root tab router (spec §Tab bar) → Task 18.
- Entrypoint with first-run routing (spec §Setup screen) → Task 19.
- Manual smoke tests (spec §Testing → Manual smoke-test checklist) → Task 20.

Error handling treatments from the spec (Local JSON parse / HTTP / Network) are implemented in: `jsoneditor.go` (local parse), `resource.go`/`collections.go` (HTTP + network via `APIErrorMsg`).
