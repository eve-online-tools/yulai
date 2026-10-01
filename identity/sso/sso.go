// Package sso speaks the EVE Online SSO protocol: discovery, PKCE authorization,
// token exchange and refresh, and access token verification. It stores nothing.
package sso

import (
	"context"
	"errors"
	"time"

	"github.com/eve-online-tools/yulai/core/todo"
)

const DefaultIssuer = "https://login.eveonline.com"

// ErrInvalidGrant means the refresh token is dead. The character must log in again.
var ErrInvalidGrant = errors.New("sso: invalid_grant")

// Config is the SSO application registration.
type Config struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret,omitempty"`
	CallbackURL  string `json:"callbackUrl"`
	// Issuer defaults to DefaultIssuer.
	Issuer string `json:"issuer,omitempty"`
}

type Tokens struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

// Identity is what an EVE access token says about its owner.
type Identity struct {
	CharacterID int64
	Name        string
	OwnerHash   string
	Scopes      []string
	IssuedAt    time.Time
}

// PKCE is one login attempt's verifier/challenge pair and state.
type PKCE struct {
	Verifier  string
	Challenge string
	State     string
}

type Client struct {
	cfg Config
}

func NewClient(cfg Config) *Client {
	if cfg.Issuer == "" {
		cfg.Issuer = DefaultIssuer
	}
	return &Client{cfg: cfg}
}

func (c *Client) Config() Config { return c.cfg }

// AuthorizeURL builds the browser URL for one PKCE login attempt.
func (c *Client) AuthorizeURL(ctx context.Context, p *PKCE, scopes []string) (string, error) {
	return "", todo.ErrNotImplemented
}

func (c *Client) Exchange(ctx context.Context, code string, p *PKCE) (*Tokens, error) {
	return nil, todo.ErrNotImplemented
}

// Refresh returns ErrInvalidGrant when the refresh token is dead.
func (c *Client) Refresh(ctx context.Context, refreshToken string) (*Tokens, error) {
	return nil, todo.ErrNotImplemented
}

// Verifier checks access tokens against the SSO's JWKS (jwx, auto-refreshing keys).
type Verifier struct {
	client *Client
}

func NewVerifier(client *Client) *Verifier { return &Verifier{client: client} }

func (v *Verifier) Verify(ctx context.Context, accessToken string) (*Identity, error) {
	return nil, todo.ErrNotImplemented
}
