// Package freelance keeps the public freelance job listing and each character's
// own jobs and participation. It needs the esi-characters freelance jobs scope.
package freelance

import (
	"context"
	"database/sql"
	"net/http"
	"sync"
	"time"

	"github.com/eve-online-tools/lib-esi-go/middleware/authentication"
	"github.com/eve-online-tools/lib-esi-go/request"

	"github.com/eve-online-tools/yulai/core/db"
	"github.com/eve-online-tools/yulai/core/task"
	"github.com/eve-online-tools/yulai/feature"
)

const Topic = "freelance"

// ProgressKey is the task.WithProgress key of FetchJobs.
const ProgressKey = "freelance:jobs"

const (
	every   = 5 * time.Minute
	timeout = 30 * time.Second
	// crawlTimeout bounds one run of the public crawl. Cursors are saved per page,
	// so a crawl cut short continues on the next tick.
	crawlTimeout = 5 * time.Minute
)

const scope = "esi-characters.read_freelance_jobs.v1"

type Input = feature.CharacterInput

type Freelance struct {
	conn   *sql.DB
	q      *Queries
	esi    *http.Client
	tokens feature.Tokens
	chars  feature.Characters
	events feature.Emitter

	mu sync.Mutex
	// none holds participation ESI answered 404 for, with the job's last_modified
	// then. In memory, so each is asked again once per session or job change.
	none map[ParticipationInput]time.Time
}

func Feature(d feature.Deps) feature.Feature {
	f := newFreelance(d)
	seed := task.WithParameters(feature.ScopeSeed(d.Characters, scope))
	return feature.Feature{
		ID:     "freelance",
		Name:   "Freelance Jobs",
		Scopes: []string{scope},
		Tasks: []task.Binding{
			FetchJobs.Bind(f, task.WithParameters(f.listingSeed)),
			FetchCharacterJobs.Bind(f, seed),
			FetchParticipation.Bind(f),
		},
	}
}

func newFreelance(d feature.Deps) *Freelance {
	return &Freelance{
		conn:   d.DB,
		q:      New(d.DB),
		esi:    d.ESI,
		tokens: d.Tokens,
		chars:  d.Characters,
		events: d.Events,
		none:   map[ParticipationInput]time.Time{},
	}
}

// listingSeed crawls the public listing only while a character has the feature,
// though the listing itself needs no token.
func (f *Freelance) listingSeed(ctx context.Context) ([]ListingInput, error) {
	ids, err := f.chars.WithScopes(ctx, scope)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	return []ListingInput{{}}, nil
}

func (f *Freelance) auth(characterID int64) request.RequestOption {
	return authentication.WithToken(f.tokens.For(characterID))
}

// inTx runs fn on queries bound to one transaction, committing when it returns nil.
func (f *Freelance) inTx(ctx context.Context, fn func(q *Queries) error) error {
	tx, err := f.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(f.q.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit()
}

// listChanged is db.Changed for the rows of a replaced list.
func listChanged[T any](prev, rows []T) bool {
	if len(prev) != len(rows) {
		return true
	}
	for i := range rows {
		if db.Changed(prev[i], rows[i]) {
			return true
		}
	}
	return false
}
