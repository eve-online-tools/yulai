package character

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eve-online-tools/yulai/core/crypt"
	"github.com/eve-online-tools/yulai/core/db"
	"github.com/eve-online-tools/yulai/core/keyring"
	"github.com/eve-online-tools/yulai/identity/login"
	"github.com/eve-online-tools/yulai/identity/sso"
	"github.com/eve-online-tools/yulai/identity/token"
)

type memKey struct{ v []byte }

func (m *memKey) Get() ([]byte, error) {
	if m.v == nil {
		return nil, keyring.ErrNotFound
	}
	return m.v, nil
}

func (m *memKey) Set(v []byte) error { m.v = v; return nil }

// fakeESI answers every request with a public character record.
type fakeESI struct{ body string }

func (f fakeESI) RoundTrip(r *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(f.body)),
		Request:    r,
	}, nil
}

type nopEmitter struct{}

func (nopEmitter) Emit(string) {}

type nopEnroller struct{ ids []int64 }

func (n *nopEnroller) Enroll(_ context.Context, id int64) error {
	n.ids = append(n.ids, id)
	return nil
}

func newTestService(t *testing.T, esiBody string) (*Service, *token.Store, *nopEnroller) {
	t.Helper()
	ctx := context.Background()
	conn, err := db.Open(ctx, filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	sealer, err := crypt.Open(&memKey{})
	if err != nil {
		t.Fatal(err)
	}
	client := sso.NewClient(sso.Config{ClientID: "test"})
	tokens := token.NewStore(conn, client, sso.NewVerifier(client), sealer)
	enroller := &nopEnroller{}
	esiClient := &http.Client{Transport: fakeESI{body: esiBody}}
	return NewService(conn, "", nil, nopEmitter{}, tokens, esiClient, nil, enroller), tokens, enroller
}

func result(scopes ...string) *login.Result {
	return &login.Result{
		Identity: sso.Identity{CharacterID: 90000001, Name: "Test Pilot", OwnerHash: "owner", Scopes: scopes, IssuedAt: time.Unix(1700000000, 0)},
		Tokens:   sso.Tokens{AccessToken: "access", RefreshToken: "refresh", ExpiresAt: time.Unix(1700001200, 0)},
	}
}

const pilot = `{"name":"Test Pilot","corporation_id":98000001,"alliance_id":99000001,"birthday":"2020-01-01T00:00:00Z","bloodline_id":1,"race_id":1,"gender":"female"}`

func TestStoreThenRelogReplacesScopes(t *testing.T) {
	ctx := context.Background()
	s, tokens, enroller := newTestService(t, pilot)

	if err := s.Store(ctx, result("esi-a.v1", "esi-b.v1")); err != nil {
		t.Fatal(err)
	}
	rows, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("List = %d rows, want 1", len(rows))
	}
	r := rows[0]
	if r.ID != 90000001 || r.CorporationID != 98000001 || r.AllianceID == nil || *r.AllianceID != 99000001 {
		t.Fatalf("row = %+v", r)
	}
	if r.Scopes != "esi-a.v1 esi-b.v1" || r.Status != StatusOK || r.TokenIssuedAt == nil || *r.TokenIssuedAt != 1700000000 {
		t.Fatalf("row = %+v", r)
	}

	if err := s.MarkNeedsLogin(ctx, r.ID, "invalid_grant"); err != nil {
		t.Fatal(err)
	}
	// Logging in again downscopes and clears the needs-login flag.
	if err := s.Store(ctx, result("esi-a.v1")); err != nil {
		t.Fatal(err)
	}
	got, err := tokens.Scopes(ctx, r.ID)
	if err != nil || len(got) != 1 || got[0] != "esi-a.v1" {
		t.Fatalf("Scopes = %v, %v", got, err)
	}
	rows, _ = s.List(ctx)
	if rows[0].Status != StatusOK || rows[0].StatusError != nil {
		t.Fatalf("status not reset: %+v", rows[0])
	}
	if len(enroller.ids) != 2 {
		t.Fatalf("Enroll called %d times, want 2", len(enroller.ids))
	}
}

func TestRemoveDropsToken(t *testing.T) {
	ctx := context.Background()
	s, tokens, _ := newTestService(t, pilot)
	if err := s.Store(ctx, result("esi-a.v1")); err != nil {
		t.Fatal(err)
	}
	if err := s.Remove(ctx, 90000001); err != nil {
		t.Fatal(err)
	}
	if rows, _ := s.List(ctx); len(rows) != 0 {
		t.Fatalf("List = %v after Remove", rows)
	}
	if _, err := tokens.Scopes(ctx, 90000001); err == nil {
		t.Fatal("token survived character removal")
	}
}

func TestStoreFailsOnESIError(t *testing.T) {
	s, _, _ := newTestService(t, "")
	s.esi = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusNotFound, Status: "404 Not Found", Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"error":"not found"}`)), Request: r}, nil
	})}
	if err := s.Store(context.Background(), result()); err == nil {
		t.Fatal("Store succeeded despite ESI 404")
	}
	if rows, _ := s.List(context.Background()); len(rows) != 0 {
		t.Fatalf("character stored despite ESI error: %v", rows)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
