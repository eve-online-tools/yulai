package task

import (
	"context"
	"sync"
)

type PauseKind string

const (
	// PauseTask pauses a task by its Pausable name.
	PauseTask PauseKind = "task"
	// PauseSubject pauses every task for one subject, e.g. "char:123".
	PauseSubject PauseKind = "subject"
)

type Pause struct {
	Kind  PauseKind `json:"kind"`
	Value string    `json:"value"`
}

// PauseStore persists user pauses. The sqlite implementation backs the
// task_pauses table.
type PauseStore interface {
	List(ctx context.Context) ([]Pause, error)
	Set(ctx context.Context, p Pause, paused bool) error
}

// MemoryPauses is a PauseStore that forgets on restart. For tests and the
// skeleton until the database exists.
type MemoryPauses struct {
	mu sync.Mutex
	m  map[Pause]bool
}

func NewMemoryPauses() *MemoryPauses { return &MemoryPauses{m: map[Pause]bool{}} }

func (m *MemoryPauses) List(context.Context) ([]Pause, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Pause, 0, len(m.m))
	for p := range m.m {
		out = append(out, p)
	}
	return out, nil
}

func (m *MemoryPauses) Set(_ context.Context, p Pause, paused bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if paused {
		m.m[p] = true
	} else {
		delete(m.m, p)
	}
	return nil
}
