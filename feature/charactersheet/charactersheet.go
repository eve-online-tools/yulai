// Package charactersheet keeps public details of our characters, their corporations
// and those corporations' alliances. It needs no scopes, so it is always on and not
// listed as an opt-in feature.
package charactersheet

import (
	"context"
	"database/sql"
	"net/http"
	"sync"
	"time"

	"github.com/eve-online-tools/yulai/core/task"
)

const EventChanged = "charactersheet:changed"

const (
	characterEvery   = time.Hour
	corporationEvery = time.Hour
	allianceEvery    = 24 * time.Hour
	// slack keeps a fetch from the previous character tick from blocking this one.
	slack   = 5 * time.Minute
	timeout = 30 * time.Second
)

// Characters is implemented by feature/character.
type Characters interface {
	IDs(ctx context.Context) ([]int64, error)
	SetAffiliation(ctx context.Context, characterID, corporationID int64, allianceID *int64) error
}

type Emitter interface {
	Emit(name string)
}

type Sheet struct {
	q      *Queries
	esi    *http.Client
	chars  Characters
	events Emitter

	mu sync.Mutex
	// fetched dedupes fan-out from characters sharing a corporation or alliance.
	// In memory, so everything is fetched at startup.
	fetched map[string]time.Time
}

func NewSheet(conn *sql.DB, esiClient *http.Client, chars Characters, events Emitter) *Sheet {
	return &Sheet{
		q:       New(conn),
		esi:     esiClient,
		chars:   chars,
		events:  events,
		fetched: map[string]time.Time{},
	}
}

func (s *Sheet) Tasks() []task.Binding {
	return []task.Binding{
		FetchCharacter.Bind(s, task.WithParameters(s.characters)),
		FetchCorporation.Bind(s),
		FetchAlliance.Bind(s),
	}
}

// claim reports whether subject is due and, if so, marks it fetched now.
func (s *Sheet) claim(subject string, every time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if last, ok := s.fetched[subject]; ok && time.Since(last) < every-slack {
		return false
	}
	s.fetched[subject] = time.Now()
	return true
}

// release lets the next fan-out retry a failed fetch.
func (s *Sheet) release(subject string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.fetched, subject)
}
