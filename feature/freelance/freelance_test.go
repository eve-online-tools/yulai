package freelance

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/eve-online-tools/lib-esi-go/common/uuid"
	"github.com/eve-online-tools/lib-esi-go/middleware/authentication"

	"github.com/eve-online-tools/yulai/core/db"
	"github.com/eve-online-tools/yulai/core/task"
	"github.com/eve-online-tools/yulai/feature"
)

const pilot = 90000001

const (
	jobA = uuid.UUID("aaaaaaaa-0000-4000-8000-000000000001")
	jobB = uuid.UUID("bbbbbbbb-0000-4000-8000-000000000002")
	jobC = uuid.UUID("cccccccc-0000-4000-8000-000000000003")
)

// fakeESI answers by path plus query, e.g. "/freelance-jobs?before=b1". A missing
// key is a 404, a body of "500" a server error. It records the requests made.
type fakeESI struct {
	mu    sync.Mutex
	pages map[string]string
	seen  []string
}

func (f *fakeESI) RoundTrip(r *http.Request) (*http.Response, error) {
	key := strings.TrimSuffix(r.URL.Path, "/")
	if r.URL.RawQuery != "" {
		key += "?" + r.URL.RawQuery
	}
	f.mu.Lock()
	f.seen = append(f.seen, key)
	body, ok := f.pages[key]
	f.mu.Unlock()
	status := http.StatusOK
	switch {
	case !ok:
		status, body = http.StatusNotFound, `{"error":"not found"}`
	case body == "500":
		status, body = http.StatusInternalServerError, `{"error":"boom"}`
	}
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    r,
	}, nil
}

func (f *fakeESI) set(key, body string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pages[key] = body
}

func (f *fakeESI) count(key string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, k := range f.seen {
		if k == key {
			n++
		}
	}
	return n
}

type fakeToken struct{}

func (fakeToken) Owner() int64                          { return pilot }
func (fakeToken) Token() string                         { return "access" }
func (fakeToken) RefreshIfNeeded(context.Context) error { return nil }

type fakeTokens struct{}

func (fakeTokens) For(int64) authentication.RefreshableToken { return fakeToken{} }

type fakeChars []int64

func (c fakeChars) WithScopes(context.Context, ...string) ([]int64, error) { return c, nil }

type countEmitter struct {
	mu sync.Mutex
	n  int
}

func (c *countEmitter) Changed(topic string) {
	if topic == Topic {
		c.mu.Lock()
		c.n++
		c.mu.Unlock()
	}
}

func (c *countEmitter) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

func newTestFeature(t *testing.T) (*Freelance, *fakeESI, *countEmitter, context.Context) {
	t.Helper()
	conn, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "test.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if _, err := conn.Exec(`INSERT INTO characters (id, name, owner_hash, added_at, updated_at) VALUES (?, 'Test Pilot', 'owner', 0, 0)`, pilot); err != nil {
		t.Fatal(err)
	}
	esi := &fakeESI{pages: map[string]string{}}
	events := &countEmitter{}
	f := newFreelance(feature.Deps{
		DB:         conn,
		ESI:        &http.Client{Transport: esi},
		Tokens:     fakeTokens{},
		Characters: fakeChars{pilot},
		Events:     events,
	})

	sched := task.NewScheduler(task.Options{})
	// Empty seeds, so startup ticks stay out of the way of the runs under test.
	sched.Register(
		FetchJobs.Bind(f, task.WithParameters(func(context.Context) ([]ListingInput, error) { return nil, nil })),
		FetchCharacterJobs.Bind(f, task.WithParameters(func(context.Context) ([]Input, error) { return nil, nil })),
		FetchParticipation.Bind(f),
	)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = sched.Start(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	return f, esi, events, task.WithScheduler(ctx, sched)
}

func job(id uuid.UUID, state, modified string) string {
	return `{"id":"` + string(id) + `","name":"Job","state":"` + state + `","last_modified":"` + modified + `",` +
		`"progress":{"current":1,"desired":10},"reward":{"initial":1000,"remaining":900}}`
}

func page(after, before string, jobs ...string) string {
	return `{"cursor":{"after":"` + after + `","before":"` + before + `"},"freelance_jobs":[` + strings.Join(jobs, ",") + `]}`
}

