package app

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"

	"github.com/eve-online-tools/yulai/identity/sso"
)

const Name = "yulai"

// Runtime overrides, applied over the build-time values and the SSO file. The
// Taskfile reads the same names from the environment or .env at build time.
const (
	ClientIDEnv    = "YULAI_SSO_CLIENT_ID"
	CallbackURLEnv = "YULAI_SSO_CALLBACK_URL"
	HostEnv        = "YULAI_SSO_HOST"
)

// DefaultCallbackURL must match the SSO app registration.
const DefaultCallbackURL = "http://localhost:45538/callback"

// Set with -ldflags -X by the Taskfile.
var (
	buildClientID    string
	buildCallbackURL string
	buildSSOHost     string
)

type Config struct {
	SSO     sso.Config
	DataDir string
}

// LoadConfig resolves the data dir and the SSO registration. A missing client ID
// is an error: without it no character can log in.
func LoadConfig() (*Config, error) {
	dataDir, err := xdg.DataFile(Name)
	if err != nil {
		return nil, fmt.Errorf("resolve data dir: %w", err)
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	source := filepath.Join(xdg.ConfigHome, Name, "sso.json")
	ssoCfg, err := resolveSSO(source)
	if err != nil {
		return nil, err
	}
	return &Config{SSO: ssoCfg, DataDir: dataDir}, nil
}

// resolveSSO layers defaults, build-time values, the SSO file at path and
// environment variables, later sources winning. The client ID is required.
func resolveSSO(path string) (sso.Config, error) {
	c := sso.Config{
		Name:        Name,
		Description: "Yulai",
		ClientID:    buildClientID,
		CallbackURL: buildCallbackURL,
		Issuer:      buildSSOHost,
	}
	if err := readSSO(path, &c); err != nil {
		return c, err
	}
	c.ClientID = cmp.Or(os.Getenv(ClientIDEnv), c.ClientID)
	c.CallbackURL = cmp.Or(os.Getenv(CallbackURLEnv), c.CallbackURL, DefaultCallbackURL)
	c.Issuer = sso.NormalizeIssuer(cmp.Or(os.Getenv(HostEnv), c.Issuer))
	if c.ClientID == "" {
		return c, fmt.Errorf("no SSO client ID: build with %s set, or set clientId in %s", ClientIDEnv, path)
	}
	return c, nil
}

// readSSO overlays the fields present in the SSO file onto c.
func readSSO(path string, c *sso.Config) error {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, c); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}

func (c *Config) DBPath() string    { return filepath.Join(c.DataDir, "yulai.sqlite") }
func (c *Config) CachePath() string { return filepath.Join(c.DataDir, "esi-cache.sqlite") }
