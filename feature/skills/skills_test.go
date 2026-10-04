package skills

import (
	"context"
	"database/sql"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

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
	attributesBody = `{"charisma":19,"intelligence":24,"memory":24,"perception":20,"willpower":20,"bonus_remaps":1,"last_remap_date":"2026-01-02T03:04:05Z"}`
	skillsBody     = `{"total_sp":5000000,"unallocated_sp":1000,"skills":[` +
		`{"skill_id":3413,"active_skill_level":4,"trained_skill_level":5,"skillpoints_in_skill":256000},` +
		`{"skill_id":3300,"active_skill_level":3,"trained_skill_level":3,"skillpoints_in_skill":8000}]}`
	queueBody = `[` +
		`{"queue_position":1,"skill_id":3301,"finished_level":2,"start_date":"2026-10-04T12:00:00Z","finish_date":"2026-10-04T13:00:00Z"},` +
		`{"queue_position":0,"skill_id":3300,"finished_level":4,"start_date":"2026-10-04T10:00:00Z","finish_date":"2026-10-04T12:00:00Z","training_start_sp":8000,"level_start_sp":8000,"level_end_sp":45255}]`
)

func TestFetchPersists(t *testing.T) {
	ctx := context.Background()
	f, _, events := newTestFeature(t, fakeESI{"/attributes": attributesBody, "/skills": skillsBody, "/skillqueue": queueBody})

	attrs, err := f.attributes(ctx, Input{pilot})
	if err != nil || attrs.Intelligence != 24 || *attrs.BonusRemaps != 1 || attrs.AccruedRemapCooldownDate != nil {
		t.Fatalf("attributes = %+v, %v", attrs, err)
	}
	list, err := f.skills(ctx, Input{pilot})
	if err != nil || list.Totals.TotalSp != 5000000 || len(list.Skills) != 2 || list.Skills[0].SkillID != 3300 {
		t.Fatalf("skills = %+v, %v", list, err)
	}
	queue, err := f.queue(ctx, Input{pilot})
	if err != nil || len(queue) != 2 || queue[0].SkillID != 3300 || *queue[0].LevelEndSp != 45255 || queue[1].TrainingStartSp != nil {
		t.Fatalf("queue = %+v, %v", queue, err)
	}
	if events.n != 3 {
		t.Fatalf("events = %d, want 3", events.n)
	}

	// Unchanged data is written but not announced.
	for _, run := range []func() error{
		func() error { _, err := f.attributes(ctx, Input{pilot}); return err },
		func() error { _, err := f.skills(ctx, Input{pilot}); return err },
		func() error { _, err := f.queue(ctx, Input{pilot}); return err },
	} {
		if err := run(); err != nil {
			t.Fatal(err)
		}
	}
	if events.n != 3 {
		t.Fatalf("events = %d after unchanged fetches, want 3", events.n)
	}
}

func TestListsReplaced(t *testing.T) {
	ctx := context.Background()
	esi := fakeESI{"/skills": skillsBody, "/skillqueue": queueBody}
	f, conn, events := newTestFeature(t, esi)
	if _, err := f.skills(ctx, Input{pilot}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.queue(ctx, Input{pilot}); err != nil {
		t.Fatal(err)
	}
	before := events.n

	esi["/skills"] = `{"total_sp":5000000,"skills":[{"skill_id":3300,"active_skill_level":3,"trained_skill_level":3,"skillpoints_in_skill":8000}]}`
	esi["/skillqueue"] = `[]`
	list, err := f.skills(ctx, Input{pilot})
	if err != nil || len(list.Skills) != 1 || list.Totals.UnallocatedSp != nil {
		t.Fatalf("skills = %+v, %v", list, err)
	}
	queue, err := f.queue(ctx, Input{pilot})
	if err != nil || len(queue) != 0 {
		t.Fatalf("queue = %+v, %v", queue, err)
	}
	if events.n != before+2 {
		t.Fatalf("events = %d, want %d", events.n, before+2)
	}

	// Removing the character drops its skills.
	if _, err := conn.Exec(`DELETE FROM characters WHERE id = ?`, pilot); err != nil {
		t.Fatal(err)
	}
	if rows, _ := f.q.ListSkills(ctx, pilot); len(rows) != 0 {
		t.Fatalf("skills = %v after delete, want none", rows)
	}
}

func TestSeed(t *testing.T) {
	f, _, _ := newTestFeature(t, fakeESI{})
	got, err := f.characters(context.Background())
	if err != nil || len(got) != 1 || got[0].CharacterID != pilot {
		t.Fatalf("characters = %v, %v", got, err)
	}
}

// Task options are validated at Bind and Register, which panic on mistakes.
func TestTasksRegister(t *testing.T) {
	f, _, _ := newTestFeature(t, fakeESI{})
	task.NewScheduler(task.Options{}).Register(f.Tasks()...)
}
