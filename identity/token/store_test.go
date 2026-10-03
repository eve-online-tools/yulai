package token

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
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

// fakeSSO rotates the refresh token on every refresh, or rejects it once revoked is
// set. Its JWKS is empty, so Verify fails.
func fakeSSO(t *testing.T, revoked *atomic.Bool) *httptest.Server {
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
		if revoked.Load() {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"invalid_grant","error_description":"revoked"}`))
			return
		}
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

type deadCalls struct {
	mu  sync.Mutex
	ids []int64
}

func (d *deadCalls) record(_ context.Context, id int64, _ string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ids = append(d.ids, id)
}

func (d *deadCalls) count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.ids)
}

type testStore struct {
	*Store
	revoked atomic.Bool
	dead    deadCalls
	key     *memKey
}

func newTestStore(t *testing.T) *testStore {
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
	ts := &testStore{key: &memKey{}}
	sealer, err := crypt.Open(ts.key)
	if err != nil {
		t.Fatal(err)
	}
	client := sso.NewClient(sso.Config{ClientID: "id", Issuer: fakeSSO(t, &ts.revoked).URL})
	ts.Store = NewStore(conn, client, sso.NewVerifier(client), sealer, ts.dead.record)
	login(t, ts.Store, "login")
	return ts
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
	if got := storedRefresh(t, s.Store); got != "refresh-1" {
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
	if got := storedRefresh(t, s.Store); got != "refresh-2" {
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
	login(t, s.Store, "relogin")

	if err := stale.refresh(ctx, true); !errors.Is(err, ErrReplaced) {
		t.Fatalf("stale refresh err = %v, want ErrReplaced", err)
	}
	if got := storedRefresh(t, s.Store); got != "relogin" {
		t.Fatalf("stored refresh token = %q, want relogin", got)
	}

	// The stale handle reloads the new login on next use.
	if err := stale.refresh(ctx, true); err != nil {
		t.Fatalf("refresh after reload: %v", err)
	}
}

func TestInvalidGrantReportsDead(t *testing.T) {
	s := newTestStore(t)
	s.revoked.Store(true)

	if err := s.Refresh(context.Background(), charID); !errors.Is(err, sso.ErrInvalidGrant) {
		t.Fatalf("refresh err = %v, want ErrInvalidGrant", err)
	}
	if got := s.dead.count(); got != 1 {
		t.Fatalf("onDead calls = %d, want 1", got)
	}
}

func TestUnreadableTokenReportsDead(t *testing.T) {
	s := newTestStore(t)
	// A lost keyring entry: a new master key cannot open the stored token.
	s.key.v = nil
	sealer, err := crypt.Open(s.key)
	if err != nil {
		t.Fatal(err)
	}
	s.sealer = sealer

	if err := s.Refresh(context.Background(), charID); !errors.Is(err, ErrUnreadable) {
		t.Fatalf("refresh err = %v, want ErrUnreadable", err)
	}
	if got := s.dead.count(); got != 1 {
		t.Fatalf("onDead calls = %d, want 1", got)
	}
}
