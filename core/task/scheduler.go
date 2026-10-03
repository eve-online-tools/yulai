package task

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/eve-online-tools/yulai/core/task/pool"
)

const (
	backoffMin = 30 * time.Second
	backoffMax = 30 * time.Minute
	// maxSpread caps how far the inputs of one tick are spread out.
	maxSpread      = 30 * time.Second
	changeDebounce = 250 * time.Millisecond
)

var ErrStopped = errors.New("task: scheduler stopped")

// Default is the scheduler Queue and Run use when ctx carries none. Set by app wiring.
var Default *Scheduler

type ctxKey struct{}

// WithScheduler makes Queue and Run on ctx use s instead of Default. Runs get it
// on their ctx, so tasks queued from inside a task stay on the same scheduler.
func WithScheduler(ctx context.Context, s *Scheduler) context.Context {
	return context.WithValue(ctx, ctxKey{}, s)
}

func resolve(ctx context.Context) (*Scheduler, error) {
	if s, ok := ctx.Value(ctxKey{}).(*Scheduler); ok && s != nil {
		return s, nil
	}
	if Default != nil {
		return Default, nil
	}
	return nil, ErrNoScheduler
}

type Options struct {
	// Workers is the size of the pool for tasks without WithPool. Default 4.
	Workers int
	// Pauses persists user pauses. Default: in memory.
	Pauses PauseStore
	Log    *slog.Logger
	// OnChange is called, debounced, after run status changes.
	OnChange func()
	// OnProgress is called for runs of tasks with WithProgress, from the run's goroutine.
	OnProgress func(ProgressEvent)
}

type Scheduler struct {
	log        *slog.Logger
	pool       *pool.Pool
	pauses     PauseStore
	onChange   func()
	onProgress func(ProgressEvent)

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu       sync.Mutex
	started  bool
	stopping bool
	entries  map[any]*entry
	names    map[string]*entry
	progress map[string]*entry // by WithProgress key
	states   map[stateKey]*state
	paused   map[Pause]bool

	changeMu      sync.Mutex
	changePending bool
}

type stateKey struct {
	e       *entry
	subject string
}

type state struct {
	in           any  // last input seen, for Trigger
	running      bool // waiting for a pool slot or executing
	executing    bool
	next         *pending // coalesced follow-up requested while running
	failures     int
	backoffUntil time.Time
	lastRun      time.Time
	lastErr      string
	lastSkip     string
	progress     Progress
	progressSent time.Time
}

type pending struct {
	in      any
	user    bool
	waiters []chan<- result
	// parent is the progress of the run that queued this one, if any.
	parent *reporter
}

type result struct {
	out any
	err error
}

func NewScheduler(o Options) *Scheduler {
	if o.Workers <= 0 {
		o.Workers = 4
	}
	if o.Pauses == nil {
		o.Pauses = NewMemoryPauses()
	}
	if o.Log == nil {
		o.Log = slog.Default()
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		log:        o.Log,
		pool:       pool.Create(o.Workers),
		pauses:     o.Pauses,
		onChange:   o.OnChange,
		onProgress: o.OnProgress,
		ctx:        ctx,
		cancel:     cancel,
		entries:    map[any]*entry{},
		names:      map[string]*entry{},
		progress:   map[string]*entry{},
		states:     map[stateKey]*state{},
		paused:     map[Pause]bool{},
	}
}

// Register adds bound tasks. It panics on a task registered twice, a duplicate
// Pausable name, or when called after Start.
func (s *Scheduler) Register(bs ...Binding) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		panic("task: Register after Start")
	}
	for _, b := range bs {
		e := b.binding()
		if _, dup := s.entries[e.key]; dup {
			panic(fmt.Sprintf("task %s: registered twice", e.name))
		}
		if e.pausable {
			if _, dup := s.names[e.name]; dup {
				panic(fmt.Sprintf("task %s: duplicate Pausable name", e.name))
			}
			s.names[e.name] = e
		}
		if k := e.cfg.progress; k != "" {
			if _, dup := s.progress[k]; dup {
				panic(fmt.Sprintf("task %s: duplicate progress key %q", e.name, k))
			}
			s.progress[k] = e
		}
		s.entries[e.key] = e
	}
}

