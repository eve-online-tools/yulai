package token

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eve-online-tools/yulai/core/crypt"
	"github.com/eve-online-tools/yulai/core/db"
	"github.com/eve-online-tools/yulai/core/keyring"
	"github.com/eve-online-tools/yulai/identity/sso"
)

const charID = 90000001

type memKey struct{ v []byte }

func (m *memKey) Get() ([]byte, error) {
	if m.v == nil {
		return nil, keyring.ErrNotFound
	}
	return m.v, nil
}

func (m *memKey) Set(v []byte) error { m.v = v; return nil }

// fakeSSO rotates the refresh token on every refresh. Its JWKS is empty, so Verify fails.
func fakeSSO(t *testing.T) *httptest.Server {
	var n atomic.Int32
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(sso.Endpoints{
			Issuer:                srv.URL,
			AuthorizationEndpoint: srv.URL + "/authorize",
			TokenEndpoint:         srv.URL + "/token",
			JWKSURI:               srv.URL + "/jwks",
		})
	})
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		i := n.Add(1)
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  fmt.Sprintf("access-%d", i),
			"refresh_token": fmt.Sprintf("refresh-%d", i),
			"expires_in":    1200,
		})
	})
	mux.HandleFunc("GET /jwks", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"keys":[]}`))
	})
	return srv
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if _, err := conn.ExecContext(ctx, `INSERT INTO characters (id, name, owner_hash, added_at, updated_at) VALUES (?, 'c', 'h', 0, 0)`, charID); err != nil {
		t.Fatal(err)
	}
	sealer, err := crypt.Open(&memKey{})
	if err != nil {
		t.Fatal(err)
	}
	client := sso.NewClient(sso.Config{ClientID: "id", Issuer: fakeSSO(t).URL})
	s := NewStore(conn, client, sso.NewVerifier(client), sealer)
	login(t, s, "login")
	return s
}

func login(t *testing.T, s *Store, refresh string) {
	t.Helper()
	err := s.Save(context.Background(), charID,
		&sso.Tokens{AccessToken: "a-" + refresh, RefreshToken: refresh, ExpiresAt: time.Now().Add(time.Hour)},
		&sso.Identity{Scopes: []string{"scope-a"}, IssuedAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
}

func storedRefresh(t *testing.T, s *Store) string {
	t.Helper()
	_, tk, err := s.load(context.Background(), charID)
	if err != nil {
		t.Fatal(err)
	}
	return tk.RefreshToken
}

func TestRefreshPersistsRotatedTokenWhenVerifyFails(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if err := s.Refresh(ctx, charID); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if got := storedRefresh(t, s); got != "refresh-1" {
		t.Fatalf("stored refresh token = %q, want refresh-1", got)
	}
	scopes, err := s.Scopes(ctx, charID)
	if err != nil {
		t.Fatal(err)
	}
	if len(scopes) != 1 || scopes[0] != "scope-a" {
		t.Fatalf("scopes = %v, want kept [scope-a]", scopes)
	}

	// The second refresh must use the rotated token, from memory and from the row.
	if err := s.Refresh(ctx, charID); err != nil {
		t.Fatalf("second refresh: %v", err)
	}
	if got := storedRefresh(t, s); got != "refresh-2" {
		t.Fatalf("stored refresh token = %q, want refresh-2", got)
	}
}

func TestRefreshDoesNotOverwriteNewLogin(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	stale := s.For(charID).(*refreshable)
	if err := stale.refresh(ctx, false); err != nil {
		t.Fatal(err)
	}
	login(t, s, "relogin")

	if err := stale.refresh(ctx, true); !errors.Is(err, ErrReplaced) {
		t.Fatalf("stale refresh err = %v, want ErrReplaced", err)
	}
	if got := storedRefresh(t, s); got != "relogin" {
		t.Fatalf("stored refresh token = %q, want relogin", got)
	}

	// The stale handle reloads the new login on next use.
	if err := stale.refresh(ctx, true); err != nil {
		t.Fatalf("refresh after reload: %v", err)
	}
}
