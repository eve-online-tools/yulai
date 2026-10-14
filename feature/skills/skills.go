// Package skills keeps each character's attributes, trained skills and skill queue.
// It needs the esi-skills read scopes.
package skills

import (
	"context"
	"database/sql"
	"fmt"
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

type Input struct {
	CharacterID int64
}

func (in Input) Subject() string { return fmt.Sprintf("char:%d", in.CharacterID) }

type Skills struct {
	conn   *sql.DB
	q      *Queries
	esi    *http.Client
	tokens feature.Tokens
	chars  feature.Characters
	events feature.Emitter
}

func Feature(d feature.Deps) feature.Feature {
	f := newSkills(d)
	return feature.Feature{
		ID:     "skills",
		Name:   "Skill Management",
		Scopes: scopes,
		Tasks: []task.Binding{
			FetchAttributes.Bind(f, task.WithParameters(f.characters)),
			FetchSkills.Bind(f, task.WithParameters(f.characters)),
			FetchQueue.Bind(f, task.WithParameters(f.characters)),
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
		chars:  d.Characters,
		events: d.Events,
	}
}

func (f *Skills) characters(ctx context.Context) ([]Input, error) {
	ids, err := f.chars.WithScopes(ctx, scopes...)
	if err != nil {
		return nil, err
	}
	out := make([]Input, len(ids))
	for i, id := range ids {
		out[i] = Input{CharacterID: id}
	}
	return out, nil
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
