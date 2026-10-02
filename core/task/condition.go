package task

import (
	"context"
	"sync"
	"time"
)

// Verdict is the answer of a condition or check. Reason is shown in the UI.
type Verdict struct {
	OK     bool
	Reason string
}

func Allow() Verdict             { return Verdict{OK: true} }
func Deny(reason string) Verdict { return Verdict{Reason: reason} }

// Condition applies to every input of a task.
type Condition interface {
	Check(ctx context.Context) Verdict
}

// Gate is a Condition that whoever observes the state opens and closes.
type Gate struct {
	reason string

	mu     sync.Mutex
	closed bool
	until  time.Time
}

func NewGate(reason string) *Gate { return &Gate{reason: reason} }

// Close blocks runs until Open, or until the given time if it is not zero.
func (g *Gate) Close(until time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.closed, g.until = true, until
}

func (g *Gate) Open() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.closed, g.until = false, time.Time{}
}

func (g *Gate) Check(context.Context) Verdict {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.closed || (!g.until.IsZero() && !time.Now().Before(g.until)) {
		return Allow()
	}
	return Deny(g.reason)
}
