// Package config reads and writes the clisense on-disk configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

var (
	ErrNotFound    = errors.New("config not found")
	ErrCorrupt     = errors.New("config corrupt")
	ErrInvalidTabs = errors.New("invalid tabs config")
)

type Config struct {
	URL    string     `toml:"url,omitempty" yaml:"url"`
	APIKey string     `toml:"api_key,omitempty" yaml:"api_key"`
	Tabs   TabsConfig `toml:"tabs" yaml:"tabs"`
}

type TabsConfig struct {
	Order []string `toml:"order" yaml:"order"`
}

var DefaultTabOrder = []string{
	"collections",
	"aliases",
	"nl_models",
	"curations",
	"synonyms",
	"stopwords",
	"presets",
	"conversations",
	"api_keys",
	"analytics",
	"cluster",
	"settings",
}

var validTabIDs = map[string]struct{}{
	"collections":   {},
	"aliases":       {},
	"nl_models":     {},
	"curations":     {},
	"synonyms":      {},
	"stopwords":     {},
	"presets":       {},
	"conversations": {},
	"api_keys":      {},
	"analytics":     {},
	"cluster":       {},
	"settings":      {},
}

// DefaultPath returns the standard config path, honoring $XDG_CONFIG_HOME.
func DefaultPath() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "clisense", "config.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "clisense", "config.toml"), nil
}

func Load(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if filepath.Ext(path) == ".toml" {
				return loadLegacyYAML(path)
			}
			return Config{}, ErrNotFound
		}
		return Config{}, err
	}
	var c Config
	if filepath.Ext(path) == ".yaml" || filepath.Ext(path) == ".yml" {
		if err := yaml.Unmarshal(b, &c); err != nil {
			return Config{}, fmt.Errorf("%w: %v", ErrCorrupt, err)
		}
	} else if err := toml.Unmarshal(b, &c); err != nil {
		return Config{}, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	if err := c.validateTabs(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func Save(path string, c Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	b, err := toml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0600)
}

func (c Config) NeedsSetup() bool {
	return c.URL == "" || c.APIKey == ""
}

func (c Config) TabOrder() []string {
	if len(c.Tabs.Order) == 0 {
		return append([]string(nil), DefaultTabOrder...)
	}
	order := append([]string(nil), c.Tabs.Order...)
	for _, id := range order {
		if id == "settings" {
			return order
		}
	}
	return append(order, "settings")
}

func (c Config) validateTabs() error {
	seen := map[string]struct{}{}
	for _, id := range c.Tabs.Order {
		if _, ok := validTabIDs[id]; !ok {
			return fmt.Errorf("%w: unknown tab id %q", ErrInvalidTabs, id)
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("%w: duplicate tab id %q", ErrInvalidTabs, id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

func loadLegacyYAML(tomlPath string) (Config, error) {
	yamlPath := filepath.Join(filepath.Dir(tomlPath), "config.yaml")
	cfg, err := Load(yamlPath)
	if errors.Is(err, ErrNotFound) {
		return Config{}, ErrNotFound
	}
	return cfg, err
}
