package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	in := Config{
		URL:    "http://localhost:8108",
		APIKey: "xyz",
		Tabs:   TabsConfig{Order: []string{"collections", "settings"}},
	}
	if err := Save(path, in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	out, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(out, in) {
		t.Errorf("got %+v, want %+v", out, in)
	}
}

func TestDefaultPathIsTOML(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	got, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Ext(got) != ".toml" {
		t.Errorf("DefaultPath() = %q, want .toml extension", got)
	}
}

func TestLoadMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.toml")
	_, err := Load(path)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}

func TestLoadLegacyYAMLFallback(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(yamlPath, []byte("url: http://localhost:8108\napi_key: xyz\n"), 0600); err != nil {
		t.Fatal(err)
	}
	tomlPath := filepath.Join(dir, "config.toml")
	cfg, err := Load(tomlPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.URL != "http://localhost:8108" || cfg.APIKey != "xyz" {
		t.Errorf("got %+v, want legacy YAML migrated", cfg)
	}
}

func TestLoadCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corrupt.toml")
	if err := os.WriteFile(path, []byte("not = valid = toml"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if !errors.Is(err, ErrCorrupt) {
		t.Errorf("got %v, want ErrCorrupt", err)
	}
}

func TestSavePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
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

func TestNeedsSetup(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
		want bool
	}{
		{"empty", Config{}, true},
		{"url only", Config{URL: "http://localhost:8108"}, true},
		{"api key only", Config{APIKey: "xyz"}, true},
		{"both set", Config{URL: "http://localhost:8108", APIKey: "xyz"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.NeedsSetup(); got != tt.want {
				t.Errorf("NeedsSetup() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTabOrderDefault(t *testing.T) {
	cfg := Config{}
	got := cfg.TabOrder()
	if !reflect.DeepEqual(got, DefaultTabOrder) {
		t.Errorf("TabOrder() = %v, want %v", got, DefaultTabOrder)
	}
}

func TestTabOrderConfigured(t *testing.T) {
	cfg := Config{Tabs: TabsConfig{Order: []string{"analytics", "collections", "settings"}}}
	got := cfg.TabOrder()
	want := []string{"analytics", "collections", "settings"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("TabOrder() = %v, want %v", got, want)
	}
}

func TestTabOrderAppendsSettings(t *testing.T) {
	cfg := Config{Tabs: TabsConfig{Order: []string{"analytics", "collections"}}}
	got := cfg.TabOrder()
	want := []string{"analytics", "collections", "settings"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("TabOrder() = %v, want %v", got, want)
	}
}

func TestLoadInvalidTabUnknown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[tabs]\norder = [\"nope\"]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if !errors.Is(err, ErrInvalidTabs) {
		t.Fatalf("got %v, want ErrInvalidTabs", err)
	}
	if errors.Is(err, ErrCorrupt) {
		t.Errorf("ErrInvalidTabs should not wrap ErrCorrupt")
	}
	msg := err.Error()
	if !strings.Contains(msg, `"nope"`) {
		t.Errorf("error %q should name the offending id", msg)
	}
	if !strings.Contains(msg, "available values:") {
		t.Errorf("error %q should list available values", msg)
	}
	for _, id := range DefaultTabOrder {
		if !strings.Contains(msg, id) {
			t.Errorf("error %q should mention %q", msg, id)
		}
	}
}

func TestLoadInvalidTabDuplicate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("[tabs]\norder = [\"collections\", \"collections\"]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if !errors.Is(err, ErrInvalidTabs) {
		t.Errorf("got %v, want ErrInvalidTabs", err)
	}
}
