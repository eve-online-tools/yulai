// Package sso speaks the EVE Online SSO protocol: discovery, PKCE authorization,
// token exchange and refresh, and access token verification. It stores nothing.
package sso

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrInvalidGrant means the refresh token is dead. The character must log in again.
var ErrInvalidGrant = errors.New("sso: invalid_grant")

// Config is the SSO application registration.
type Config struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ClientID    string `json:"clientId"`
	CallbackURL string `json:"callbackUrl"`
	// Issuer is the SSO host, e.g. "https://login.eveonline.com" or a test server
	// like Singularity's. A bare host gets https. Defaults to DefaultIssuer.
	Issuer string `json:"ssoHost,omitempty"`
}

type Tokens struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

type Client struct {
	cfg  Config
	http *http.Client
	disc *discovery
}

func NewClient(cfg Config) *Client {
	cfg.Issuer = NormalizeIssuer(cfg.Issuer)
	hc := &http.Client{Timeout: 15 * time.Second}
	return &Client{cfg: cfg, http: hc, disc: &discovery{issuer: cfg.Issuer, http: hc}}
}

func (c *Client) Config() Config { return c.cfg }

// NormalizeIssuer defaults an empty issuer, adds https to a bare host and drops a
// trailing slash, so it compares equal to the discovery document and the iss claim.
func NormalizeIssuer(issuer string) string {
	issuer = strings.TrimRight(strings.TrimSpace(issuer), "/")
	if issuer == "" {
		return DefaultIssuer
	}
	if !strings.Contains(issuer, "://") {
		issuer = "https://" + issuer
	}
	return issuer
}

// Endpoints returns the discovered SSO endpoints.
func (c *Client) Endpoints(ctx context.Context) (*Endpoints, error) { return c.disc.endpoints(ctx) }

// AuthorizeURL builds the browser URL for one PKCE login attempt.
func (c *Client) AuthorizeURL(ctx context.Context, p *PKCE, scopes []string) (string, error) {
	eps, err := c.disc.endpoints(ctx)
	if err != nil {
		return "", err
	}
	params := url.Values{
		"response_type":         {"code"},
		"redirect_uri":          {c.cfg.CallbackURL},
		"client_id":             {c.cfg.ClientID},
		"scope":                 {strings.Join(scopes, " ")},
		"state":                 {p.State},
		"code_challenge":        {p.Challenge},
		"code_challenge_method": {"S256"},
	}
	return eps.AuthorizationEndpoint + "?" + params.Encode(), nil
}

func (c *Client) Exchange(ctx context.Context, code string, p *PKCE) (*Tokens, error) {
	return c.tokenRequest(ctx, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"code_verifier": {p.Verifier},
	})
}

func (c *Client) Refresh(ctx context.Context, refreshToken string) (*Tokens, error) {
	return c.tokenRequest(ctx, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	})
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

func (c *Client) tokenRequest(ctx context.Context, form url.Values) (*Tokens, error) {
	eps, err := c.disc.endpoints(ctx)
	if err != nil {
		return nil, err
	}
	form.Set("client_id", c.cfg.ClientID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, eps.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return nil, fmt.Errorf("sso: token endpoint %d: %s", resp.StatusCode, string(body))
	}
	if tr.Error == "invalid_grant" {
		return nil, fmt.Errorf("%w: %s", ErrInvalidGrant, tr.ErrorDesc)
	}
	if tr.Error != "" || resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sso: token endpoint %d: %s %s", resp.StatusCode, tr.Error, tr.ErrorDesc)
	}
	return &Tokens{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second),
	}, nil
}
