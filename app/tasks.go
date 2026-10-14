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

// TaskService reports what the scheduler is doing. The UI refetches List on
// EventTasksChanged and follows progress through the progress events.
type TaskService struct {
	s *task.Scheduler
}

func (t *TaskService) ServiceName() string { return "TaskService" }

// List returns the status of every task and subject seen this session.
func (t *TaskService) List() []task.Status {
	out := t.s.List()
	if out == nil {
		out = []task.Status{}
	}
	return out
}

// Progress returns the progress of the running task with the WithProgress key, nil
// when it is not running. A UI that opens mid-run reads it once, then follows the events.
func (t *TaskService) Progress(key string) *task.Progress {
	pr, ok := t.s.Progress(key, "")
	if !ok {
		return nil
	}
	return &pr
}
