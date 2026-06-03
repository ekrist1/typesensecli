// Package config reads and writes the clisense on-disk configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

var (
	ErrNotFound    = errors.New("config not found")
	ErrCorrupt     = errors.New("config corrupt")
	ErrInvalidTabs = errors.New("invalid tabs config")
)

type Config struct {
	Global   GlobalConfig             `toml:"global" yaml:"global"`
	Clusters map[string]ClusterConfig `toml:"clusters" yaml:"clusters"`
	Tabs     TabsConfig               `toml:"tabs" yaml:"tabs"`
}

type GlobalConfig struct {
	DefaultCluster string `toml:"default_cluster" yaml:"default_cluster"`
	URL            string `toml:"url,omitempty" yaml:"url"`
	APIKey         string `toml:"api_key,omitempty" yaml:"api_key"`
}

type ClusterConfig struct {
	URL    string `toml:"url" yaml:"url"`
	APIKey string `toml:"api_key,omitempty" yaml:"api_key,omitempty"`
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

// SecretsPath returns the sidecar `secrets.toml` path next to a config file.
// When this file exists, Load merges its API keys into the loaded config and
// Save splits API keys into it so the main `config.toml` can be committed.
func SecretsPath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "secrets.toml")
}

type secretsFile struct {
	Global   secretsGlobal             `toml:"global,omitempty"`
	Clusters map[string]secretsCluster `toml:"clusters,omitempty"`
}

type secretsGlobal struct {
	APIKey string `toml:"api_key,omitempty"`
}

type secretsCluster struct {
	APIKey string `toml:"api_key"`
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
		migrateFlatYAML(b, &c)
	} else if err := toml.Unmarshal(b, &c); err != nil {
		return Config{}, fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	if err := c.validateTabs(); err != nil {
		return Config{}, err
	}
	c.migrateLegacyConnection()
	if filepath.Ext(path) != ".toml" {
		return c, nil
	}
	secretsPath := SecretsPath(path)
	secrets, secretsExists, err := loadSecrets(secretsPath)
	if err != nil {
		return Config{}, err
	}
	if secretsExists {
		c.mergeSecrets(secrets)
		return c, nil
	}
	// Migrate inline API keys into a sidecar secrets.toml so the main
	// config can be committed to dotfiles.
	if hasInlineSecrets(c) {
		if err := Save(path, c); err != nil {
			return Config{}, err
		}
	}
	return c, nil
}

func Save(path string, c Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	public, secrets := splitSecrets(c)
	if err := writeTOML(SecretsPath(path), secrets); err != nil {
		return err
	}
	return writeTOML(path, public)
}

func writeTOML(path string, v any) error {
	b, err := toml.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0600)
}

func loadSecrets(path string) (secretsFile, bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return secretsFile{}, false, nil
		}
		return secretsFile{}, false, err
	}
	var s secretsFile
	if err := toml.Unmarshal(b, &s); err != nil {
		return secretsFile{}, false, fmt.Errorf("%w: secrets: %v", ErrCorrupt, err)
	}
	return s, true, nil
}

func (c *Config) mergeSecrets(s secretsFile) {
	if c.Clusters == nil && len(s.Clusters) > 0 {
		c.Clusters = map[string]ClusterConfig{}
	}
	for name, secret := range s.Clusters {
		if secret.APIKey == "" {
			continue
		}
		cluster, ok := c.Clusters[name]
		if !ok {
			continue
		}
		cluster.APIKey = secret.APIKey
		c.Clusters[name] = cluster
	}
	if c.Global.APIKey == "" && s.Global.APIKey != "" {
		c.Global.APIKey = s.Global.APIKey
	}
}

func hasInlineSecrets(c Config) bool {
	if c.Global.APIKey != "" {
		return true
	}
	for _, cluster := range c.Clusters {
		if cluster.APIKey != "" {
			return true
		}
	}
	return false
}

