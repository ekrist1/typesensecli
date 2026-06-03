package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/config"
	"clisense/internal/tui/screens"
)

func TestNewAppUsesConfiguredTabOrder(t *testing.T) {
	cfg := config.WithConnection("http://localhost:8108", "xyz")
	cfg.Tabs = config.TabsConfig{Order: []string{"analytics", "collections", "settings"}}
	app := NewApp(cfg, filepath.Join(t.TempDir(), "config.toml"), false)

	want := []string{"analytics", "collections", "settings"}
	if len(app.tabIDs) != len(want) {
		t.Fatalf("tabIDs = %+v, want %+v", app.tabIDs, want)
	}
	for i, id := range want {
		if app.tabIDs[i] != id {
			t.Fatalf("tabIDs = %+v, want %+v", app.tabIDs, want)
		}
	}
}

func TestSettingsEnterSavesConfig(t *testing.T) {
	cfg := config.WithConnection("http://localhost:8108", "xyz")
	cfg.Tabs = config.TabsConfig{Order: []string{"analytics", "settings", "collections"}}
	path := filepath.Join(t.TempDir(), "config.toml")
	app := NewApp(cfg, path, false)

	app.activateTab(app.settingsTabIndex())
	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter in settings returned no command")
	}
	msg := cmd()
	if _, ok := msg.(screens.SetupDoneMsg); !ok {
		t.Fatalf("Enter in settings returned %T, want SetupDoneMsg", msg)
	}

	model, cmd = model.Update(msg)
	if cmd != nil {
		cmd()
	}
	app = model.(App)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file was not saved: %v", err)
	}
	if app.inSetup {
		t.Fatal("app remained in setup after saving settings")
	}
	if app.activeTabID() != "collections" {
		t.Fatalf("active tab = %q, want collections", app.activeTabID())
	}
}

func TestSettingsEscActivatesCollectionsByID(t *testing.T) {
	cfg := config.WithConnection("http://localhost:8108", "xyz")
	cfg.Tabs = config.TabsConfig{Order: []string{"analytics", "settings", "collections"}}
	app := NewApp(cfg, filepath.Join(t.TempDir(), "config.toml"), false)

	app.activateTab(app.settingsTabIndex())
	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		cmd()
	}
	app = model.(App)
	if app.activeTabID() != "collections" {
		t.Fatalf("active tab = %q, want collections", app.activeTabID())
	}
}

func TestSettingsCanSelectDefaultCluster(t *testing.T) {
	cfg := config.Config{
		Global: config.GlobalConfig{DefaultCluster: "local"},
		Clusters: map[string]config.ClusterConfig{
			"local": {URL: "http://localhost:8108", APIKey: "local-key"},
			"prod":  {URL: "https://typesense.example.com", APIKey: "prod-key"},
		},
		Tabs: config.TabsConfig{Order: []string{"collections", "settings"}},
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	app := NewApp(cfg, path, false)

	app.activateTab(app.settingsTabIndex())
	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRight})
	if cmd != nil {
		cmd()
	}
	app = model.(App)
	model, cmd = app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter in settings returned no command")
	}
	msg := cmd()
	if _, ok := msg.(screens.SetupDoneMsg); !ok {
		t.Fatalf("Enter in settings returned %T, want SetupDoneMsg", msg)
	}

	model, cmd = model.Update(msg)
	if cmd != nil {
		cmd()
	}
	app = model.(App)
	if app.cfg.ActiveClusterName() != "prod" {
		t.Fatalf("active cluster = %q, want prod", app.cfg.ActiveClusterName())
	}
	if app.cfg.URL() != "https://typesense.example.com" || app.cfg.APIKey() != "prod-key" {
		t.Fatalf("got url=%q apiKey=%q", app.cfg.URL(), app.cfg.APIKey())
	}
}

