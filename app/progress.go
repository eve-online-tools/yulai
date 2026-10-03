package app

import "github.com/eve-online-tools/yulai/core/task"

// Progress events carry a task.ProgressEvent for tasks with task.WithProgress.
const (
	EventProgressStart  = "progress:start"
	EventProgressUpdate = "progress:update"
	EventProgressDone   = "progress:done"
)

var progressEvents = map[string]string{
	task.ProgressStart:  EventProgressStart,
	task.ProgressUpdate: EventProgressUpdate,
	task.ProgressDone:   EventProgressDone,
}

// ProgressService gives a UI that opens mid-run the current progress of a key.
// After that it follows the progress events.
type ProgressService struct {
	s *task.Scheduler
}

func (p *ProgressService) ServiceName() string { return "ProgressService" }

// Get returns the progress of the running task with the WithProgress key, nil
// when it is not running.
func (p *ProgressService) Get(key string) *task.Progress {
	pr, ok := p.s.Progress(key, "")
	if !ok {
		return nil
	}
	return &pr
}
