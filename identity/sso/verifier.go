package sso

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lestrrat-go/httprc/v3"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

// Identity is what an EVE access token says about its owner.
type Identity struct {
	CharacterID int64
	Name        string
	OwnerHash   string
	Scopes      []string
	IssuedAt    time.Time
}

// Verifier checks access tokens against the SSO's JWKS. Keys are fetched from the
// discovered jwks_uri and refreshed in the background, so key rotation just works.
type Verifier struct {
	client *Client

	mu    sync.Mutex
	cache *jwk.Cache
	keys  jwk.Set
}

func NewVerifier(client *Client) *Verifier {
	return &Verifier{client: client}
}

// keySet registers the JWKS URL on first use. Registration needs the discovery
// document, so it cannot happen in the constructor without network.
func (v *Verifier) keySet(ctx context.Context) (jwk.Set, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.keys != nil {
		return v.keys, nil
	}

	eps, err := v.client.Endpoints(ctx)
	if err != nil {
		return nil, err
	}
	if v.cache == nil {
		// The cache runs for the life of the process, not the caller's request.
		c, err := jwk.NewCache(context.Background(), httprc.NewClient())
		if err != nil {
			return nil, fmt.Errorf("sso: jwk cache: %w", err)
		}
		v.cache = c
	}
	if err := v.cache.Register(ctx, eps.JWKSURI, jwk.WithMinInterval(15*time.Minute), jwk.WithMaxInterval(24*time.Hour)); err != nil {
		return nil, fmt.Errorf("sso: register jwks: %w", err)
	}
	set, err := v.cache.CachedSet(eps.JWKSURI)
	if err != nil {
		return nil, fmt.Errorf("sso: jwks: %w", err)
	}
	v.keys = set
	return v.keys, nil
}

// Verify checks signature, expiry, issuer and that the token was issued to this
// client (azp). We are not an API, so aud is not checked.
func (v *Verifier) Verify(ctx context.Context, accessToken string) (*Identity, error) {
	keys, err := v.keySet(ctx)
	if err != nil {
		return nil, err
	}

	tok, err := jwt.ParseString(accessToken,
		jwt.WithKeySet(keys),
		jwt.WithValidate(false),
	)
	if err != nil {
		return nil, fmt.Errorf("sso: parse token: %w", err)
	}
	if err := jwt.Validate(tok,
		jwt.WithIssuer(v.client.cfg.Issuer),
		jwt.WithClaimValue("azp", v.client.cfg.ClientID),
		jwt.WithAcceptableSkew(30*time.Second),
	); err != nil {
		return nil, fmt.Errorf("sso: validate token: %w", err)
	}

	sub, _ := tok.Subject()
	var id int64
	if _, err := fmt.Sscanf(sub, "CHARACTER:EVE:%d", &id); err != nil {
		return nil, fmt.Errorf("sso: unexpected subject %q", sub)
	}

	ident := &Identity{CharacterID: id}
	if iat, ok := tok.IssuedAt(); ok {
		ident.IssuedAt = iat
	}
	_ = tok.Get("name", &ident.Name)
	_ = tok.Get("owner", &ident.OwnerHash)

	// scp is a string for one scope, an array for many.
	var scp any
	_ = tok.Get("scp", &scp)
	switch s := scp.(type) {
	case string:
		ident.Scopes = []string{s}
	case []any:
		for _, x := range s {
			ident.Scopes = append(ident.Scopes, fmt.Sprint(x))
		}
	case []string:
		ident.Scopes = s
	}
	return ident, nil
}