func TestGlobalClusterPickerSwitchesCluster(t *testing.T) {
	cfg := config.Config{
		Global: config.GlobalConfig{DefaultCluster: "local"},
		Clusters: map[string]config.ClusterConfig{
			"local": {URL: "http://localhost:8108", APIKey: "local-key"},
			"prod":  {URL: "https://typesense.example.com", APIKey: "prod-key"},
		},
		Tabs: config.TabsConfig{Order: []string{"collections", "settings"}},
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	app := NewApp(cfg, path, false)

	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if cmd != nil {
		cmd()
	}
	app = model.(App)
	if !app.picking {
		t.Fatal("global cluster picker did not open")
	}
	model, cmd = app.Update(tea.KeyMsg{Type: tea.KeyDown})
	if cmd != nil {
		cmd()
	}
	app = model.(App)
	model, cmd = app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		cmd()
	}
	app = model.(App)

	if app.picking {
		t.Fatal("global cluster picker stayed open after selection")
	}
	if app.cfg.ActiveClusterName() != "prod" {
		t.Fatalf("active cluster = %q, want prod", app.cfg.ActiveClusterName())
	}
	if app.cfg.URL() != "https://typesense.example.com" || app.cfg.APIKey() != "prod-key" {
		t.Fatalf("got url=%q apiKey=%q", app.cfg.URL(), app.cfg.APIKey())
	}
	saved, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load saved config: %v", err)
	}
	if saved.ActiveClusterName() != "prod" {
		t.Fatalf("saved active cluster = %q, want prod", saved.ActiveClusterName())
	}
}

func TestGlobalClusterPickerKeepsClusterOnSaveFailure(t *testing.T) {
	cfg := config.Config{
		Global: config.GlobalConfig{DefaultCluster: "local"},
		Clusters: map[string]config.ClusterConfig{
			"local": {URL: "http://localhost:8108", APIKey: "local-key"},
			"prod":  {URL: "https://typesense.example.com", APIKey: "prod-key"},
		},
		Tabs: config.TabsConfig{Order: []string{"collections", "settings"}},
	}
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	app := NewApp(cfg, filepath.Join(blocker, "config.toml"), false)

	if !app.openClusterPicker() {
		t.Fatal("global cluster picker did not open")
	}
	app.moveClusterPick(1)
	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		cmd()
	}
	app = model.(App)

	if !app.picking {
		t.Fatal("global cluster picker closed after failed save")
	}
	if app.pickErr == "" {
		t.Fatal("save failure did not set picker error")
	}
	if app.cfg.ActiveClusterName() != "local" {
		t.Fatalf("active cluster = %q, want local", app.cfg.ActiveClusterName())
	}
	if app.cfg.URL() != "http://localhost:8108" || app.cfg.APIKey() != "local-key" {
		t.Fatalf("got url=%q apiKey=%q, want local cluster", app.cfg.URL(), app.cfg.APIKey())
	}
}

func TestGlobalClusterPickerOverlaysCurrentPage(t *testing.T) {
	cfg := config.Config{
		Global: config.GlobalConfig{DefaultCluster: "local"},
		Clusters: map[string]config.ClusterConfig{
			"local": {URL: "http://localhost:8108", APIKey: "local-key"},
			"prod":  {URL: "https://typesense.example.com", APIKey: "prod-key"},
		},
		Tabs: config.TabsConfig{Order: []string{"collections", "settings"}},
	}
	app := NewApp(cfg, filepath.Join(t.TempDir(), "config.toml"), false)
	model, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if cmd != nil {
		cmd()
	}
	app = model.(App)

	view := app.View()
	if !strings.Contains(view, "Clusters") {
		t.Fatal("view does not contain cluster picker")
	}
	if !strings.Contains(view, "Collections") {
		t.Fatal("view does not preserve underlying page content")
	}
}

func TestCtrlCQuitsInitialSetup(t *testing.T) {
	app := NewApp(config.Config{}, filepath.Join(t.TempDir(), "config.toml"), true)

	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c in setup returned no command")
	}
	if msg := cmd(); msg != tea.Quit() {
		t.Fatalf("ctrl+c command returned %v, want tea.Quit", msg)
	}
}