// Start loads pauses, starts the schedules and blocks until ctx ends. It then
// waits for in-flight runs, whose ctx is cancelled, to return.
func (s *Scheduler) Start(ctx context.Context) error {
	pauses, err := s.pauses.List(ctx)
	if err != nil {
		return fmt.Errorf("task: load pauses: %w", err)
	}

	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return errors.New("task: scheduler already started")
	}
	s.started = true
	for _, p := range pauses {
		s.paused[p] = true
	}
	for _, e := range s.entries {
		if e.cfg.startup || e.cfg.interval > 0 {
			s.wg.Add(1)
			go s.loop(e)
		}
	}
	s.mu.Unlock()

	select {
	case <-ctx.Done():
	case <-s.ctx.Done():
	}
	s.mu.Lock()
	s.stopping = true
	s.mu.Unlock()
	s.cancel()
	s.wg.Wait()
	return nil
}

func (s *Scheduler) loop(e *entry) {
	defer s.wg.Done()
	if e.cfg.startup {
		s.tick(e)
	}
	if e.cfg.interval <= 0 {
		return
	}
	t := time.NewTicker(e.cfg.interval)
	defer t.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-t.C:
			s.tick(e)
		}
	}
}

func (s *Scheduler) tick(e *entry) {
	s.mu.Lock()
	paused := s.pausedReason(e, "") != ""
	s.mu.Unlock()
	if paused {
		return
	}

	inputs := []any{e.zero()}
	if e.seed != nil {
		var err error
		if inputs, err = e.seed(s.ctx); err != nil {
			s.log.Warn("task seed failed", "task", e.name, "err", err)
			return
		}
	}

	step := spread(e.cfg.interval, len(inputs))
	for i, in := range inputs {
		if i == 0 || step == 0 {
			s.scheduled(e, in)
			continue
		}
		delay := step * time.Duration(i)
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			t := time.NewTimer(delay)
			defer t.Stop()
			select {
			case <-s.ctx.Done():
			case <-t.C:
				s.scheduled(e, in)
			}
		}()
	}
}

// spread is the delay between consecutive inputs of one tick.
func spread(interval time.Duration, n int) time.Duration {
	if interval <= 0 || n < 2 {
		return 0
	}
	return min(interval/2, maxSpread) / time.Duration(n)
}

func (s *Scheduler) scheduled(e *entry, in any) {
	if err := s.submit(s.ctx, e, in, false, nil); err != nil && !errors.Is(err, ErrBlocked) && !errors.Is(err, ErrStopped) {
		s.log.Warn("task submit failed", "task", e.name, "err", err)
	}
}

// request is the on-demand path behind Queue and Run.
func (s *Scheduler) request(ctx context.Context, key any, in any, waiter chan<- result) error {
	s.mu.Lock()
	e, ok := s.entries[key]
	s.mu.Unlock()
	if !ok {
		return ErrNotRegistered
	}
	return s.submit(ctx, e, in, true, waiter)
}

