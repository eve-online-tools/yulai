// Package presence tracks whether a character is online, where they are and what
// they fly. It needs the three esi-location scopes.
package presence

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/eve-online-tools/lib-esi-go/middleware/authentication"
	"github.com/eve-online-tools/lib-esi-go/request"
	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/eve-online-tools/yulai/core/task"
	"github.com/eve-online-tools/yulai/feature"
)

const Topic = "presence"

const (
	onlineFast = time.Minute
	onlineSlow = 5 * time.Minute
	// slowAfter is how long a character must be logged out before onlineSlow applies.
	slowAfter    = 15 * time.Minute
	locationFast = 10 * time.Second
	shipFast     = 30 * time.Second
	// offlineEvery paces location and ship while the character is offline.
	offlineEvery = 15 * time.Minute

	// tick is how often the seeds look for due characters. Half of it is slack, so
	// the spread of one tick's inputs does not push a character to the next tick.
	tick    = 10 * time.Second
	slack   = tick / 2
	timeout = 30 * time.Second
)

var scopes = []string{
	"esi-location.read_online.v1",
	"esi-location.read_location.v1",
	"esi-location.read_ship_type.v1",
}

type Input = feature.CharacterInput

type part int

const (
	partOnline part = iota
	partLocation
	partShip
)

type Presence struct {
	q      *Queries
	esi    *http.Client
	tokens feature.Tokens
	chars  feature.Characters
	events feature.Emitter

	mu sync.Mutex
	// fetched is when each part was last attempted. In memory, so everything is
	// due at startup.
	fetched map[fetchKey]time.Time
}

type fetchKey struct {
	characterID int64
	part        part
}

func Feature(d feature.Deps) feature.Feature {
	f := newPresence(d)
	return feature.Feature{
		ID:     "presence",
		Name:   "Presence",
		Scopes: scopes,
		Tasks: []task.Binding{
			Online.Bind(f, task.WithParameters(f.due(partOnline))),
			Location.Bind(f, task.WithParameters(f.due(partLocation))),
			Ship.Bind(f, task.WithParameters(f.due(partShip))),
		},
		Services: []application.Service{
			application.NewService(&Service{q: f.q}),
		},
	}
}

func newPresence(d feature.Deps) *Presence {
	return &Presence{
		q:       New(d.DB),
		esi:     d.ESI,
		tokens:  d.Tokens,
		chars:   d.Characters,
		events:  d.Events,
		fetched: map[fetchKey]time.Time{},
	}
}

// due seeds a task with the characters whose part is older than its interval.
func (f *Presence) due(p part) task.Seed[Input] {
	return func(ctx context.Context) ([]Input, error) {
		ids, err := f.chars.WithScopes(ctx, scopes...)
		if err != nil || len(ids) == 0 {
			return nil, err
		}
		rows, err := f.q.ListOnline(ctx)
		if err != nil {
			return nil, err
		}
		stored := make(map[int64]*PresenceOnline, len(rows))
		for i := range rows {
			stored[rows[i].CharacterID] = &rows[i]
		}

		now := time.Now()
		f.mu.Lock()
		defer f.mu.Unlock()
		var out []Input
		for _, id := range ids {
			last, ok := f.fetched[fetchKey{id, p}]
			if !ok || now.Sub(last) >= interval(p, stored[id], now)-slack {
				out = append(out, Input{CharacterID: id})
			}
		}
		return out, nil
	}
}

// interval is how often a part is fetched given the stored online state, nil if none.
func interval(p part, r *PresenceOnline, now time.Time) time.Duration {
	online := r != nil && r.Online
	switch p {
	case partOnline:
		if r == nil || online || !loggedOutFor(r, now, slowAfter) {
			return onlineFast
		}
		return onlineSlow
	case partLocation:
		if online {
			return locationFast
		}
	case partShip:
		if online {
			return shipFast
		}
	}
	return offlineEvery
}

// loggedOutFor treats an unknown logout time as long ago.
func loggedOutFor(r *PresenceOnline, now time.Time, d time.Duration) bool {
	return r.LastLogout == nil || now.Sub(*r.LastLogout) >= d
}

func (f *Presence) mark(characterID int64, p part) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fetched[fetchKey{characterID, p}] = time.Now()
}

// forget makes parts due on the next tick.
func (f *Presence) forget(characterID int64, ps ...part) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range ps {
		delete(f.fetched, fetchKey{characterID, p})
	}
}

func (f *Presence) auth(characterID int64) request.RequestOption {
	return authentication.WithToken(f.tokens.For(characterID))
}
