# clisense

A fast, keyboard-driven terminal UI for administering a [Typesense](https://typesense.org) instance — built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

`clisense` gives you a tabbed, full-screen TUI to browse and manage the building blocks of a Typesense deployment without juggling `curl` commands: collections, aliases, natural-language search models, curations, synonyms, stopwords, presets, and conversation models. Each resource has an embedded JSON editor seeded from sensible templates, plus guided wizards for the trickier ones.

> Targets the Typesense API as documented for [v30.2](https://typesense.org/docs/30.2/api/).

---

## Features

- **Tabbed admin console** — switch between resource types with number keys or `Tab`/`Shift+Tab`.
- **Collections** — list, inspect, update (PATCH), delete, and run ad-hoc document searches.
- **Aliases** — list, view, upsert, and delete collection aliases.
- **NL Search Models** — manage natural-language search models.
- **Curations** — manage curation sets, with a step-by-step **curation wizard** and a document picker (`Ctrl+F`) to pin/hide results.
- **Synonyms** — manage synonym sets with a dedicated **synonym wizard**.
- **Stopwords** — manage stopword sets.
- **Presets** — manage reusable search presets.
- **Conversations** — manage conversation models and test conversational search.
- **Embedded JSON editor** — every create/update flow opens a JSON editor pre-filled from a template.
- **First-run setup** — a guided setup screen captures your Typesense URL and API key, persisted to a config file.

## Requirements

- **Go 1.25+** (only needed to build from source)
- A reachable **Typesense server** and an **API key** with admin privileges

## Installation

### Download a release binary

Pre-built Linux binaries (`amd64` and `arm64`) are published on the [Releases page](https://github.com/ekrist1/typesensecli/releases). Download the archive for your platform, extract it, and put the binary on your `PATH`:

```sh
tar -xzf clisense_*_linux_amd64.tar.gz
sudo mv clisense /usr/local/bin/
```

### Build from source

```sh
git clone https://github.com/ekrist1/typesensecli.git
cd typesensecli
go build -o clisense .
```

### Install with `go install`

```sh
go install github.com/ekrist1/typesensecli@latest
```

The resulting binary is named `clisense`.

## Usage

Launch the TUI:

```sh
clisense
```

On first run (or if no config is found) you'll be dropped into the **setup screen**. Enter your Typesense URL (e.g. `http://localhost:8108`) and API key; they're saved to your config file and reused on subsequent launches. You can change them later from the **Settings** tab.

### Keybindings

| Key | Action |
| --- | --- |
| `1`–`9` | Jump directly to a tab |
| `Tab` / `Shift+Tab` | Cycle to the next / previous tab |
| `q` | Quit |
| `Ctrl+C` | Quit (always) |
| `Esc` | Leave Settings and return to Collections |
| `Ctrl+F` | Open the document picker (in the curation wizard) |

> When a modal, wizard, or text input is open, `q`, the number keys, and `Tab` are passed through to the input instead of triggering global navigation.

## Configuration

Configuration lives in a YAML file:

- `$XDG_CONFIG_HOME/clisense/config.yaml` if `XDG_CONFIG_HOME` is set, otherwise
- `~/.config/clisense/config.yaml`

It is created automatically by the setup screen, but you can also write it by hand:

```yaml
url: http://localhost:8108
api_key: your-admin-api-key
```

The file is written with `0600` permissions since it holds your API key.

## Project layout

```
.
├── main.go                  # entrypoint: load config, start the TUI
├── internal/
│   ├── config/              # read/write the on-disk YAML config
│   ├── client/              # thin HTTP wrapper + Typesense endpoint helpers
│   ├── templates/           # embedded JSON skeletons for the editor
│   └── tui/
│       ├── app.go           # root model, tab routing, global keybindings
│       ├── components/      # reusable widgets (JSON editor, tab bar, doc picker, …)
│       └── screens/         # one screen per resource type + setup/settings
└── .goreleaser.yaml         # release build configuration
```

The `client` package is intentionally thin: each endpoint helper returns the `(method, path)` pair for a Typesense API route, and `Client.Do` performs the request. HTTP error statuses (4xx/5xx) are returned as data so the UI can render the server's error message.

## Development

```sh
go test ./...      # run the test suite
go build -o clisense .
go vet ./...
```

## Releasing

Releases are produced by [GoReleaser](https://goreleaser.com) and automated via GitHub Actions. Pushing a semver tag builds the Linux binaries and publishes a GitHub Release:

```sh
git tag v0.1.0
git push origin v0.1.0
```

See [`.github/workflows/release.yml`](.github/workflows/release.yml) and [`.goreleaser.yaml`](.goreleaser.yaml).

## License

_No license has been declared yet._ Add a `LICENSE` file to clarify how others may use this project.
