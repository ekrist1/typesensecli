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

	in := WithConnection("http://localhost:8108", "xyz")
	in.Tabs = TabsConfig{Order: []string{"collections", "settings"}}
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
	if cfg.URL() != "http://localhost:8108" || cfg.APIKey() != "xyz" {
		t.Errorf("got url=%q apiKey=%q, want legacy YAML migrated", cfg.URL(), cfg.APIKey())
	}
	if cfg.ActiveClusterName() != "default" {
		t.Errorf("active cluster = %q, want %q", cfg.ActiveClusterName(), "default")
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
	if err := Save(path, WithConnection("u", "k")); err != nil {
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
		{"url only", Config{Global: GlobalConfig{URL: "http://localhost:8108"}}, true},
		{"api key only", Config{Global: GlobalConfig{APIKey: "xyz"}}, true},
		{"flat both set", Config{Global: GlobalConfig{URL: "http://localhost:8108", APIKey: "xyz"}}, false},
		{"cluster set", WithConnection("http://localhost:8108", "xyz"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.NeedsSetup(); got != tt.want {
				t.Errorf("NeedsSetup() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMultipleClustersUseDefault(t *testing.T) {
	cfg := Config{
		Global: GlobalConfig{DefaultCluster: "prod"},
		Clusters: map[string]ClusterConfig{
			"local": {URL: "http://localhost:8108", APIKey: "local-key"},
			"prod":  {URL: "https://typesense.example.com", APIKey: "prod-key"},
		},
	}
	if cfg.ActiveClusterName() != "prod" {
		t.Errorf("active cluster = %q, want prod", cfg.ActiveClusterName())
	}
	if cfg.URL() != "https://typesense.example.com" {
		t.Errorf("URL() = %q, want prod url", cfg.URL())
	}
	if cfg.APIKey() != "prod-key" {
		t.Errorf("APIKey() = %q, want prod key", cfg.APIKey())
	}
	if cfg.NeedsSetup() {
		t.Errorf("NeedsSetup() = true, want false with valid default cluster")
	}
}

func TestMissingDefaultClusterPromptsSetup(t *testing.T) {
	cfg := Config{
		Clusters: map[string]ClusterConfig{
			"local": {URL: "http://localhost:8108", APIKey: "local-key"},
			"prod":  {URL: "https://typesense.example.com", APIKey: "prod-key"},
		},
	}
	if !cfg.NeedsSetup() {
		t.Errorf("NeedsSetup() = false, want true when default_cluster is unset and there are multiple clusters")
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

func TestSaveSplitsAPIKeysIntoSecrets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	in := WithConnection("http://localhost:8108", "secret-key")

	if err := Save(path, in); err != nil {
		t.Fatalf("Save: %v", err)
	}

	main, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(main), "secret-key") {
		t.Errorf("config.toml should not contain the API key:\n%s", main)
	}

	secrets, err := os.ReadFile(SecretsPath(path))
	if err != nil {
		t.Fatalf("secrets.toml was not written: %v", err)
	}
	if !strings.Contains(string(secrets), "secret-key") {
		t.Errorf("secrets.toml does not contain the API key:\n%s", secrets)
	}
}

func TestLoadMergesSecretsSidecar(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`[global]
default_cluster = "local"

[clusters.local]
url = "http://localhost:8108"
`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(SecretsPath(path), []byte(`[clusters.local]
api_key = "secret-key"
`), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.APIKey() != "secret-key" {
		t.Errorf("APIKey() = %q, want secret-key", cfg.APIKey())
	}
	if cfg.URL() != "http://localhost:8108" {
		t.Errorf("URL() = %q, want http://localhost:8108", cfg.URL())
	}
}

func TestLoadMigratesInlineAPIKeysToSecrets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	inline := []byte(`[global]
default_cluster = "local"

[clusters.local]
url = "http://localhost:8108"
api_key = "secret-key"
`)
	if err := os.WriteFile(path, inline, 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.APIKey() != "secret-key" {
		t.Errorf("APIKey() = %q, want secret-key (in-memory)", cfg.APIKey())
	}

	main, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(main), "secret-key") {
		t.Errorf("config.toml still contains the inline key after migration:\n%s", main)
	}
	secrets, err := os.ReadFile(SecretsPath(path))
	if err != nil {
		t.Fatalf("secrets.toml was not created: %v", err)
	}
	if !strings.Contains(string(secrets), "secret-key") {
		t.Errorf("secrets.toml does not contain the migrated key:\n%s", secrets)
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
