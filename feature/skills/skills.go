// Package skills keeps each character's attributes, trained skills and skill queue.
// It needs the esi-skills read scopes.
package skills

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/eve-online-tools/lib-esi-go/middleware/authentication"
	"github.com/eve-online-tools/lib-esi-go/request"
	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/eve-online-tools/yulai/core/db"
	"github.com/eve-online-tools/yulai/core/task"
	"github.com/eve-online-tools/yulai/feature"
)

const EventChanged = "skills:changed"

const (
	every   = 5 * time.Minute
	timeout = 30 * time.Second
)

var scopes = []string{
	"esi-skills.read_skills.v1",
	"esi-skills.read_skillqueue.v1",
}

type Input = feature.CharacterInput

type Skills struct {
	conn   *sql.DB
	q      *Queries
	esi    *http.Client
	tokens feature.Tokens
	events feature.Emitter
}

func Feature(d feature.Deps) feature.Feature {
	f := newSkills(d)
	seed := task.WithParameters(feature.ScopeSeed(d.Characters, scopes...))
	return feature.Feature{
		ID:     "skills",
		Name:   "Skill Management",
		Scopes: scopes,
		Tasks: []task.Binding{
			FetchAttributes.Bind(f, seed),
			FetchSkills.Bind(f, seed),
			FetchQueue.Bind(f, seed),
		},
		Services: []application.Service{
			application.NewService(&Service{q: f.q}),
		},
	}
}

func newSkills(d feature.Deps) *Skills {
	return &Skills{
		conn:   d.DB,
		q:      New(d.DB),
		esi:    d.ESI,
		tokens: d.Tokens,
		events: d.Events,
	}
}

func (f *Skills) auth(characterID int64) request.RequestOption {
	return authentication.WithToken(f.tokens.For(characterID))
}

// inTx runs fn on queries bound to one transaction, committing when it returns nil.
func (f *Skills) inTx(ctx context.Context, fn func(q *Queries) error) error {
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
