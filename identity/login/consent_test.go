package login

import (
	"context"
	"database/sql"
	"testing"
)

type recordBrowser struct{ url string }

func (b *recordBrowser) OpenURL(url string) error { b.url = url; return nil }

type fixedScopes map[int64][]string

func (f fixedScopes) Scopes(_ context.Context, id int64) ([]string, error) {
	s, ok := f[id]
	if !ok {
		return nil, sql.ErrNoRows
	}
	return s, nil
}

func TestRelogin(t *testing.T) {
	h := newHarness(t)
	tests := []struct {
		name string
		id   int64
		want string
	}{
		// Wallet also needs the wallet scope, so only Assets is checked.
		{"preselects covered features", 1, "?feature=&feature=Assets"},
		{"unknown character opens an empty picker", 2, "?feature="},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &recordBrowser{}
			c := NewConsent(h.srv, b, fixedScopes{1: {"esi-assets.read_assets.v1"}})
			if err := c.Relogin(t.Context(), tt.id); err != nil {
				t.Fatal(err)
			}
			if want := h.srv.URL() + tt.want; b.url != want {
				t.Errorf("opened %q, want %q", b.url, want)
			}
		})
	}
}

func TestAddCharacterOpensPicker(t *testing.T) {
	h := newHarness(t)
	b := &recordBrowser{}
	c := NewConsent(h.srv, b, fixedScopes{})
	if err := c.AddCharacter(); err != nil {
		t.Fatal(err)
	}
	if b.url != h.srv.URL() {
		t.Errorf("opened %q, want %q", b.url, h.srv.URL())
	}
	if got := c.Features(); len(got) != len(features) || got[0].Name != features[0].Name {
		t.Errorf("Features = %v, want %v", got, features)
	}
}
