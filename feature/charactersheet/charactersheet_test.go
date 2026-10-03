package charactersheet

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/eve-online-tools/yulai/core/db"
	"github.com/eve-online-tools/yulai/core/task"
)

const (
	pilot    = 90000001
	pilotTwo = 90000002

	characterBody   = `{"name":"Test Pilot","corporation_id":98000001,"alliance_id":99000001,"birthday":"2020-01-01T00:00:00Z","bloodline_id":1,"race_id":1,"gender":"female","security_status":1.5}`
	corporationBody = `{"name":"Test Corp","ticker":"TEST","alliance_id":99000001,"ceo_id":90000001,"home_station_id":60003760,"member_count":2,"tax_rates":{"isk":0.1,"loyalty_point":0},"war_eligible":true,"description":"","shares":1000,"state":"active","type":"player","friendly_fire":"legal"}`
	allianceBody    = `{"name":"Test Alliance","ticker":"TSTA","creator_id":90000001,"creator_corporation_id":98000001,"executor_corporation_id":98000001,"date_founded":"2021-01-01T00:00:00Z"}`
)

// fakeESI answers by path prefix and counts requests.
type fakeESI struct {
	mu    sync.Mutex
	calls map[string]int
}

func (f *fakeESI) RoundTrip(r *http.Request) (*http.Response, error) {
	body := "{}"
	kind := strings.Split(strings.Trim(r.URL.Path, "/"), "/")[0]
	switch kind {
	case "characters":
		body = characterBody
	case "corporations":
		body = corporationBody
	case "alliances":
		body = allianceBody
	}
	f.mu.Lock()
	f.calls[kind]++
	f.mu.Unlock()
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    r,
	}, nil
}

func (f *fakeESI) count(kind string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[kind]
}

type fakeChars struct {
	ids  []int64
	mu   sync.Mutex
	corp map[int64]int64
}

func (c *fakeChars) IDs(context.Context) ([]int64, error) { return c.ids, nil }

func (c *fakeChars) SetAffiliation(_ context.Context, id, corp int64, _ *int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.corp[id] = corp
	return nil
}

type nopEmitter struct{}

func (nopEmitter) Emit(string) {}

func newTestSheet(t *testing.T) (*Sheet, *fakeESI, *fakeChars, context.Context) {
	t.Helper()
	conn, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	for _, id := range []int64{pilot, pilotTwo} {
		if _, err := conn.Exec(`INSERT INTO characters (id, name, owner_hash, added_at, updated_at) VALUES (?, 'p', 'o', 0, 0)`, id); err != nil {
			t.Fatal(err)
		}
	}
	esi := &fakeESI{calls: map[string]int{}}
	chars := &fakeChars{ids: []int64{pilot, pilotTwo}, corp: map[int64]int64{}}
	s := NewSheet(conn, &http.Client{Transport: esi}, chars, nopEmitter{})

	sched := task.NewScheduler(task.Options{})
	sched.Register(s.Tasks()...)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = sched.Start(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	return s, esi, chars, task.WithScheduler(ctx, sched)
}

func waitFor[T any](t *testing.T, get func() (T, error)) T {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		v, err := get()
		if err == nil {
			return v
		}
		if err != sql.ErrNoRows || time.Now().After(deadline) {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestFanOut(t *testing.T) {
	s, esi, chars, ctx := newTestSheet(t)

	for _, id := range []int64{pilot, pilotTwo} {
		if _, err := FetchCharacter.Run(ctx, CharacterInput{id}); err != nil {
			t.Fatal(err)
		}
	}
	sheet, err := s.q.GetCharacter(ctx, pilot)
	if err != nil || sheet.CorporationID != 98000001 || *sheet.AllianceID != 99000001 || *sheet.SecurityStatus != 1.5 {
		t.Fatalf("sheet = %+v, %v", sheet, err)
	}
	if chars.corp[pilot] != 98000001 {
		t.Fatalf("affiliation = %v", chars.corp)
	}

	corp := waitFor(t, func() (Corporation, error) { return s.q.GetCorporation(ctx, 98000001) })
	if corp.Ticker != "TEST" || *corp.AllianceID != 99000001 || !corp.WarEligible {
		t.Fatalf("corporation = %+v", corp)
	}
	a := waitFor(t, func() (Alliance, error) { return s.q.GetAlliance(ctx, 99000001) })
	if a.Ticker != "TSTA" || *a.ExecutorCorporationID != 98000001 {
		t.Fatalf("alliance = %+v", a)
	}

	// Two characters in one corporation fetch it, and its alliance, once.
	if n := esi.count("corporations"); n != 1 {
		t.Fatalf("corporation fetched %d times, want 1", n)
	}
	if n := esi.count("alliances"); n != 1 {
		t.Fatalf("alliance fetched %d times, want 1", n)
	}
}

func TestClaim(t *testing.T) {
	s, _, _, _ := newTestSheet(t)
	if !s.claim("corp:1", corporationEvery) {
		t.Fatal("first claim refused")
	}
	if s.claim("corp:1", corporationEvery) {
		t.Fatal("second claim allowed")
	}
	s.fetched["corp:1"] = time.Now().Add(-corporationEvery + slack)
	if !s.claim("corp:1", corporationEvery) {
		t.Fatal("claim refused after the interval")
	}
	s.release("corp:1")
	if !s.claim("corp:1", corporationEvery) {
		t.Fatal("claim refused after release")
	}
}
