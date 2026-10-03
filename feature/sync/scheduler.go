// Package sync runs feature jobs per character on the cadence the jobs ask for.
package sync

import (
	"context"
	"database/sql"
	"time"

	"github.com/eve-online-tools/lib-esi-go/middleware/authentication"

	"github.com/eve-online-tools/yulai/core/todo"
	"github.com/eve-online-tools/yulai/feature"
)

const EventJobsChanged = "sync:jobs:changed"

// retryAfter is the backoff for a job that failed for a non-auth reason.
const retryAfter = 5 * time.Minute

// jobTimeout bounds one run. Paginated syncs should chunk work well under this.
const jobTimeout = 2 * time.Minute

// Tokens is what the scheduler needs from identity/token.
type Tokens interface {
	Scopes(ctx context.Context, characterID int64) ([]string, error)
	For(characterID int64) authentication.RefreshableToken
}

// SyncJob is one row of the sync_jobs table. sqlc will generate this.
type SyncJob struct {
	CharacterID int64   `json:"characterId"`
	Job         string  `json:"job"`
	NextRun     *int64  `json:"nextRun"`
	LastRun     *int64  `json:"lastRun"`
	LastError   *string `json:"lastError"`
	State       string  `json:"state"`
}

type Scheduler struct {
	conn     *sql.DB
	features []feature.Feature
	tokens   Tokens
	workers  int
	tick     time.Duration
}

func NewScheduler(conn *sql.DB, features []feature.Feature, tokens Tokens, workers int) *Scheduler {
	return &Scheduler{
		conn:     conn,
		features: features,
		tokens:   tokens,
		workers:  workers,
		tick:     time.Second,
	}
}

// Enroll reconciles a character's job rows with the features its token unlocks.
// Jobs of disabled features are removed, new ones start now, and RunAtStartup jobs
// are made due so stored data gets verified. Call at boot and after every login.
// Succeeds as a no-op until the scheduler is wired, so logins can complete.
func (s *Scheduler) Enroll(ctx context.Context, characterID int64) error {
	return nil
}

// Start will run the scheduling loop until ctx is cancelled: tick, list due jobs,
// run up to workers at once, record outcome/state/errors, wake requested jobs.
func (s *Scheduler) Start(ctx context.Context) {
	<-ctx.Done()
}

// RunNow makes a job due on the next tick.
func (s *Scheduler) RunNow(ctx context.Context, characterID int64, job string) error {
	return todo.ErrNotImplemented
}
