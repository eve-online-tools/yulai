# Scheduler plan

Replaces the asset-manager style per-character jobs in a `sync_jobs` table.

## Requirements

- Not ESI-specific: ESI syncs, SDE updates, self-update checks.
- Scheduled runs (interval, startup) and on-demand runs. On-demand runs can be awaited.
- Parameterized: a task runs once per input its parameter seed returns (e.g. every character with a scope).
- Conditional: gates (network, ESI error limit) and per-input checks skip a run. The next tick retries, so
  conditions that become true again (e.g. re-consent) need no explicit resume.
- User pause per character and per task, persisted across restarts.

## Model

- A **task** is a package-level value: a method expression plus options. It holds no state.
- A **binding** attaches the method's receiver, which carries the dependencies, during wiring.
  Several tasks can share one receiver type and one receiver value.
- The **scheduler** owns the bindings, timers, pools, pauses and run status. `task.Default` is the global
  one. Tests bring their own.
- A **tick** (interval or startup) calls the task's parameter seed and runs the task once per input.
  There are no persistent instances.
- **Fan-out** is chaining: a runner queues another task from inside `Run`.

Nothing about scheduling is stored except user pauses (`task_pauses`). Run status and errors are kept in
memory and logged with slog. After a restart, startup tasks run and interval tasks start their timers again.
The lib-esi-go disk cache decides real freshness: a 1 minute interval on an endpoint cached for an hour
costs one request per hour. Tasks skip writes when the response came from cache (`esi.Cached(resp)`).

## API (`core/task`)

```go
// Optional on an input. Gives per-input dedupe, backoff and subject pauses.
// Conventionally "kind:id", e.g. "char:123". Without it all runs of a task share one key.
type Subjecter interface {
	Subject() string
}

type Verdict struct {
	OK     bool
	Reason string // shown in the UI and returned in *BlockedError
}

func Allow() Verdict
func Deny(reason string) Verdict

// Input-independent condition, used with When. Gates implement it.
type Condition interface {
	Check(ctx context.Context) Verdict
}

type Seed[I any] func(ctx context.Context) ([]I, error)

// run is a method expression, e.g. (*Wallet).balance. R, I and O are inferred from it.
func New[R, I, O any](run func(R, context.Context, I) (O, error), opts ...Option) *Task[R, I, O]

func (t *Task[R, I, O]) Bind(r R, opts ...Option) Binding         // r is the receiver; runtime-dependent options allowed
func (t *Task[R, I, O]) Queue(ctx context.Context, in I) error      // fire and forget
func (t *Task[R, I, O]) Run(ctx context.Context, in I) (O, error)   // queue and wait
func (t *Task[R, I, O]) On(s *Scheduler) Handle[I, O]               // explicit scheduler

func WithScheduler(ctx context.Context, s *Scheduler) context.Context

func NewScheduler(o Options) *Scheduler // Workers, Pauses (PauseStore), Log, OnChange
func (s *Scheduler) Register(bs ...Binding)
func (s *Scheduler) Start(ctx context.Context) error // blocks until ctx ends, then waits for runs
func (s *Scheduler) Pause(ctx context.Context, p Pause) error
func (s *Scheduler) Resume(ctx context.Context, p Pause) error
func (s *Scheduler) Trigger(ctx context.Context, name, subject string) error
func (s *Scheduler) List() []Status
func (s *Scheduler) Progress(key, subject string) (Progress, bool) // while a WithProgress task runs
func (s *Scheduler) Refresh(subject string) // run every seed now, dispatch that subject's inputs

// Inside a run of a WithProgress task, or a sub-task it queued: sets its progress. No-op elsewhere.
func Report(ctx context.Context, p Progress) // Phase, Item, Done, Total (0: unknown)
```

Go only allows a variadic parameter last, so the method comes first: `task.New((*Wallet).balance, opts...)`.

### Options

| Option | Meaning |
|---|---|
| `WithParameters(seed)` | Inputs for scheduled ticks. Requires `WithInterval` and/or `WithStartup` |
| `WithInterval(d)` | Tick every `d`. Inputs of one tick are spread evenly over `min(d/2, 30s)` |
| `WithStartup()` | Tick once when the scheduler starts |
| `WithTimeout(d)` | Bound one run. Default 2 minutes |
| `WithPool(p)` | Run in a shared concurrency pool (`pool.Create(n)`). Default: the scheduler's pool |
| `When(cond...)` | Conditions checked before every run, e.g. gates |
| `WithCheck(fn)` | Per-input check with the receiver's dependencies, `func(R, context.Context, I) Verdict` |
| `Pausable(name)` | Stable name: user can pause the task and the UI can trigger it by name |
| `WithProgress(key)` | Runs report progress under `key` (unique per scheduler), see Progress below |

Validation happens in `New` and panics, so it fails on process or test start:

- `WithParameters` without `WithInterval` or `WithStartup`.
- A seed whose element type is not `I`, or a `WithCheck` whose receiver or input type does not match.
- A duplicate `Pausable` name, or the same task registered twice (checked at `Register`).

