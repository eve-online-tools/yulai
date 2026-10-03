package task

import (
	"context"
	"time"
)

// progressEvery throttles update events per run. Start and done always go out.
const progressEvery = 100 * time.Millisecond

// Progress of a running task, for a progress bar.
type Progress struct {
	Phase string `json:"phase"`
	Item  string `json:"item"`
	Done  int64  `json:"done"`
	// Total is 0 when unknown.
	Total int64 `json:"total"`
}

const (
	ProgressStart  = "start"
	ProgressUpdate = "update"
	ProgressDone   = "done"
)

// ProgressEvent is what Options.OnProgress receives for tasks with WithProgress.
type ProgressEvent struct {
	Key     string `json:"key"`
	Subject string `json:"subject"`
	// State is ProgressStart, ProgressUpdate or ProgressDone.
	State    string   `json:"state"`
	Progress Progress `json:"progress"`
	// Error is the run's error, on done only.
	Error string `json:"error"`
}

type progressCtxKey struct{}

type reporter struct {
	s       *Scheduler
	st      *state
	key     string
	subject string
}

// Report sets the progress of the run ctx belongs to. It does nothing outside a
// run of a task with WithProgress, or of a task that one queued without a key of
// its own: such sub-tasks report into the queuing run's progress.
func Report(ctx context.Context, p Progress) {
	r, ok := ctx.Value(progressCtxKey{}).(reporter)
	if !ok {
		return
	}
	now := time.Now()
	r.s.mu.Lock()
	r.st.progress = p
	emit := now.Sub(r.st.progressSent) >= progressEvery
	if emit {
		r.st.progressSent = now
	}
	r.s.mu.Unlock()
	if emit {
		r.emit(ProgressUpdate, p, "")
	}
}

func parentProgress(ctx context.Context) *reporter {
	if r, ok := ctx.Value(progressCtxKey{}).(reporter); ok {
		return &r
	}
	return nil
}

func (r reporter) emit(state string, p Progress, errMsg string) {
	if r.s.onProgress == nil {
		return
	}
	r.s.onProgress(ProgressEvent{
		Key:      r.key,
		Subject:  r.subject,
		State:    state,
		Progress: p,
		Error:    errMsg,
	})
}

// Progress returns the progress of the running task with the WithProgress key
// and subject. ok is false when it is not running.
func (s *Scheduler) Progress(key, subject string) (p Progress, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.progress[key]
	if e == nil {
		return Progress{}, false
	}
	st := s.states[stateKey{e, subject}]
	if st == nil || !st.executing {
		return Progress{}, false
	}
	return st.progress, true
}
