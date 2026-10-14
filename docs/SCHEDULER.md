# Scheduler

`core/task` runs background and on-demand work: ESI syncs, SDE checks and updates. Nothing in it knows
about ESI or Wails.

## Model

- A **task** is a package-level value: a method expression plus options. It holds no state.
- A **binding** attaches the method's receiver, which carries the dependencies, during wiring.
  Several tasks can share one receiver type and one receiver value.
- The **scheduler** owns the bindings, timers, the worker pool and run status. `app` creates one and passes
  it where it is needed. Tests bring their own.
- A **tick** (interval or startup) calls the task's seed and runs the task once per input.
  There are no persistent instances.
- **Fan-out** is chaining: a run queues another task from inside itself.

Nothing about scheduling is stored. Run status and errors are kept in memory and logged with slog. After a
restart, startup tasks run and interval tasks start their timers again. The lib-esi-go disk cache decides
real freshness: a 1 minute interval on an endpoint cached for an hour costs one request per hour.

## API

```go
// Optional on an input. Gives per-input dedupe and backoff.
// Conventionally "kind:id", e.g. "char:123". Without it all runs of a task share one key.
type Subjecter interface {
	Subject() string
}

type Seed[I any] func(ctx context.Context) ([]I, error)

// run is a method expression, e.g. (*Wallet).balance. R, I and O are inferred from it.
func New[R, I, O any](run func(R, context.Context, I) (O, error), opts ...Option) *Task[R, I, O]

func (t *Task[R, I, O]) Bind(r R, opts ...Option) Binding          // r is the receiver; runtime options allowed
func (t *Task[R, I, O]) Queue(ctx context.Context, in I) error      // fire and forget, scheduler from ctx
func (t *Task[R, I, O]) Run(ctx context.Context, in I) (O, error)   // queue and wait, scheduler from ctx
func (t *Task[R, I, O]) On(s *Scheduler) Handle[I, O]               // explicit scheduler
func (t *Task[R, I, O]) Name() string                               // derived from the method

func WithScheduler(ctx context.Context, s *Scheduler) context.Context

func NewScheduler(o Options) *Scheduler // Workers, Log, OnChange, OnProgress
func (s *Scheduler) Register(bs ...Binding)
func (s *Scheduler) Start(ctx context.Context) error // blocks until ctx ends, then waits for runs
func (s *Scheduler) Refresh(subject string)          // run every seed now, dispatch that subject's inputs
func (s *Scheduler) List() []Status
func (s *Scheduler) Progress(key, subject string) (Progress, bool) // while a WithProgress task runs

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
| `WithProgress(key)` | Runs report progress under `key` (unique per scheduler), see Progress below |

Validation panics, so mistakes fail on process or test start:

- `WithParameters` without `WithInterval` or `WithStartup`, or a seed whose element type is not `I`.
- A negative interval or timeout.
- The same task registered twice, a duplicate progress key, or `Register` after `Start`.

Without `WithInterval` and `WithStartup` a task is on-demand only. Without `WithParameters` a scheduled tick
runs once with the zero input.

## Engine behaviour

1. **Ticks.** One timer per scheduled task. On a tick: call the seed, then dispatch each input. A seed error
   is logged and the tick is skipped.
2. **Per input**, skip if it is in backoff or the same key is already running. The reason is recorded.
3. **Concurrency.** Each run takes a slot from the scheduler's pool (`Options.Workers`, default 4).
   On-demand runs are served before scheduled ones.
4. **Errors.** Logged. The key goes into backoff, doubling from 30s to 30m, in memory. Panics become errors.
   A timeout cancels the run context.
5. **On demand.** `Queue` and `Run` go ahead of scheduled work and bypass backoff. If the key is already
   running, `Queue` coalesces into one follow-up run. `Run` waits for that follow-up and gets its result. A
   cancelled ctx stops the wait, not the run.
6. **Refresh.** `Refresh(subject)` runs every seed now and dispatches only that subject's inputs, as a tick
   would. `app` calls it after a login so a new character does not wait for the next interval.
7. **Scheduler resolution** for `Queue` and `Run`: the scheduler on ctx (`WithScheduler`), else
   `ErrNoScheduler`. Runs carry theirs, so a task can queue another. Elsewhere, such as a Wails service,
   use `On(s)`.
8. **Status.** `List()` returns, per task and key: queued, running, last run, last error, last skip reason,
   failures and backoff until. Changes call `Options.OnChange` (debounced 250ms); `app` sends that as the
   `tasks` topic of `data:changed`, and `TaskService` serves the list.
9. **Progress.** Opt-in with `WithProgress(key)`. A run emits `ProgressStart` when it starts executing,
   `ProgressUpdate` on `Report` (at most every 100ms) and `ProgressDone` (with the last progress and the error)
   when it ends, through `Options.OnProgress`, so a UI shows a bar on start and hides it on done.
   `Report` finds the run through ctx; a sub-task queued with that ctx and no key of its own reports into the
   same run, without start or done of its own. `Progress(key, subject)` gives the current value to a UI that
   opens mid-run.
10. **Stop.** When the `Start` ctx ends, run contexts are cancelled, waiting `Run` callers get `ErrStopped` or
    the cancellation error, and `Start` returns once every run has.

## Layout

```
core/task/               engine, worker pool, progress
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

## Example

A hypothetical `feature/wallet/wallet.go`:

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

Test. `testing/synctest` provides virtual time, so intervals, spread and backoff need no injectable clock:

```go
func TestBalance(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := task.NewScheduler(task.Options{})
		s.Register(Balance.Bind(&Wallet{q: New(testdb(t)), esi: fakeESI(t), tokens: fakeTokens{}}))

		b, err := Balance.On(s).Run(t.Context(), Input{CharacterID: 123})
		// or: ctx := task.WithScheduler(t.Context(), s); svc.Refresh(ctx, 123); synctest.Wait()
	})
}
```
