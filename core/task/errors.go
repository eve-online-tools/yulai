package task

import (
	"errors"
	"time"
)

var (
	// ErrBlocked matches every *BlockedError.
	ErrBlocked       = errors.New("task: blocked")
	ErrNotRegistered = errors.New("task: not registered with the scheduler")
	ErrNoScheduler   = errors.New("task: no scheduler on context and task.Default is nil")
	ErrNotFound      = errors.New("task: not found")
)

// BlockedError is returned by Queue and Run when a condition or check denies the run.
type BlockedError struct {
	Reason string
}

func (e *BlockedError) Error() string        { return "task: blocked: " + e.Reason }
func (e *BlockedError) Is(target error) bool { return target == ErrBlocked }

type retryError struct {
	err error
	at  time.Time
}

func (e *retryError) Error() string { return e.err.Error() }
func (e *retryError) Unwrap() error { return e.err }

// RetryAt marks a failure that should not be retried before t, overriding the
// exponential backoff. Use it for e.g. ESI Retry-After.
func RetryAt(err error, t time.Time) error {
	if err == nil {
		return nil
	}
	return &retryError{err: err, at: t}
}