func splitSecrets(c Config) (Config, secretsFile) {
	public := c
	secrets := secretsFile{Clusters: map[string]secretsCluster{}}
	if len(c.Clusters) > 0 {
		publicClusters := make(map[string]ClusterConfig, len(c.Clusters))
		for name, cluster := range c.Clusters {
			if cluster.APIKey != "" {
				secrets.Clusters[name] = secretsCluster{APIKey: cluster.APIKey}
			}
			publicClusters[name] = ClusterConfig{URL: cluster.URL}
		}
		public.Clusters = publicClusters
	}
	if public.Global.APIKey != "" {
		secrets.Global.APIKey = public.Global.APIKey
		public.Global.APIKey = ""
	}
	return public, secrets
}

func (c Config) URL() string {
	cluster, ok := c.ActiveCluster()
	if !ok {
		return ""
	}
	return cluster.URL
}

func (c Config) APIKey() string {
	cluster, ok := c.ActiveCluster()
	if !ok {
		return ""
	}
	return cluster.APIKey
}

func (c Config) ActiveClusterName() string {
	if c.Global.DefaultCluster != "" {
		return c.Global.DefaultCluster
	}
	if len(c.Clusters) == 1 {
		for name := range c.Clusters {
			return name
		}
	}
	if c.Global.URL != "" || c.Global.APIKey != "" {
		return "default"
	}
	return ""
}

func (c Config) ActiveCluster() (ClusterConfig, bool) {
	name := c.ActiveClusterName()
	if name == "" {
		return ClusterConfig{}, false
	}
	if cluster, ok := c.Clusters[name]; ok {
		return cluster, true
	}
	if name == "default" && (c.Global.URL != "" || c.Global.APIKey != "") {
		return ClusterConfig{URL: c.Global.URL, APIKey: c.Global.APIKey}, true
	}
	return ClusterConfig{}, false
}

func (c Config) ClusterNames() []string {
	names := make([]string, 0, len(c.Clusters))
	for name := range c.Clusters {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (c Config) NeedsSetup() bool {
	cluster, ok := c.ActiveCluster()
	return !ok || cluster.URL == "" || cluster.APIKey == ""
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

// WithConnection returns a Config seeded with a single "default" cluster.
// Convenience for tests and the first-run setup flow.
func WithConnection(url, apiKey string) Config {
	c := Config{Tabs: TabsConfig{Order: append([]string(nil), DefaultTabOrder...)}}
	c.SetConnection("default", url, apiKey)
	return c
}

func (c *Config) SetConnection(name, url, apiKey string) {
	if name == "" {
		name = "default"
	}
	if c.Clusters == nil {
		c.Clusters = map[string]ClusterConfig{}
	}
	c.Clusters[name] = ClusterConfig{URL: url, APIKey: apiKey}
	c.Global.DefaultCluster = name
	c.Global.URL = ""
	c.Global.APIKey = ""
}

func migrateFlatYAML(b []byte, c *Config) {
	if c.Global.URL != "" || c.Global.APIKey != "" {
		return
	}
	type flatConfig struct {
		URL    string `yaml:"url"`
		APIKey string `yaml:"api_key"`
	}
	var flat flatConfig
	if err := yaml.Unmarshal(b, &flat); err == nil {
		c.Global.URL = flat.URL
		c.Global.APIKey = flat.APIKey
	}
}

func (c *Config) migrateLegacyConnection() {
	if len(c.Clusters) != 0 || (c.Global.URL == "" && c.Global.APIKey == "") {
		return
	}
	c.SetConnection("default", c.Global.URL, c.Global.APIKey)
}

func (c Config) validateTabs() error {
	seen := map[string]struct{}{}
	for _, id := range c.Tabs.Order {
		if _, ok := validTabIDs[id]; !ok {
			return fmt.Errorf("%w: unknown tab id %q in tabs.order; available values: %s",
				ErrInvalidTabs, id, strings.Join(DefaultTabOrder, ", "))
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("%w: duplicate tab id %q in tabs.order", ErrInvalidTabs, id)
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
