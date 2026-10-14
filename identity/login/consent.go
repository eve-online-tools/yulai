package login

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"slices"
)

// Browser opens URLs in the system browser. Implemented by app with Wails.
type Browser interface {
	OpenURL(url string) error
}

// ScopeReader reads the scopes on a character's stored token. Implemented by identity/token.
type ScopeReader interface {
	Scopes(ctx context.Context, characterID int64) ([]string, error)
}

// Consent is the Wails-facing side of the picker: what can be opted into, and opening
// the picker to add a character or log one in again.
type Consent struct {
	url      string
	features []Feature
	browser  Browser
	scopes   ScopeReader
}

func NewConsent(s *Server, browser Browser, scopes ScopeReader) *Consent {
	return &Consent{url: s.URL(), features: s.features, browser: browser, scopes: scopes}
}

func (c *Consent) ServiceName() string { return "ConsentService" }

// Features lists every opt-in feature and the scopes it needs. The frontend diffs
// them against a character's token scopes to show what is enabled.
func (c *Consent) Features() []Feature { return c.features }

// AddCharacter opens the feature picker in the system browser. Completed logins
// reach the login handler.
func (c *Consent) AddCharacter() error { return c.browser.OpenURL(c.url) }

// Relogin opens the feature picker with the character's current features checked,
// to replace a token that stopped working.
func (c *Consent) Relogin(ctx context.Context, characterID int64) error {
	scopes, err := c.scopes.Scopes(ctx, characterID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	q := url.Values{"feature": {""}}
	for _, f := range c.features {
		if !slices.ContainsFunc(f.Scopes, func(s string) bool { return !slices.Contains(scopes, s) }) {
			q.Add("feature", f.Name)
		}
	}
	return c.browser.OpenURL(c.url + "?" + q.Encode())
}
