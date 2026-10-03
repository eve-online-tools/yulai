package task

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/eve-online-tools/yulai/core/task/pool"
)

type charIn struct{ ID int64 }

func (c charIn) Subject() string { return fmt.Sprintf("char:%d", c.ID) }

type recv struct {
	mu    sync.Mutex
	calls []int64
	fail  error
	block chan struct{}
	deny  int64 // CanRun denies this ID
}

func (r *recv) Do(ctx context.Context, in charIn) (string, error) {
	r.mu.Lock()
	r.calls = append(r.calls, in.ID)
	fail, block := r.fail, r.block
	r.mu.Unlock()
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	if fail != nil {
		return "", fail
	}
	return fmt.Sprintf("done %d", in.ID), nil
}

func (r *recv) Panic(context.Context, charIn) (string, error) { panic("boom") }

func (r *recv) CanRun(_ context.Context, in charIn) Verdict {
	if in.ID == r.deny {
		return Deny("denied")
	}
	return Allow()
}

func (r *recv) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

func (r *recv) set(f func(r *recv)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	f(r)
}

func seedOf(ids ...int64) Seed[charIn] {
	return func(context.Context) ([]charIn, error) {
		out := make([]charIn, len(ids))
		for i, id := range ids {
			out[i] = charIn{id}
		}
		return out, nil
	}
}

func newScheduler(o Options) *Scheduler {
	if o.Log == nil {
		o.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return NewScheduler(o)
}

// started registers the bindings and runs the scheduler until the test ends.
func started(t *testing.T, o Options, bs ...Binding) *Scheduler {
	s := newScheduler(o)
	s.Register(bs...)
	go s.Start(t.Context())
	synctest.Wait()
	return s
}

func status(s *Scheduler, subject string) Status {
	for _, st := range s.List() {
		if st.Subject == subject {
			return st
		}
	}
	return Status{}
}

func mustPanic(t *testing.T, want string, f func()) {
	t.Helper()
	defer func() {
		p := recover()
		if p == nil {
			t.Fatalf("no panic, want %q", want)
		}
		if !strings.Contains(fmt.Sprint(p), want) {
			t.Fatalf("panic %q, want %q", p, want)
		}
	}()
	f()
}

func TestRunReturnsOutput(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tk := New((*recv).Do)
		s := newScheduler(Options{})
		s.Register(tk.Bind(&recv{}))

		out, err := tk.On(s).Run(t.Context(), charIn{7})
		if err != nil || out != "done 7" {
			t.Fatalf("got %q, %v", out, err)
		}
	})
}

func TestResolve(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tk := New((*recv).Do)
		s := newScheduler(Options{})
		s.Register(tk.Bind(&recv{}))

		if _, err := tk.Run(t.Context(), charIn{1}); !errors.Is(err, ErrNoScheduler) {
			t.Fatalf("err = %v, want ErrNoScheduler", err)
		}
		if _, err := tk.Run(WithScheduler(t.Context(), s), charIn{1}); err != nil {
			t.Fatal(err)
		}

		Default = s
		defer func() { Default = nil }()
		if _, err := tk.Run(t.Context(), charIn{1}); err != nil {
			t.Fatal(err)
		}
		if _, err := New((*recv).Do).Run(t.Context(), charIn{1}); !errors.Is(err, ErrNotRegistered) {
			t.Fatalf("err = %v, want ErrNotRegistered", err)
		}
	})
}

func TestNestedQueueStaysOnScheduler(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		inner := &recv{}
		innerTask := New((*recv).Do)
		outer := New(func(_ *recv, ctx context.Context, in charIn) (string, error) {
			return "", innerTask.Queue(ctx, in)
		})
		s := newScheduler(Options{})
		s.Register(innerTask.Bind(inner), outer.Bind(&recv{}))

		if _, err := outer.On(s).Run(t.Context(), charIn{1}); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if inner.count() != 1 {
			t.Fatalf("inner ran %d times", inner.count())
		}
	})
}