func cursor(t *testing.T, f *Freelance) FreelanceCursor {
	t.Helper()
	c, err := f.q.GetCursor(context.Background(), listingPublic)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCrawl(t *testing.T) {
	f, esi, events, ctx := newTestFeature(t)
	esi.set("/freelance-jobs", page("a1", "b1",
		job(jobA, "active", "2026-10-10T10:00:00Z"),
		job(jobB, "active", "2026-10-10T11:00:00Z")))
	// An older copy of B in the backfill must not replace the newer one.
	esi.set("/freelance-jobs?before=b1", page("x", "b2",
		job(jobC, "completed", "2026-10-01T00:00:00Z"),
		job(jobB, "unpublished", "2026-10-09T00:00:00Z")))
	esi.set("/freelance-jobs?before=b2", page("x", "b3"))
	esi.set("/freelance-jobs?after=a1", page("a1", "x"))

	crawl, err := FetchJobs.Run(ctx, ListingInput{})
	if err != nil || crawl.Pages != 3 || crawl.Changed != 3 {
		t.Fatalf("crawl = %+v, %v", crawl, err)
	}
	jobs, err := f.q.ListJobs(ctx)
	if err != nil || len(jobs) != 3 {
		t.Fatalf("jobs = %+v, %v", jobs, err)
	}
	b, err := f.q.GetJob(ctx, jobB)
	if err != nil || b.State != "active" || *b.RewardRemaining != 900 {
		t.Fatalf("job B = %+v, %v", b, err)
	}
	if c := cursor(t, f); *c.AfterToken != "a1" || c.BeforeToken != nil {
		t.Fatalf("cursor = %+v", c)
	}
	if n := events.count(); n != 2 {
		t.Fatalf("events = %d, want one per changed page", n)
	}

	// Later runs follow after only, and replace with the newer copy.
	esi.set("/freelance-jobs?after=a1", page("a2", "x", job(jobA, "completed", "2026-10-12T00:00:00Z")))
	esi.set("/freelance-jobs?after=a2", page("a2", "x"))
	crawl, err = FetchJobs.Run(ctx, ListingInput{})
	if err != nil || crawl.Pages != 1 || crawl.Changed != 1 {
		t.Fatalf("update = %+v, %v", crawl, err)
	}
	a, err := f.q.GetJob(ctx, jobA)
	if err != nil || a.State != "completed" {
		t.Fatalf("job A = %+v, %v", a, err)
	}
	if c := cursor(t, f); *c.AfterToken != "a2" {
		t.Fatalf("cursor = %+v", c)
	}
	if n := esi.count("/freelance-jobs"); n != 1 {
		t.Fatalf("listing without cursor requested %d times, want 1", n)
	}
	if n := events.count(); n != 3 {
		t.Fatalf("events = %d, want 3", n)
	}
}

func TestCrawlResumes(t *testing.T) {
	f, esi, _, ctx := newTestFeature(t)
	esi.set("/freelance-jobs", page("a1", "b1", job(jobA, "active", "2026-10-10T10:00:00Z")))
	esi.set("/freelance-jobs?before=b1", page("x", "b2", job(jobB, "active", "2026-10-09T00:00:00Z")))
	esi.set("/freelance-jobs?before=b2", "500")

	if _, err := FetchJobs.Run(ctx, ListingInput{}); err == nil {
		t.Fatal("crawl succeeded on a server error")
	}
	if c := cursor(t, f); *c.AfterToken != "a1" || *c.BeforeToken != "b2" {
		t.Fatalf("cursor = %+v", c)
	}

	esi.set("/freelance-jobs?before=b2", page("x", "b3", job(jobC, "active", "2026-10-08T00:00:00Z")))
	esi.set("/freelance-jobs?before=b3", page("x", "b4"))
	esi.set("/freelance-jobs?after=a1", page("a1", "x"))
	crawl, err := FetchJobs.Run(ctx, ListingInput{})
	if err != nil || crawl.Pages != 2 || crawl.Changed != 1 {
		t.Fatalf("crawl = %+v, %v", crawl, err)
	}
	if n := esi.count("/freelance-jobs?before=b1"); n != 1 {
		t.Fatalf("stored page refetched %d times", n)
	}
	jobs, err := f.q.ListJobs(ctx)
	if err != nil || len(jobs) != 3 {
		t.Fatalf("jobs = %+v, %v", jobs, err)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("timed out")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestCharacterJobs(t *testing.T) {
	f, esi, _, ctx := newTestFeature(t)
	list := "/characters/90000001/freelance-jobs"
	partA := list + "/" + string(jobA) + "/participation"
	partB := list + "/" + string(jobB) + "/participation"
	esi.set(list, `{"freelance_jobs":[`+job(jobA, "active", "2026-10-10T10:00:00Z")+`,`+job(jobB, "active", "2026-10-10T11:00:00Z")+`]}`)
	esi.set(partA, `{"contributed":5,"state":"committed","last_modified":"2026-10-10T10:00:00Z"}`)
	// No participation in B: a 404.

	rows, err := FetchCharacterJobs.Run(ctx, Input{CharacterID: pilot})
	if err != nil || len(rows) != 2 {
		t.Fatalf("jobs = %+v, %v", rows, err)
	}
	waitFor(t, func() bool { return esi.count(partA) == 1 && esi.count(partB) == 1 })
	waitFor(t, func() bool {
		p, err := f.q.GetParticipation(ctx, GetParticipationParams{CharacterID: pilot, JobID: jobA})
		return err == nil && p.Contributed == 5
	})

	// Unchanged jobs with participation stored, or known to have none, are not asked again.
	if _, err := FetchCharacterJobs.Run(ctx, Input{CharacterID: pilot}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if a, b := esi.count(partA), esi.count(partB); a != 1 || b != 1 {
		t.Fatalf("participation requests = %d, %d, want 1, 1", a, b)
	}

	// A job that leaves the listing takes its participation with it.
	esi.set(list, `{"freelance_jobs":[`+job(jobB, "active", "2026-10-10T11:00:00Z")+`]}`)
	if _, err := FetchCharacterJobs.Run(ctx, Input{CharacterID: pilot}); err != nil {
		t.Fatal(err)
	}
	parts, err := f.q.ListParticipation(ctx, pilot)
	if err != nil || len(parts) != 0 {
		t.Fatalf("participation = %+v, %v", parts, err)
	}
}