Without `WithInterval` and `WithStartup` a task is on-demand only. Without `WithParameters` a scheduled tick
runs once with the zero input.

Tasks without `Pausable` are anonymous. Logs and the status list name them after the method
(`feature/wallet.(*Wallet).journal`). Subject pauses still apply to them.

### Gates

```go
func NewGate(reason string) *Gate
func (g *Gate) Close(until time.Time) // zero: closed until Open
func (g *Gate) Open()
```

A gate is a `Condition` with no dependencies. Whoever observes the state owns the gate: `core/esi` declares
`esi.ErrorLimit` (closed on 420 until the reset) and `esi.Offline` (closed on dial errors, opened on the next
success).

## Engine behaviour

1. **Ticks.** One timer per scheduled task. On a tick: skip if the task is paused, call the seed, then
   dispatch each input. A seed error is logged and the tick is skipped.
2. **Per input**, in order. Skip if:
   1. its subject is paused,
   2. it is in backoff,
   3. the same key is already running or queued,
   4. a `When` condition or `WithCheck` fails. The reason is recorded.
3. **Concurrency.** Each run takes a slot from its pool. Pools are shared by passing the same `*Pool`.
4. **Errors.** Logged. The key goes into backoff, doubling from 30s to 30m, in memory.
   `task.RetryAt(err, t)` overrides the backoff, e.g. for ESI `Retry-After`. Panics become errors.
   A timeout cancels the run context.
5. **On demand.** `Queue` and `Run` go ahead of scheduled work and ignore pauses (an explicit request runs once,
   the pause stays). They do not ignore conditions: a failing condition returns `ErrBlocked{Reason}` straight
   away. User requests also bypass backoff. If the key is already running, `Queue` coalesces into one
   follow-up run. `Run` waits for that follow-up and gets its result. A cancelled ctx stops the wait, not the run.
   `Trigger(name, subject)` is the by-name form for the UI and reuses the last input seen for the subject.
6. **Refresh.** `Refresh(subject)` runs every seed now and dispatches only that subject's inputs, as a tick
   would. `app` calls it after a login so a new character does not wait for the next interval.
7. **Scheduler resolution** for `Queue` and `Run`: `WithScheduler` on ctx, then `task.Default`.
   Code under test that queues internally therefore reaches the test scheduler.
8. **Pauses.** `Pause(kind, value)` / `Resume(kind, value)` with kind `task` (a `Pausable` name) or `subject`.
   Persisted through a `PauseStore` (`MemoryPauses` until the sqlite store backs `task_pauses`), loaded at
   `Start`, checked at dispatch.
9. **Status.** `List()` returns, per task and key: running, last run, last error, last skip reason, backoff
   until, paused, queued. Changes call `Options.OnChange` (debounced 250ms); `app` turns that into the
   `data:changed` event with topic `tasks`, so `core/task` does not import Wails.
10. **Progress.** Opt-in with `WithProgress(key)`. A run emits `ProgressStart` when it starts executing,
   `ProgressUpdate` on `Report` (at most every 100ms) and `ProgressDone` (with the last progress and the error)
   when it ends, through `Options.OnProgress`, so a UI shows a bar on start and hides it on done.
   `Report` finds the run through ctx; a sub-task queued with that ctx and no key of its own reports into the
   same run, without start or done of its own. `Progress(key, subject)` gives the current value to a UI that
   opens mid-run.
11. **Stop.** When the `Start` ctx ends, run contexts are cancelled, waiting `Run` callers get `ErrStopped` or
   the cancellation error, and `Start` returns once every run has.

## Layout

```
core/task/               engine, gates, pause store; owns task_pauses (the one table in core)
core/task/pool/          concurrency pools with priority for on-demand runs
core/esi/                Pool, Timeout, ErrorLimit and Offline gates, Cached(resp)
feature/input.go         CharacterInput, CharacterSeed, ScopeSeed
app/tasks.go             Wails TaskService: List, Progress
feature/<name>/
  <name>.go              Feature(d feature.Deps) feature.Feature, the receiver type, shared state and seeds
  <endpoint>.go          one task var and run method per ESI endpoint
  queries.sql, sqlc_*    the feature's queries and sqlc output
```

Features hand their bindings to `app` in `feature.Feature.Tasks`.

### Seeds that need dependencies

Seeds read live state, so they are attached at `Bind` with `task.WithParameters`, where the feature has its
`feature.Deps`. Per-character tasks take `feature.CharacterInput` (subject `char:<id>`), usually as
`type Input = feature.CharacterInput`, and seed with:

- `feature.ScopeSeed(d.Characters, scopes...)`: characters with a working token that covers the scopes. Dead
  tokens and missing consent drop out on the next tick and come back after a login.
- `feature.CharacterSeed(list)`: any `func(ctx) ([]int64, error)`, e.g. every character whatever its token.

Dead refresh tokens are handled by the token store, not the scheduler: on `sso.ErrInvalidGrant` it calls the
hook `app.New` wires to `character.MarkNeedsLogin`.

