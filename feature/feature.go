// Package feature defines what an opt-in feature is and how its background jobs talk
// to the scheduler. Concrete features live in subpackages.
package feature

import (
	"context"
	"encoding/json"
	"time"

	"github.com/eve-online-tools/lib-esi-go/middleware/authentication"
)

// Feature is a unit of functionality a user consents to through SSO scopes.
type Feature interface {
	Name() string
	// Scopes must all be present on the character's token, or the feature is off.
	Scopes() []string
	Jobs() []Job
}

// Enabled reports whether a token with the given scopes unlocks the feature.
func Enabled(f Feature, scopes []string) bool {
	have := map[string]bool{}
	for _, s := range scopes {
		have[s] = true
	}
	for _, s := range f.Scopes() {
		if !have[s] {
			return false
		}
	}
	return true
}

// Job is one ESI-backed unit of work, run per character.
type Job struct {
	Name string
	// RunAtStartup forces a run when the app boots, so stored data is verified quickly.
	RunAtStartup bool
	Run          func(ctx context.Context, r *Run) (Outcome, error)
}

// Run is what a job gets handed.
type Run struct {
	CharacterID int64
	Token       authentication.RefreshableToken
	// State is this job's persisted scratch for this character. Empty on first run.
	State json.RawMessage
}

// LoadState decodes r.State into v. Empty state is not an error.
func (r *Run) LoadState(v any) error {
	if len(r.State) == 0 {
		return nil
	}
	return json.Unmarshal(r.State, v)
}

// Outcome tells the scheduler what to do next.
type Outcome struct {
	// Next is when to run again. Zero pauses the job until another job wakes it.
	Next time.Time
	// State is persisted for the next run. nil keeps the previous state.
	State any
	// Wake lists other jobs of the same character to run now.
	Wake []string
}