func TestValidation(t *testing.T) {
	mustPanic(t, "needs WithInterval or WithStartup", func() {
		New((*recv).Do, WithParameters(seedOf(1)))
	})
	mustPanic(t, "WithParameters seed", func() {
		New((*recv).Do, WithStartup(), WithParameters(Seed[int](nil)))
	})
	mustPanic(t, "WithCheck", func() {
		New((*recv).Do, WithCheck(func(*recv, context.Context, int) Verdict { return Allow() }))
	})
	mustPanic(t, "duplicate Pausable name", func() {
		s := newScheduler(Options{})
		s.Register(New((*recv).Do, Pausable("x")).Bind(&recv{}), New((*recv).Do, Pausable("x")).Bind(&recv{}))
	})

	// A func literal infers the seed type.
	New((*recv).Do, WithStartup(), WithParameters(func(context.Context) ([]charIn, error) { return nil, nil }))
}

func TestName(t *testing.T) {
	if got := New((*recv).Do).Name(); got != "core/task.(*recv).Do" {
		t.Fatalf("name = %q", got)
	}
	if got := New((*recv).Do, Pausable("wallet.balance")).Name(); got != "wallet.balance" {
		t.Fatalf("name = %q", got)
	}
}

func TestIntervalSpreadsInputs(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := &recv{}
		started(t, Options{}, New((*recv).Do, WithParameters(seedOf(1, 2, 3)), WithInterval(time.Minute)).Bind(r))

		if r.count() != 0 {
			t.Fatal("ran before the first tick")
		}
		// Spread is min(interval/2, 30s) / 3 inputs = 10s apart.
		for i, at := range []time.Duration{time.Minute, 10 * time.Second, 10 * time.Second} {
			time.Sleep(at)
			synctest.Wait()
			if r.count() != i+1 {
				t.Fatalf("after step %d: %d runs", i, r.count())
			}
		}
	})
}

func TestStartupWithoutParameters(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := &recv{}
		started(t, Options{}, New((*recv).Do, WithStartup()).Bind(r))
		if r.count() != 1 || r.calls[0] != 0 {
			t.Fatalf("calls = %v, want one zero-input run", r.calls)
		}
	})
}

func TestTickSkipsRunningKey(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := &recv{block: make(chan struct{})}
		s := started(t, Options{}, New((*recv).Do, WithParameters(seedOf(1)), WithStartup(), WithInterval(time.Minute)).Bind(r))

		time.Sleep(time.Minute)
		synctest.Wait()
		if r.count() != 1 {
			t.Fatalf("%d runs, want 1", r.count())
		}
		if st := status(s, "char:1"); !st.Running || st.LastSkip != "already running" {
			t.Fatalf("status = %+v", st)
		}
		close(r.block)
	})
}

func TestOnDemandCoalesces(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := &recv{block: make(chan struct{})}
		tk := New((*recv).Do)
		s := newScheduler(Options{})
		s.Register(tk.Bind(r))
		h := tk.On(s)

		results := make(chan error, 3)
		run := func() {
			_, err := h.Run(t.Context(), charIn{1})
			results <- err
		}
		go run()
		synctest.Wait()
		// Both coalesce into one follow-up run.
		go run()
		if err := h.Queue(t.Context(), charIn{1}); err != nil {
			t.Fatal(err)
		}
		go run()
		synctest.Wait()

		close(r.block)
		for range 3 {
			if err := <-results; err != nil {
				t.Fatal(err)
			}
		}
		synctest.Wait()
		if r.count() != 2 {
			t.Fatalf("%d runs, want 2", r.count())
		}
	})
}

func TestConditions(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := &recv{deny: 2}
		gate := NewGate("esi error limit")
		tk := New((*recv).Do, When(gate), WithCheck((*recv).CanRun))
		s := newScheduler(Options{})
		s.Register(tk.Bind(r))
		h := tk.On(s)

		gate.Close(time.Now().Add(time.Minute))
		_, err := h.Run(t.Context(), charIn{1})
		var be *BlockedError
		if !errors.As(err, &be) || be.Reason != "esi error limit" || !errors.Is(err, ErrBlocked) {
			t.Fatalf("err = %v", err)
		}

		time.Sleep(time.Minute)
		if _, err := h.Run(t.Context(), charIn{1}); err != nil {
			t.Fatalf("gate should reopen at until: %v", err)
		}
		if _, err := h.Run(t.Context(), charIn{2}); !errors.As(err, &be) || be.Reason != "denied" {
			t.Fatalf("check: err = %v", err)
		}
		if st := status(s, "char:2"); st.LastSkip != "denied" {
			t.Fatalf("status = %+v", st)
		}
	})
}

