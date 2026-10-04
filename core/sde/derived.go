package sde

import (
	"context"
	"sync"
)

// Derived is a value computed from the installed SDE, built once per install
// instead of on every read. It is built ahead of the first Get: when created on
// an installed store, and again after every install.
type Derived[T any] struct {
	store *Store
	build func(ctx context.Context, q *Queries) (T, error)

	mu sync.Mutex
	// gen is the store generation val was built from, 0 before the first build.
	gen uint64
	val T
}

func NewDerived[T any](s *Store, build func(ctx context.Context, q *Queries) (T, error)) *Derived[T] {
	d := &Derived[T]{store: s, build: build}
	s.mu.Lock()
	s.derived = append(s.derived, d.warm)
	installed := s.q != nil
	s.mu.Unlock()
	if installed {
		go d.warm()
	}
	return d
}

// Get returns the value for the installed SDE, building it if this install has
// none yet. It returns ErrNotInstalled before the first install.
func (d *Derived[T]) Get(ctx context.Context) (T, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	var out T
	err := d.store.Read(ctx, func(q *Queries) error {
		if d.gen == d.store.gen {
			out = d.val
			return nil
		}
		v, err := d.build(ctx, q)
		if err != nil {
			return err
		}
		d.val, d.gen, out = v, d.store.gen, v
		return nil
	})
	return out, err
}

func (d *Derived[T]) warm() {
	if _, err := d.Get(context.Background()); err != nil && err != ErrNotInstalled {
		d.store.log.Warn("sde: build derived value", "err", err)
	}
}
