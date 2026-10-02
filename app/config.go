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

// IssuerEnv overrides the SSO issuer from the SSO file, e.g. to use a test server.
const IssuerEnv = "YULAI_SSO_ISSUER"

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

	if err := readSSO(path, &cfg.SSO); err != nil {
		return nil, err
	}
	if issuer := os.Getenv(IssuerEnv); issuer != "" {
		cfg.SSO.Issuer = issuer
	}
	cfg.SSO.Issuer = sso.NormalizeIssuer(cfg.SSO.Issuer)
	return cfg, nil
}

func readSSO(path string, c *sso.Config) error {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		slog.Warn("no SSO config found; login is disabled", "want", []string{DevFile, path})
		return nil
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, c); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	if c.ClientID == "" || c.CallbackURL == "" {
		return fmt.Errorf("%s: clientId and callbackUrl are required", path)
	}
	return nil
}

func (c *Config) DBPath() string    { return filepath.Join(c.DataDir, "yulai.sqlite") }
func (c *Config) CachePath() string { return filepath.Join(c.DataDir, "esi-cache.sqlite") }
