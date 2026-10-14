package task

import "errors"

var (
	ErrNotRegistered = errors.New("task: not registered with the scheduler")
	ErrNoScheduler   = errors.New("task: no scheduler on context")
	ErrStopped       = errors.New("task: scheduler stopped")
)
