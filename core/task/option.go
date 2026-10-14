package task

import (
	"context"
	"time"
)

const DefaultTimeout = 2 * time.Minute

// Seed returns the inputs of a scheduled tick.
type Seed[I any] func(ctx context.Context) ([]I, error)

type Option func(*config)

type config struct {
	progress string
	interval time.Duration
	startup  bool
	timeout  time.Duration
	// Typed as Seed[I]. Checked against the task's input type when applied, since
	// options are not generic.
	seed any
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

// WithProgress lets the task report progress under key. Its runs emit start,
// update and done events through Options.OnProgress. Keys are unique per scheduler.
func WithProgress(key string) Option {
	return func(c *config) { c.progress = key }
}
