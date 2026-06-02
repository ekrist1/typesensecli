package tui

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"clisense/internal/config"
	"clisense/internal/tui/screens"
)

func TestNewAppUsesConfiguredTabOrder(t *testing.T) {
	cfg := config.Config{
		URL:    "http://localhost:8108",
		APIKey: "xyz",
		Tabs:   config.TabsConfig{Order: []string{"analytics", "collections", "settings"}},
	}
	app := NewApp(cfg, "/tmp/config.toml", false)

	want := []string{"analytics", "collections", "settings"}
	for i, id := range want {
		if app.tabIDs[i] != id {
			t.Fatalf("tabIDs = %+v, want %+v", app.tabIDs, want)
		}
	}
}

func TestSettingsEnterSavesConfig(t *testing.T) {
	cfg := config.Config{URL: "http://localhost:8108", APIKey: "xyz"}
	path := filepath.Join(t.TempDir(), "config.yaml")
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
	if app.active != 0 {
		t.Fatalf("active tab = %d, want 0", app.active)
	}
}

func TestCtrlCQuitsInitialSetup(t *testing.T) {
	app := NewApp(config.Config{}, "/tmp/config.yaml", true)

	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c in setup returned no command")
	}
	if msg := cmd(); msg != tea.Quit() {
		t.Fatalf("ctrl+c command returned %v, want tea.Quit", msg)
	}
}