// submit starts a run, coalesces it into the running one (user only), or skips
// it. User requests ignore pauses and backoff but not conditions.
func (s *Scheduler) submit(ctx context.Context, e *entry, in any, user bool, waiter chan<- result) error {
	k := stateKey{e, subject(in)}

	s.mu.Lock()
	if s.stopping {
		s.mu.Unlock()
		return ErrStopped
	}
	st := s.state(k)
	st.in = in
	if !user {
		if reason := s.skipReason(e, k.subject, st); reason != "" {
			st.lastSkip = reason
			s.mu.Unlock()
			s.changed()
			return nil
		}
	}
	s.mu.Unlock()

	// Conditions may do I/O, so they run outside the lock.
	if v := e.allowed(ctx, in); !v.OK {
		s.mu.Lock()
		st.lastSkip = v.Reason
		s.mu.Unlock()
		s.changed()
		return &BlockedError{Reason: v.Reason}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopping {
		return ErrStopped
	}
	if st.running {
		if !user {
			st.lastSkip = "already running"
			return nil
		}
		if st.next == nil {
			st.next = &pending{user: true}
		}
		st.next.in = in
		st.next.parent = parentProgress(ctx)
		if waiter != nil {
			st.next.waiters = append(st.next.waiters, waiter)
		}
		return nil
	}
	st.lastSkip = ""
	st.running = true
	p := &pending{in: in, user: user, parent: parentProgress(ctx)}
	if waiter != nil {
		p.waiters = append(p.waiters, waiter)
	}
	s.wg.Add(1)
	go s.execute(e, k, st, p)
	s.changed()
	return nil
}

func (s *Scheduler) skipReason(e *entry, subject string, st *state) string {
	if r := s.pausedReason(e, subject); r != "" {
		return r
	}
	if time.Now().Before(st.backoffUntil) {
		return "backing off after failure"
	}
	if st.running {
		return "already running"
	}
	return ""
}

func (s *Scheduler) pausedReason(e *entry, subject string) string {
	if e.pausable && s.paused[Pause{PauseTask, e.name}] {
		return "task paused"
	}
	if subject != "" && s.paused[Pause{PauseSubject, subject}] {
		return "subject paused"
	}
	return ""
}

func (s *Scheduler) state(k stateKey) *state {
	st, ok := s.states[k]
	if !ok {
		st = &state{}
		s.states[k] = st
	}
	return st
}

func (s *Scheduler) execute(e *entry, k stateKey, st *state, p *pending) {
	defer s.wg.Done()

	pl := cmp.Or(e.cfg.pool, s.pool)
	if err := pl.Acquire(s.ctx, p.user); err != nil {
		s.mu.Lock()
		deliver(p.waiters, result{err: ErrStopped})
		if st.next != nil {
			deliver(st.next.waiters, result{err: ErrStopped})
			st.next = nil
		}
		st.running = false
		s.mu.Unlock()
		return
	}

	s.mu.Lock()
	st.executing = true
	s.mu.Unlock()
	s.changed()

	ctx := WithScheduler(s.ctx, s)
	var rep *reporter
	switch {
	case e.cfg.progress != "":
		rep = &reporter{s: s, st: st, key: e.cfg.progress, subject: k.subject}
		ctx = context.WithValue(ctx, progressCtxKey{}, *rep)
		rep.emit(ProgressStart, Progress{}, "")
	case p.parent != nil:
		// A sub-task without its own key reports into the bar of the run that queued it.
		ctx = context.WithValue(ctx, progressCtxKey{}, *p.parent)
	}
	ctx, cancel := context.WithTimeout(ctx, cmp.Or(e.cfg.timeout, DefaultTimeout))
	out, err := safeRun(ctx, e, p.in)
	cancel()
	pl.Release()
	if rep != nil {
		s.mu.Lock()
		last := st.progress
		st.progress, st.progressSent = Progress{}, time.Time{}
		s.mu.Unlock()
		var msg string
		if err != nil {
			msg = err.Error()
		}
		rep.emit(ProgressDone, last, msg)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	st.executing = false
	st.lastRun = now
	if err != nil {
		st.failures++
		st.lastErr = err.Error()
		st.backoffUntil = backoffUntil(now, st.failures, err)
		s.log.Warn("task failed", "task", e.name, "subject", k.subject, "failures", st.failures,
			"retry", st.backoffUntil, "err", err)
	} else {
		st.failures = 0
		st.lastErr = ""
		st.backoffUntil = time.Time{}
	}
	deliver(p.waiters, result{out: out, err: err})

	next := st.next
	st.next = nil
	switch {
	case next != nil && !s.stopping:
		s.wg.Add(1)
		go s.execute(e, k, st, next)
	case next != nil:
		deliver(next.waiters, result{err: ErrStopped})
		st.running = false
	default:
		st.running = false
	}
	s.changed()
}

func deliver(ws []chan<- result, r result) {
	for _, w := range ws {
		w <- r // buffered, one result per waiter
	}
}

// safeRun turns a panicking task into a failed run instead of a dead app.
func safeRun(ctx context.Context, e *entry, in any) (out any, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("task %s panicked: %v", e.name, p)
		}
	}()
	return e.run(ctx, in)
}

func backoffUntil(now time.Time, failures int, err error) time.Time {
	var re *retryError
	if errors.As(err, &re) {
		return re.at
	}
	d := backoffMin
	for i := 1; i < failures && d < backoffMax; i++ {
		d *= 2
	}
	return now.Add(min(d, backoffMax))
}

