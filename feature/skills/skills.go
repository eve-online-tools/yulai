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

	"github.com/eve-online-tools/yulai/core/db"
	"github.com/eve-online-tools/yulai/core/task"
	"github.com/eve-online-tools/yulai/feature"
)

const (
	Name         = "Skill Management"
	EventChanged = "skills:changed"
)

const (
	every   = 5 * time.Minute
	timeout = 30 * time.Second
)

var scopes = []string{
	"esi-skills.read_skills.v1",
	"esi-skills.read_skillqueue.v1",
}

// Characters lists characters whose token unlocks a feature. Implemented by
// feature/character.
type Characters interface {
	WithFeature(ctx context.Context, f feature.Feature) ([]int64, error)
}

type Tokens interface {
	For(characterID int64) authentication.RefreshableToken
}

type Emitter interface {
	Emit(name string)
}

type Input struct {
	CharacterID int64
}

func (in Input) Subject() string { return fmt.Sprintf("char:%d", in.CharacterID) }

type Feature struct {
	conn   *sql.DB
	q      *Queries
	esi    *http.Client
	tokens Tokens
	chars  Characters
	events Emitter
}

func NewFeature(conn *sql.DB, esiClient *http.Client, tokens Tokens, chars Characters, events Emitter) *Feature {
	return &Feature{
		conn:   conn,
		q:      New(conn),
		esi:    esiClient,
		tokens: tokens,
		chars:  chars,
		events: events,
	}
}

func (f *Feature) Name() string     { return Name }
func (f *Feature) Scopes() []string { return scopes }

func (f *Feature) Tasks() []task.Binding {
	return []task.Binding{
		FetchAttributes.Bind(f, task.WithParameters(f.characters)),
		FetchSkills.Bind(f, task.WithParameters(f.characters)),
		FetchQueue.Bind(f, task.WithParameters(f.characters)),
	}
}

func (f *Feature) characters(ctx context.Context) ([]Input, error) {
	ids, err := f.chars.WithFeature(ctx, f)
	if err != nil {
		return nil, err
	}
	out := make([]Input, len(ids))
	for i, id := range ids {
		out[i] = Input{CharacterID: id}
	}
	return out, nil
}

func (f *Feature) auth(characterID int64) request.RequestOption {
	return authentication.WithToken(f.tokens.For(characterID))
}

// inTx runs fn on queries bound to one transaction, committing when it returns nil.
func (f *Feature) inTx(ctx context.Context, fn func(q *Queries) error) error {
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