func TestBackoff(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := &recv{fail: errors.New("esi 502")}
		tk := New((*recv).Do, WithParameters(seedOf(1)), WithStartup(), WithInterval(10*time.Second))
		s := started(t, Options{}, tk.Bind(r))

		// Fails at 0s, backoff 30s: ticks at 10s and 20s skip, 30s runs.
		time.Sleep(25 * time.Second)
		synctest.Wait()
		if r.count() != 1 || status(s, "char:1").LastSkip != "backing off after failure" {
			t.Fatalf("runs = %d, status = %+v", r.count(), status(s, "char:1"))
		}
		time.Sleep(5 * time.Second)
		synctest.Wait()
		if r.count() != 2 {
			t.Fatalf("runs = %d, want 2", r.count())
		}
		// Second failure doubles to 60s.
		time.Sleep(55 * time.Second)
		synctest.Wait()
		if r.count() != 2 {
			t.Fatalf("runs = %d, want 2 during backoff", r.count())
		}
		if st := status(s, "char:1"); st.Failures != 2 || st.LastError != "esi 502" {
			t.Fatalf("status = %+v", st)
		}

		// On demand bypasses backoff.
		if _, err := tk.On(s).Run(t.Context(), charIn{1}); err == nil {
			t.Fatal("want failure")
		}
		if r.count() != 3 {
			t.Fatalf("runs = %d, want 3", r.count())
		}
	})
}

func TestRetryAt(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := &recv{}
		r.fail = RetryAt(errors.New("420"), time.Now().Add(5*time.Minute))
		tk := New((*recv).Do)
		s := newScheduler(Options{})
		s.Register(tk.Bind(r))

		_, err := tk.On(s).Run(t.Context(), charIn{1})
		if err == nil || err.Error() != "420" {
			t.Fatalf("err = %v", err)
		}
		if got := time.Until(status(s, "char:1").BackoffUntil); got != 5*time.Minute {
			t.Fatalf("backoff = %v, want 5m", got)
		}
	})
}

func TestPauses(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := &recv{}
		tk := New((*recv).Do, WithParameters(seedOf(1, 2)), WithInterval(time.Minute), Pausable("wallet"))
		store := NewMemoryPauses()
		_ = store.Set(t.Context(), Pause{PauseSubject, "char:2"}, true)
		s := started(t, Options{Pauses: store}, tk.Bind(r))

		// Subject pause loaded from the store at Start.
		time.Sleep(time.Minute + 30*time.Second)
		synctest.Wait()
		if r.count() != 1 || r.calls[0] != 1 || status(s, "char:2").LastSkip != "subject paused" {
			t.Fatalf("calls = %v", r.calls)
		}

		if err := s.Pause(t.Context(), Pause{PauseTask, "wallet"}); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Minute)
		synctest.Wait()
		if r.count() != 1 {
			t.Fatalf("ran while task paused: %v", r.calls)
		}

		// On demand ignores pauses.
		if _, err := tk.On(s).Run(t.Context(), charIn{2}); err != nil || r.count() != 2 {
			t.Fatalf("err = %v, calls = %v", err, r.calls)
		}

		if err := s.Resume(t.Context(), Pause{PauseTask, "wallet"}); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Minute)
		synctest.Wait()
		if r.count() != 3 {
			t.Fatalf("calls = %v after resume", r.calls)
		}

		if err := s.Pause(t.Context(), Pause{PauseTask, "nope"}); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
		if ps, _ := store.List(t.Context()); len(ps) != 1 {
			t.Fatalf("store = %v, want the subject pause only", ps)
		}
	})
}

