package task

import (
	"context"
	"fmt"
	"slices"
	"sync"
)

// pool limits how many runs execute at once. Waiters with priority, on-demand
// runs, are served before the rest.
type pool struct {
	mu   sync.Mutex
	free int
	high []chan struct{}
	low  []chan struct{}
}

func newPool(n int) *pool {
	if n < 1 {
		panic(fmt.Sprintf("task: pool size must be at least 1, got %d", n))
	}
	return &pool{free: n}
}

// acquire blocks until a slot is free or ctx ends. Every nil return must be paired
// with a release.
func (p *pool) acquire(ctx context.Context, priority bool) error {
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
			p.release()
		}
		return ctx.Err()
	}
}

func (p *pool) release() {
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

func (p *pool) remove(ch chan struct{}) bool {
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
