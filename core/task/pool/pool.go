// Package pool limits how many task runs execute at once. Waiters with priority
// are served before the rest.
package pool

import (
	"context"
	"fmt"
	"slices"
	"sync"
)

type Pool struct {
	mu   sync.Mutex
	free int
	high []chan struct{}
	low  []chan struct{}
}

// Create returns a pool with n slots. Share a pool by sharing the pointer.
func Create(n int) *Pool {
	if n < 1 {
		panic(fmt.Sprintf("pool: size must be at least 1, got %d", n))
	}
	return &Pool{free: n}
}

// Acquire blocks until a slot is free or ctx ends. Every nil return must be paired
// with a Release.
func (p *Pool) Acquire(ctx context.Context, priority bool) error {
	p.mu.Lock()
	if p.free > 0 && len(p.high) == 0 && len(p.low) == 0 {
		p.free--
		p.mu.Unlock()
		return nil
	}
	ch := make(chan struct{})
	if priority {
		p.high = append(p.high, ch)
	} else {
		p.low = append(p.low, ch)
	}
	p.mu.Unlock()

	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		p.mu.Lock()
		removed := p.remove(ch)
		p.mu.Unlock()
		if !removed {
			// Granted between ctx ending and taking the lock.
			p.Release()
		}
		return ctx.Err()
	}
}

func (p *Pool) Release() {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch {
	case len(p.high) > 0:
		close(p.high[0])
		p.high = p.high[1:]
	case len(p.low) > 0:
		close(p.low[0])
		p.low = p.low[1:]
	default:
		p.free++
	}
}

func (p *Pool) remove(ch chan struct{}) bool {
	if i := slices.Index(p.high, ch); i >= 0 {
		p.high = slices.Delete(p.high, i, i+1)
		return true
	}
	if i := slices.Index(p.low, ch); i >= 0 {
		p.low = slices.Delete(p.low, i, i+1)
		return true
	}
	return false
}