// Pause stops scheduled runs for a task (by Pausable name) or a subject until
// Resume. On-demand runs still go through.
func (s *Scheduler) Pause(ctx context.Context, p Pause) error  { return s.setPause(ctx, p, true) }
func (s *Scheduler) Resume(ctx context.Context, p Pause) error { return s.setPause(ctx, p, false) }

func (s *Scheduler) setPause(ctx context.Context, p Pause, paused bool) error {
	switch p.Kind {
	case PauseTask:
		s.mu.Lock()
		_, ok := s.names[p.Value]
		s.mu.Unlock()
		if !ok {
			return fmt.Errorf("%w: pausable task %q", ErrNotFound, p.Value)
		}
	case PauseSubject:
		if p.Value == "" {
			return errors.New("task: empty subject")
		}
	default:
		return fmt.Errorf("task: unknown pause kind %q", p.Kind)
	}
	if err := s.pauses.Set(ctx, p, paused); err != nil {
		return err
	}
	s.mu.Lock()
	if paused {
		s.paused[p] = true
	} else {
		delete(s.paused, p)
	}
	s.mu.Unlock()
	s.changed()
	return nil
}

// Pauses lists the active user pauses.
func (s *Scheduler) Pauses() []Pause {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Pause, 0, len(s.paused))
	for p := range s.paused {
		out = append(out, p)
	}
	return out
}

// Trigger runs a Pausable task on demand by name, for the UI. Parameterized
// tasks reuse the last input seen for the subject.
func (s *Scheduler) Trigger(ctx context.Context, name, subject string) error {
	s.mu.Lock()
	e, ok := s.names[name]
	var in any
	if ok {
		if st := s.states[stateKey{e, subject}]; st != nil {
			in = st.in
		} else if subject == "" && e.seed == nil {
			in = e.zero()
		}
	}
	s.mu.Unlock()
	if !ok || in == nil {
		return fmt.Errorf("%w: %s %s", ErrNotFound, name, subject)
	}
	return s.submit(ctx, e, in, true, nil)
}

// Status is one task and subject as the UI sees it. Times are zero when unset.
type Status struct {
	Task         string    `json:"task"`
	Subject      string    `json:"subject"`
	Pausable     bool      `json:"pausable"`
	Paused       bool      `json:"paused"`
	Queued       bool      `json:"queued"`
	Running      bool      `json:"running"`
	LastRun      time.Time `json:"lastRun"`
	LastError    string    `json:"lastError"`
	LastSkip     string    `json:"lastSkip"`
	Failures     int       `json:"failures"`
	BackoffUntil time.Time `json:"backoffUntil"`
}

// List returns the status of every task and subject seen this session, plus
// unparameterized tasks that have not run yet.
func (s *Scheduler) List() []Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Status
	seen := map[*entry]bool{}
	for k, st := range s.states {
		seen[k.e] = true
		out = append(out, Status{
			Task:         k.e.name,
			Subject:      k.subject,
			Pausable:     k.e.pausable,
			Paused:       s.pausedReason(k.e, k.subject) != "",
			Queued:       st.running && !st.executing,
			Running:      st.executing,
			LastRun:      st.lastRun,
			LastError:    st.lastErr,
			LastSkip:     st.lastSkip,
			Failures:     st.failures,
			BackoffUntil: st.backoffUntil,
		})
	}
	for _, e := range s.entries {
		if !seen[e] && e.seed == nil {
			out = append(out, Status{Task: e.name, Pausable: e.pausable, Paused: s.pausedReason(e, "") != ""})
		}
	}
	slices.SortFunc(out, func(a, b Status) int {
		return cmp.Or(cmp.Compare(a.Task, b.Task), cmp.Compare(a.Subject, b.Subject))
	})
	return out
}

// changed only takes changeMu, so it is safe to call with mu held.
func (s *Scheduler) changed() {
	if s.onChange == nil {
		return
	}
	s.changeMu.Lock()
	defer s.changeMu.Unlock()
	if s.changePending {
		return
	}
	s.changePending = true
	time.AfterFunc(changeDebounce, func() {
		s.changeMu.Lock()
		s.changePending = false
		s.changeMu.Unlock()
		s.onChange()
	})
}