## Examples

Scheduled per character, a hypothetical `feature/wallet/wallet.go`:

```go
package wallet

const scope = "esi-wallet.read_character_wallet.v1"

type Input = feature.CharacterInput

type Wallet struct {
	q      *Queries
	esi    *http.Client
	tokens feature.Tokens
}

var Balance = task.New((*Wallet).balance,
	task.WithStartup(),
	task.WithInterval(time.Minute),
	task.Pausable("wallet.balance"),
)

var Journal = task.New((*Wallet).journal,
	task.WithInterval(time.Minute),
)

func (w *Wallet) balance(ctx context.Context, in Input) (*WalletBalance, error) {
	// ESI request per docs/CONVENTIONS.md, then upsert and return the row.
}

func (w *Wallet) journal(ctx context.Context, in Input) (int, error) {
	// returns the number of new entries; chains when something changed
	if added > 0 {
		_ = Balance.Queue(ctx, in)
	}
	return added, nil
}
```

On demand with a per-input check, `feature/universe/tasks/tasks.go`:

```go
type StructureInput struct {
	ID  int64
	Via int64 // character whose token is borrowed
}

func (in StructureInput) Subject() string { return fmt.Sprintf("structure:%d", in.ID) }

type Universe struct {
	DB     *db.Queries
	ESI    *esi.Client
	Tokens *token.Store
}

var Structure = task.New((*Universe).Structure,
	task.WithCheck((*Universe).CanReadStructure),
	task.WithTimeout(esi.Timeout),
	task.WithPool(esi.Pool),
	task.When(esi.ErrorLimit, esi.Offline),
)

func (u *Universe) CanReadStructure(ctx context.Context, in StructureInput) task.Verdict {
	return u.Tokens.Require(ctx, in.Via, "esi-universe.read_structures.v1")
}

func (u *Universe) Structure(ctx context.Context, in StructureInput) (db.Structure, error) { ... }

// Caller anywhere, e.g. fanning out from an assets task:
s, err := tasks.Structure.Run(ctx, tasks.StructureInput{ID: 1035466617946, Via: charID})
```

Global, not ESI, `core/sde` (see `docs/SDE.md`):

```go
var Check = task.New((*Updater).Check,
	task.WithStartup(),
	task.WithInterval(time.Hour),
	task.Pausable("sde.check"),
)

var Update = task.New((*Updater).Update,
	task.WithTimeout(30*time.Minute),
)

func (u *Updater) Check(ctx context.Context, _ struct{}) (*Build, error) {
	latest, err := u.latest(ctx)
	if err != nil {
		return nil, err
	}
	if !u.store.Outdated(latest) {
		return latest, nil
	}
	return latest, Update.Queue(ctx, *latest)
}
```

Binding, `feature/wallet/wallet.go`:

```go
func Feature(d feature.Deps) feature.Feature {
	w := &Wallet{q: New(d.DB), esi: d.ESI, tokens: d.Tokens}
	seed := task.WithParameters(feature.ScopeSeed(d.Characters, scope))
	return feature.Feature{
		ID:     "wallet",
		Name:   "Wallet",
		Scopes: []string{scope},
		Tasks: []task.Binding{
			Balance.Bind(w, seed),
			Journal.Bind(w, seed),
		},
	}
}
```

Wiring, `app/app.go`:

```go
task.Default = task.NewScheduler(task.Options{Workers: 8, Pauses: sqlitePauses, Log: slog.Default(), OnChange: syncSvc.Emit})
for _, f := range features {
	task.Default.Register(f.Tasks...)
}
// in Start: go task.Default.Start(ctx)
```

Test. `testing/synctest` provides virtual time, so intervals, spread and backoff need no injectable clock:

```go
func TestBalance(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		s := task.NewScheduler(task.Options{})
		s.Register(Balance.Bind(&Wallet{q: New(testdb(t)), esi: fakeESI(t), tokens: fakeTokens{}}))

		b, err := Balance.On(s).Run(t.Context(), Input{CharacterID: 123})
		// or: ctx := task.WithScheduler(t.Context(), s); svc.Refresh(ctx, 123); synctest.Wait()
	})
}
```

## Deferred

- Per-run scheduling output (next run from cache expiry, wake other tasks). Interval plus cache covers it for now.
- Compile-time checks for seed and `WithCheck` types (chained methods on `Task` would give them).
- `task.Func` for tasks without dependencies (no receiver, no binding).
- Corporation tasks: a seed over corporations with a role-holder token. No engine change expected.

## Steps

1. Done: `core/task` and `core/task/pool` with tests (synctest).
2. `task_pauses` in the first migration, sqlite `PauseStore` with sqlc queries (needs `core/db`).
3. `core/esi` gates, pool, `Cached`. `identity/token` seed helpers, `Use`, invalid-grant hook.
4. Done: replace `feature/sync` with `TaskService` and wire everything in `app.New`. Then the frontend query key,
   event listener and status per task.
