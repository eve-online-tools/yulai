package task

import (
	"context"
	"time"

	"github.com/eve-online-tools/yulai/core/task/pool"
)

const DefaultTimeout = 2 * time.Minute

// Seed returns the inputs of a scheduled tick.
type Seed[I any] func(ctx context.Context) ([]I, error)

type Option func(*config)

type config struct {
	name     string
	progress string
	interval time.Duration
	startup  bool
	timeout  time.Duration
	pool     *pool.Pool
	conds    []Condition
	// Typed as Seed[I] and func(R, context.Context, I) Verdict. Checked against
	// the task's types when applied, since options are not generic.
	seed  any
	check any
}

// WithParameters seeds scheduled ticks: the task runs once per returned input.
// Requires WithInterval and/or WithStartup.
func WithParameters[I any](seed Seed[I]) Option {
	return func(c *config) { c.seed = seed }
}

// WithInterval ticks every d.
func WithInterval(d time.Duration) Option {
	return func(c *config) { c.interval = d }
}

// WithStartup ticks once when the scheduler starts.
func WithStartup() Option {
	return func(c *config) { c.startup = true }
}

func WithTimeout(d time.Duration) Option {
	return func(c *config) { c.timeout = d }
}

// WithPool runs the task in a pool shared with other tasks.
func WithPool(p *pool.Pool) Option {
	return func(c *config) { c.pool = p }
}

// When adds conditions checked before every run.
func When(conds ...Condition) Option {
	return func(c *config) { c.conds = append(c.conds, conds...) }
}

// WithCheck adds a per-input check, typically a method expression on the task's
// receiver so it can use the receiver's dependencies.
func WithCheck[R, I any](check func(R, context.Context, I) Verdict) Option {
	return func(c *config) { c.check = check }
}

// Pausable gives the task a stable name, so users can pause it and the UI can
// trigger it. Names must be unique per scheduler.
func Pausable(name string) Option {
	return func(c *config) { c.name = name }
}

// WithProgress lets the task report progress under key. Its runs emit start,
// update and done events through Options.OnProgress. Keys are unique per scheduler.
func WithProgress(key string) Option {
	return func(c *config) { c.progress = key }
}
