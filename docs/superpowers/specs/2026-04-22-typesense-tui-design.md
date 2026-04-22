# clisense — Typesense TUI Design

**Date:** 2026-04-22
**Module:** `clisense`
**Go version:** 1.25.9
**Stack:** Bubble Tea / Bubbles / Lip Gloss (Charm)

## Purpose

A terminal UI for managing a single Typesense instance. Provides focused
workflows for the resources a developer interacts with day-to-day: Collections,
Natural Language Search Models, Curation Sets, and Conversation Models.

Out of scope (possible future work): a generic "curl-for-every-endpoint"
explorer. This v1 targets the specific resources above.

## Scope — per resource

| Resource              | Operations                            |
|-----------------------|---------------------------------------|
| Collections / Indices | List + view schema (read-only)        |
| NL Search Models      | Full CRUD (list, create, update, del) |
| Curation Sets         | Full CRUD                             |
| Conversation Models   | Full CRUD + one-shot test             |

The Conversation Model "test" is a one-shot request: user enters a query,
sees the JSON response. No multi-turn chat.

## Connection & configuration

- Config file: `~/.config/clisense/config.yaml`, mode `0600`.
- Single connection (URL + API key). No profiles.
- First launch (file missing) opens a setup screen; subsequent launches load
  silently.
- A `Settings` tab lets the user edit values at any time.
- Corrupted YAML → fall back to the setup screen; do not crash.

## Architecture

```
clisense/
├── main.go                    # entrypoint: load config → launch TUI
├── internal/
│   ├── config/                # load/save ~/.config/clisense/config.yaml
│   ├── client/                # thin net/http Typesense client
│   │   ├── client.go          # Do(method, path, body) → (bytes, status, err)
│   │   └── endpoints.go       # typed path wrappers
│   ├── tui/
│   │   ├── app.go             # root Bubble Tea model, holds tabs + active tab
│   │   ├── tabs.go            # tab bar rendering + key routing
│   │   ├── components/
│   │   │   ├── jsoneditor.go  # multi-line JSON editor (textarea + validation)
│   │   │   ├── jsonview.go    # pretty-printed read-only JSON pane
│   │   │   └── list.go        # selectable list of resources
│   │   └── screens/
│   │       ├── setup.go       # first-run: URL + API key form
│   │       ├── collections.go # list + schema view
│   │       ├── nlmodels.go    # CRUD
│   │       ├── curations.go   # CRUD
│   │       └── conversations.go # CRUD + test
│   └── templates/             # embedded JSON skeletons for create/update
│       ├── nlmodel.json
│       ├── curation.json
│       └── conversation.json
└── go.mod
```

### Dependencies

- `github.com/charmbracelet/bubbletea`
- `github.com/charmbracelet/bubbles` (textinput, textarea, list, viewport, spinner)
- `github.com/charmbracelet/lipgloss`
- `gopkg.in/yaml.v3`
- stdlib `net/http`, `encoding/json`

No Typesense SDK — all API access goes through our thin HTTP client. This keeps
request/response transparent and avoids SDK-version lag for newer endpoints.

## UX

### Setup screen

Two `textinput` fields: `Typesense URL`, `API Key`. `Enter` saves to
`config.yaml` and continues into the app.

### Tab bar

Always visible at the top:

```
[1] Collections  [2] NL Models  [3] Curations  [4] Conversations  [5] Settings
```

- Number keys jump directly to a tab.
- `Tab` / `Shift+Tab` cycle.

### Common resource-tab layout (2-pane)

- **Left:** scrollable list of resource IDs/names, fetched on tab open.
- **Right:** pretty-printed JSON detail for the selected item.
- **Footer hint bar:** action keys available in the current context.

**Action keys:**

