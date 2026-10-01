package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"

	"github.com/eve-online-tools/yulai/identity/sso"
)

const Name = "yulai"

// DevFile is loaded from the working directory when present. Gitignored.
const DevFile = "sso.dev.json"

type Config struct {
	SSO     sso.Config
	DataDir string
	Source  string
}

// LoadConfig resolves the data dir and reads the SSO registration. A missing SSO
// file is not fatal while the skeleton has no login; it will be once it does.
func LoadConfig() (*Config, error) {
	dataDir, err := xdg.DataFile(Name)
	if err != nil {
		return nil, fmt.Errorf("resolve data dir: %w", err)
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	cfg := &Config{
		DataDir: dataDir,
		SSO:     sso.Config{Name: Name, Description: "Yulai"},
	}

	path := DevFile
	if _, err := os.Stat(path); err != nil {
		path = filepath.Join(xdg.ConfigHome, Name, "sso.json")
	}
	cfg.Source = path

	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		slog.Warn("no SSO config found; login is disabled", "want", []string{DevFile, path})
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &cfg.SSO); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if cfg.SSO.ClientID == "" || cfg.SSO.CallbackURL == "" {
		return nil, fmt.Errorf("%s: clientId and callbackUrl are required", path)
	}
	return cfg, nil
}

func (c *Config) DBPath() string    { return filepath.Join(c.DataDir, "yulai.sqlite") }
func (c *Config) CachePath() string { return filepath.Join(c.DataDir, "esi-cache.sqlite") }
