package sso

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// DefaultIssuer is EVE's SSO for Tranquility.
const DefaultIssuer = "https://login.eveonline.com"

// Endpoints is the subset of the OpenID discovery document we use.
type Endpoints struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	RevocationEndpoint    string `json:"revocation_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

// discovery fetches the well-known document once and keeps it. A failed fetch is not
// cached, so an offline start recovers on the next call.
type discovery struct {
	issuer string
	http   *http.Client

	mu  sync.Mutex
	eps *Endpoints
}

func (d *discovery) endpoints(ctx context.Context) (*Endpoints, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.eps != nil {
		return d.eps, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.issuer+"/.well-known/openid-configuration", nil)
	if err != nil {
		return nil, err
	}
	resp, err := d.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sso: discovery: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sso: discovery: status %d", resp.StatusCode)
	}

	var eps Endpoints
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&eps); err != nil {
		return nil, fmt.Errorf("sso: discovery: %w", err)
	}
	if eps.Issuer != d.issuer {
		return nil, fmt.Errorf("sso: discovery issuer %q does not match %q", eps.Issuer, d.issuer)
	}
	if eps.AuthorizationEndpoint == "" || eps.TokenEndpoint == "" || eps.JWKSURI == "" {
		return nil, fmt.Errorf("sso: discovery document incomplete")
	}
	d.eps = &eps
	return d.eps, nil
}
