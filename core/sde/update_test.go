package sde

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/eve-online-tools/yulai/core/task"
)

type events struct{ n atomic.Int32 }

func (e *events) Emit(string) { e.n.Add(1) }

// cdn serves latest.jsonl and the zip of one build like developers.eveonline.com.
type cdn struct {
	build int
	zip   []byte

	mu      sync.Mutex
	ranges  []string
	matched int // requests answered with 304
}

func (c *cdn) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/latest.jsonl":
		etag := `"` + strconv.Itoa(c.build) + `"`
		if r.Header.Get("If-None-Match") == etag {
			c.mu.Lock()
			c.matched++
			c.mu.Unlock()
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		w.Write([]byte(`{"_key": "sde", "buildNumber": ` + strconv.Itoa(c.build) + `, "releaseDate": "2026-10-02T11:08:57Z"}` + "\n"))
	case "/eve-online-static-data-" + strconv.Itoa(c.build) + "-jsonl.zip":
		c.mu.Lock()
		c.ranges = append(c.ranges, r.Header.Get("Range"))
		c.mu.Unlock()
		http.ServeContent(w, r, "export.zip", time.Time{}, bytes.NewReader(c.zip))
	default:
		http.NotFound(w, r)
	}
}

func setup(t *testing.T, build int) (*Updater, *cdn, *events, *task.Scheduler) {
	t.Helper()
	zip, err := os.ReadFile(writeZip(t, t.TempDir(), fixture(build)))
	if err != nil {
		t.Fatal(err)
	}
	c := &cdn{build: build, zip: zip}
	srv := httptest.NewServer(c)
	t.Cleanup(srv.Close)

	store, err := Open(t.Context(), t.TempDir(), quiet)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	ev := &events{}
	u := NewUpdater(store, srv.Client(), ev)
	u.base = srv.URL + "/"

	s := task.NewScheduler(task.Options{Log: quiet})
	s.Register(u.Tasks()...)
	return u, c, ev, s
}

func TestCheckQueuesUpdate(t *testing.T) {
	u, c, ev, s := setup(t, 5)
	ctx := task.WithScheduler(t.Context(), s)

	b, err := Check.On(s).Run(ctx, struct{}{})
	if err != nil {
		t.Fatal(err)
	}
	if b.Number != 5 {
		t.Fatalf("latest = %+v", b)
	}
	// Coalesces with the update Check queued, which installs build 5.
	m, err := Update.On(s).Run(ctx, *b)
	if err != nil {
		t.Fatal(err)
	}
	if m == nil || m.Build != 5 {
		t.Fatalf("meta = %+v", m)
	}
	if ev.n.Load() != 1 {
		t.Fatalf("%d change events, want 1", ev.n.Load())
	}
	if entries, _ := filepath.Glob(filepath.Join(u.store.dir, "sde-*")); len(entries) != 0 {
		t.Fatalf("downloads left behind: %v", entries)
	}

	// Unchanged: 304, nothing queued.
	if _, err := Check.On(s).Run(ctx, struct{}{}); err != nil {
		t.Fatal(err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.matched != 1 || len(c.ranges) != 1 {
		t.Fatalf("304s = %d, downloads = %d", c.matched, len(c.ranges))
	}
}

func TestUpdateResumesDownload(t *testing.T) {
	u, c, _, s := setup(t, 6)
	old := filepath.Join(u.store.dir, "sde-1.zip.part")
	part := filepath.Join(u.store.dir, "sde-6.zip.part")
	if err := os.WriteFile(old, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(part, c.zip[:100], 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := Update.On(s).Run(t.Context(), Build{Number: 6})
	if err != nil {
		t.Fatal(err)
	}
	if m.Build != 6 {
		t.Fatalf("meta = %+v", m)
	}
	if len(c.ranges) != 1 || c.ranges[0] != "bytes=100-" {
		t.Fatalf("ranges = %q", c.ranges)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatalf("download of another build kept: %v", err)
	}
}

func TestUpdateRestartsBadPart(t *testing.T) {
	u, c, _, s := setup(t, 7)
	part := filepath.Join(u.store.dir, "sde-7.zip.part")
	if err := os.WriteFile(part, make([]byte, len(c.zip)+10), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Update.On(s).Run(t.Context(), Build{Number: 7}); err == nil {
		t.Fatal("no error for an oversized part")
	}
	if m, err := Update.On(s).Run(t.Context(), Build{Number: 7}); err != nil || m.Build != 7 {
		t.Fatalf("retry: %+v, %v", m, err)
	}
}

func TestServiceStatus(t *testing.T) {
	u, _, _, s := setup(t, 8)
	svc := NewService(u, s.List)
	if st := svc.Status(); st.Build != 0 || st.Updating {
		t.Fatalf("status before check = %+v", st)
	}
	b, err := Check.On(s).Run(task.WithScheduler(t.Context(), s), struct{}{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Update.On(s).Run(t.Context(), *b); err != nil {
		t.Fatal(err)
	}
	st := svc.Status()
	if st.Build != 8 || st.Latest != 8 || st.LastCheck.IsZero() || st.Updating || st.Progress != nil || st.LastError != "" {
		t.Fatalf("status = %+v", st)
	}
}
