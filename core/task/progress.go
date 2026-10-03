package task

import "context"

// Progress of a running task, for a progress bar.
type Progress struct {
	Phase string `json:"phase"`
	Item  string `json:"item"`
	Done  int64  `json:"done"`
	// Total is 0 when unknown.
	Total int64 `json:"total"`
}

type progressKey struct{}

type reporter struct {
	s  *Scheduler
	st *state
}

// Report sets the progress of the run ctx belongs to. It shows in Status until
// the run ends. Outside a run it does nothing.
func Report(ctx context.Context, p Progress) {
	r, ok := ctx.Value(progressKey{}).(reporter)
	if !ok {
		return
	}
	r.s.mu.Lock()
	r.st.progress = &p
	r.s.mu.Unlock()
	r.s.changed()
}
