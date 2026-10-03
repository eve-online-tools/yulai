package presence

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eve-online-tools/lib-esi-go/middleware/authentication"

	"github.com/eve-online-tools/yulai/core/db"
	"github.com/eve-online-tools/yulai/core/task"
	"github.com/eve-online-tools/yulai/feature"
)

const pilot = 90000001

// fakeESI answers by path suffix.
type fakeESI map[string]string

func (f fakeESI) RoundTrip(r *http.Request) (*http.Response, error) {
	for suffix, body := range f {
		if strings.HasSuffix(strings.TrimSuffix(r.URL.Path, "/"), suffix) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {"application/json"}},
				Body:       io.NopCloser(strings.NewReader(body)),
				Request:    r,
			}, nil
		}
	}
	return &http.Response{StatusCode: http.StatusNotFound, Body: io.NopCloser(strings.NewReader("{}")), Request: r}, nil
}

type fakeToken struct{}

func (fakeToken) Owner() int64                          { return pilot }
func (fakeToken) Token() string                         { return "access" }
func (fakeToken) RefreshIfNeeded(context.Context) error { return nil }

type fakeTokens struct{}

func (fakeTokens) For(int64) authentication.RefreshableToken { return fakeToken{} }

type fakeChars []int64

func (c fakeChars) WithFeature(context.Context, feature.Feature) ([]int64, error) { return c, nil }

type countEmitter struct{ n int }

func (c *countEmitter) Emit(string) { c.n++ }

func newTestFeature(t *testing.T, esi fakeESI) (*Feature, *sql.DB, *countEmitter) {
	t.Helper()
	conn, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if _, err := conn.Exec(`INSERT INTO characters (id, name, owner_hash, added_at, updated_at) VALUES (?, 'Test Pilot', 'owner', 0, 0)`, pilot); err != nil {
		t.Fatal(err)
	}
	events := &countEmitter{}
	return NewFeature(conn, &http.Client{Transport: esi}, fakeTokens{}, fakeChars{pilot}, events), conn, events
}

const (
	onlineBody   = `{"online":true,"last_login":"2026-10-03T10:00:00Z","last_logout":"2026-10-02T10:00:00Z","logins":7}`
	locationBody = `{"solar_system_id":30000142,"station_id":60003760}`
	shipBody     = `{"ship_item_id":1000000000001,"ship_name":"Pod","ship_type_id":670}`
)

func TestInterval(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	recent := now.Add(-5 * time.Minute).Unix()
	long := now.Add(-20 * time.Minute).Unix()
	online := &Presence{Online: 1}
	offlineRecent := &Presence{LastLogout: &recent}
	offlineLong := &Presence{LastLogout: &long}

	cases := []struct {
		name string
		p    part
		r    *Presence
		want time.Duration
	}{
		{"online unknown", partOnline, nil, onlineFast},
		{"online while online", partOnline, online, onlineFast},
		{"online just logged out", partOnline, offlineRecent, onlineFast},
		{"online logged out long ago", partOnline, offlineLong, onlineSlow},
		{"online no logout time", partOnline, &Presence{}, onlineSlow},
		{"location online", partLocation, online, locationFast},
		{"location offline", partLocation, offlineRecent, offlineEvery},
		{"location unknown", partLocation, nil, offlineEvery},
		{"ship online", partShip, online, shipFast},
		{"ship offline", partShip, offlineLong, offlineEvery},
	}
	for _, c := range cases {
		if got := interval(c.p, c.r, now); got != c.want {
			t.Errorf("%s: interval = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestFetchPersists(t *testing.T) {
	ctx := context.Background()
	f, _, events := newTestFeature(t, fakeESI{"/online": onlineBody, "/location": locationBody, "/ship": shipBody})

	if on, err := f.online(ctx, Input{pilot}); err != nil || !on {
		t.Fatalf("online = %v, %v", on, err)
	}
	if _, err := f.location(ctx, Input{pilot}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.ship(ctx, Input{pilot}); err != nil {
		t.Fatal(err)
	}
	r, err := f.q.Get(ctx, pilot)
	if err != nil {
		t.Fatal(err)
	}
	if r.Online != 1 || *r.Logins != 7 || *r.SolarSystemID != 30000142 || *r.StationID != 60003760 ||
		r.StructureID != nil || *r.ShipTypeID != 670 || *r.ShipName != "Pod" || r.OnlineAt == nil {
		t.Fatalf("stored %+v", r)
	}
	if events.n != 3 {
		t.Fatalf("events = %d, want 3", events.n)
	}

	// Unchanged data is written but not announced.
	if _, err := f.location(ctx, Input{pilot}); err != nil {
		t.Fatal(err)
	}
	if events.n != 3 {
		t.Fatalf("events = %d after unchanged fetch, want 3", events.n)
	}
}

func TestDue(t *testing.T) {
	ctx := context.Background()
	f, conn, _ := newTestFeature(t, fakeESI{"/online": onlineBody})

	for _, p := range []part{partOnline, partLocation, partShip} {
		if got, _ := f.due(p)(ctx); len(got) != 1 {
			t.Fatalf("part %d due = %v at startup, want the pilot", p, got)
		}
		f.mark(pilot, p)
		if got, _ := f.due(p)(ctx); len(got) != 0 {
			t.Fatalf("part %d due = %v right after a fetch, want none", p, got)
		}
	}

	// No stored row counts as offline, so location waits for the slow pace.
	f.fetched[fetchKey{pilot, partLocation}] = time.Now().Add(-time.Minute)
	if got, _ := f.due(partLocation)(ctx); len(got) != 0 {
		t.Fatalf("location due = %v while offline, want none", got)
	}

	// Coming online makes location due right away and then every locationFast.
	if _, err := f.online(ctx, Input{pilot}); err != nil {
		t.Fatal(err)
	}
	if got, _ := f.due(partLocation)(ctx); len(got) != 1 {
		t.Fatalf("location due = %v after coming online, want the pilot", got)
	}
	f.fetched[fetchKey{pilot, partLocation}] = time.Now().Add(-locationFast)
	if got, _ := f.due(partLocation)(ctx); len(got) != 1 {
		t.Fatalf("location due = %v after %v online, want the pilot", got, locationFast)
	}

	// Removing the character drops its presence.
	if _, err := conn.Exec(`DELETE FROM characters WHERE id = ?`, pilot); err != nil {
		t.Fatal(err)
	}
	if rows, _ := f.q.List(ctx); len(rows) != 0 {
		t.Fatalf("presence rows = %v after delete, want none", rows)
	}
}

// Task options are validated at Bind and Register, which panic on mistakes.
func TestTasksRegister(t *testing.T) {
	f, _, _ := newTestFeature(t, fakeESI{})
	task.NewScheduler(task.Options{}).Register(f.Tasks()...)
}