func TestTimeoutAndPanic(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := &recv{block: make(chan struct{})}
		slow := New((*recv).Do, WithTimeout(time.Second))
		bad := New((*recv).Panic)
		s := newScheduler(Options{})
		s.Register(slow.Bind(r), bad.Bind(r))

		if _, err := slow.On(s).Run(t.Context(), charIn{1}); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("err = %v", err)
		}
		if _, err := bad.On(s).Run(t.Context(), charIn{1}); err == nil || !strings.Contains(err.Error(), "panicked: boom") {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestTrigger(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := &recv{}
		s := started(t, Options{}, New((*recv).Do, WithParameters(seedOf(5)), WithStartup(), Pausable("p")).Bind(r))

		if err := s.Trigger(t.Context(), "p", "char:5"); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if r.count() != 2 || r.calls[1] != 5 {
			t.Fatalf("calls = %v", r.calls)
		}
		if err := s.Trigger(t.Context(), "p", "char:6"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestPool(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := &recv{block: make(chan struct{})}
		defer close(r.block)
		tk := New((*recv).Do, WithPool(pool.Create(1)))
		s := newScheduler(Options{})
		s.Register(tk.Bind(r))

		_ = tk.On(s).Queue(t.Context(), charIn{1})
		_ = tk.On(s).Queue(t.Context(), charIn{2})
		synctest.Wait()
		var running, queued int
		for _, st := range s.List() {
			if st.Running {
				running++
			}
			if st.Queued {
				queued++
			}
		}
		if running != 1 || queued != 1 {
			t.Fatalf("list = %+v", s.List())
		}
	})
}

func TestStopCancelsRuns(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := &recv{block: make(chan struct{})}
		tk := New((*recv).Do)
		s := newScheduler(Options{})
		s.Register(tk.Bind(r))
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan struct{})
		go func() { _ = s.Start(ctx); close(done) }()

		errc := make(chan error)
		go func() { _, err := tk.On(s).Run(t.Context(), charIn{1}); errc <- err }()
		synctest.Wait()
		cancel()
		if err := <-errc; !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v", err)
		}
		<-done
		if err := tk.On(s).Queue(t.Context(), charIn{1}); !errors.Is(err, ErrStopped) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestOnChangeDebounced(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var n atomic.Int32
		tk := New((*recv).Do)
		s := newScheduler(Options{OnChange: func() { n.Add(1) }})
		s.Register(tk.Bind(&recv{}))

		for i := range 5 {
			_, _ = tk.On(s).Run(t.Context(), charIn{int64(i)})
		}
		time.Sleep(time.Second)
		if got := n.Load(); got != 1 {
			t.Fatalf("OnChange called %d times, want 1", got)
		}
	})
}

type reporting struct{ step chan struct{} }

func (r *reporting) Do(ctx context.Context, _ struct{}) (struct{}, error) {
	Report(ctx, Progress{Phase: "download", Done: 1, Total: 4})
	<-r.step
	Report(ctx, Progress{Phase: "build", Item: "types.jsonl", Done: 3, Total: 4})
	<-r.step
	return struct{}{}, nil
}

func TestProgress(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		Report(t.Context(), Progress{Done: 1}) // outside a run: no-op

		r := &reporting{step: make(chan struct{})}
		tk := New((*reporting).Do)
		s := newScheduler(Options{})
		s.Register(tk.Bind(r))
		progress := func() *Progress { return status(s, "").Progress }

		if err := tk.On(s).Queue(t.Context(), struct{}{}); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if p := progress(); p == nil || *p != (Progress{Phase: "download", Done: 1, Total: 4}) {
			t.Fatalf("progress = %+v", p)
		}
		r.step <- struct{}{}
		synctest.Wait()
		if p := progress(); p == nil || p.Item != "types.jsonl" || p.Done != 3 {
			t.Fatalf("progress = %+v", p)
		}
		r.step <- struct{}{}
		synctest.Wait()
		if p := progress(); p != nil {
			t.Fatalf("progress after run = %+v", p)
		}
	})
}