| Key     | Action                                                          |
|---------|-----------------------------------------------------------------|
| `n`     | New (opens JSON editor with a template)                         |
| `e`     | Edit (opens JSON editor pre-filled with current item)           |
| `d`     | Delete (confirmation prompt)                                    |
| `t`     | Test (Conversations tab only) — query input → JSON response     |
| `r`     | Refresh list                                                    |
| `Esc`   | Cancel / back out                                               |
| `?`     | Help overlay                                                    |

### Collections tab

Read-only. `n`/`e`/`d` disabled (hint bar reflects this). `Enter` on an item
loads the full schema JSON into the right pane.

### JSON editor overlay

- Full-screen `textarea` pre-filled from `internal/templates/`.
- `Ctrl+S` → validate JSON locally → send request.
- On success: close overlay, refresh list, select new/updated item.
- On failure: overlay stays open; footer strip shows the error.

### Conversation test modal

- Small `textarea` for the query + optional `collection` field.
- `Ctrl+S` sends; response lands in a read-only JSON view.
- One-shot — no follow-up turns.

## Data flow

1. Screen builds a request intent: `(METHOD, path, optional JSON body)`.
2. Screen dispatches a `tea.Cmd` that calls `client.Do(...)`. This runs off
   the UI goroutine; a spinner shows in the footer.
3. The command returns a `tea.Msg`: `apiSuccessMsg{status, body}` or
   `apiErrorMsg{status, body, err}`.
4. `Update` handles it — refresh list, close editor, or surface error.

## HTTP client

`internal/client/client.go`:

- Holds `baseURL`, `apiKey`, `http.Client` (30s timeout).
- One method: `Do(ctx, method, path, body []byte) ([]byte, int, error)`.
- Injects `X-TYPESENSE-API-KEY` and `Content-Type: application/json`.
- Returns raw body + status — no JSON parsing, no typed responses.
- Returns a non-nil `error` only for transport/timeout/context failures;
  HTTP error statuses return `(body, status, nil)` so the caller decides.

`endpoints.go` provides thin named wrappers (`ListCollections`,
`CreateNLModel`, etc.) that just compose paths.

## Error handling

| Category              | Example                    | Treatment                                               |
|-----------------------|----------------------------|---------------------------------------------------------|
| Local JSON parse      | user's edited body invalid | Footer strip: `invalid JSON: <msg>`, editor stays open  |
| HTTP error (4xx/5xx)  | 404, 422 from Typesense    | Footer strip: `HTTP <status>: <server body>`, stays open|
| Network / timeout     | no connection, DNS fail    | Footer strip: `network error: <msg>`, `r` retries       |

No silent failures. Successful response bytes are kept verbatim and
pretty-printed into the detail pane; copy-paste reproduces exactly what the
server sent.

## Testing

Narrow, meaningful scope — no model/rendering tests.

### HTTP client (`internal/client/`) — `httptest.Server`

- Sends correct method, path, and headers.
- Sends body bytes unmodified.
- Returns `(body, status, nil)` for both 2xx and non-2xx.
- Returns non-nil `error` for transport failures.
- Respects context cancellation.

### Config (`internal/config/`) — temp dir

- Round-trip: write → read returns same values.
- Missing file → distinct sentinel error.
- Corrupted YAML → distinct sentinel error.
- File created with mode `0600`.

### Templates (`internal/templates/`)

- Every embedded template parses as valid JSON.

### Explicitly not tested

Bubble Tea models, key routing, rendering. Manual smoke-test against a real
Typesense instance before shipping changes.

### Manual smoke-test checklist

1. First-run → setup screen → save → Collections tab loads.
2. Every tab loads its list without error.
3. Create → edit → delete round-trip on NL Models, Curations, Conversations.
4. Conversation `t` returns a response.
5. Bad API key surfaces an error; app does not crash.

## Non-goals (v1)

- Multiple named connection profiles.
- Generic "any endpoint" curl-style explorer.
- Multi-turn conversation chat UI.
- SDK-based client, typed response models.
- Bubble Tea model/rendering unit tests.
