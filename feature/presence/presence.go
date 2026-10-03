// Package presence tracks whether a character is online, where they are and what
// they fly. It needs the three esi-location scopes.
package presence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	esicharacter "github.com/eve-online-tools/lib-esi-go/common/character"
	"github.com/eve-online-tools/lib-esi-go/esi/getcharacterscharacteridlocation"
	"github.com/eve-online-tools/lib-esi-go/esi/getcharacterscharacteridonline"
	"github.com/eve-online-tools/lib-esi-go/esi/getcharacterscharacteridship"
	"github.com/eve-online-tools/lib-esi-go/middleware/authentication"

	"github.com/eve-online-tools/yulai/core/esi"
	"github.com/eve-online-tools/yulai/core/task"
	"github.com/eve-online-tools/yulai/feature"
)

const (
	Name         = "Presence"
	EventChanged = "presence:changed"
)

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

type part int

const (
	partOnline part = iota
	partLocation
	partShip
)

// Every task ticks at the same rate; the seeds decide who is due.
var (
	Online = task.New(
		(*Feature).online,
		task.WithStartup(),
		task.WithInterval(tick),
		task.WithTimeout(timeout),
		task.Pausable("presence.online"),
	)

	Location = task.New(
		(*Feature).location,
		task.WithStartup(),
		task.WithInterval(tick),
		task.WithTimeout(timeout),
		task.Pausable("presence.location"),
	)

	Ship = task.New(
		(*Feature).ship,
		task.WithStartup(),
		task.WithInterval(tick),
		task.WithTimeout(timeout),
		task.Pausable("presence.ship"),
	)
)

type Feature struct {
	q      *Queries
	esi    *http.Client
	tokens Tokens
	chars  Characters
	events Emitter

	mu sync.Mutex
	// fetched is when each part was last attempted. In memory, so everything is
	// due at startup.
	fetched map[fetchKey]time.Time
}

type fetchKey struct {
	characterID int64
	part        part
}

func NewFeature(conn *sql.DB, esiClient *http.Client, tokens Tokens, chars Characters, events Emitter) *Feature {
	return &Feature{
		q:       New(conn),
		esi:     esiClient,
		tokens:  tokens,
		chars:   chars,
		events:  events,
		fetched: map[fetchKey]time.Time{},
	}
}

func (f *Feature) Name() string     { return Name }
func (f *Feature) Scopes() []string { return scopes }

func (f *Feature) Tasks() []task.Binding {
	return []task.Binding{
		Online.Bind(f, task.WithParameters(f.due(partOnline))),
		Location.Bind(f, task.WithParameters(f.due(partLocation))),
		Ship.Bind(f, task.WithParameters(f.due(partShip))),
	}
}

// due seeds a task with the characters whose part is older than its interval.
func (f *Feature) due(p part) task.Seed[Input] {
	return func(ctx context.Context) ([]Input, error) {
		ids, err := f.chars.WithFeature(ctx, f)
		if err != nil || len(ids) == 0 {
			return nil, err
		}
		rows, err := f.q.List(ctx)
		if err != nil {
			return nil, err
		}
		stored := make(map[int64]*Presence, len(rows))
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

// interval is how often a part is fetched given the stored presence, nil if none.
func interval(p part, r *Presence, now time.Time) time.Duration {
	online := r != nil && r.Online != 0
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
func loggedOutFor(r *Presence, now time.Time, d time.Duration) bool {
	return r.LastLogout == nil || now.Sub(time.Unix(*r.LastLogout, 0)) >= d
}

func (f *Feature) mark(characterID int64, p part) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fetched[fetchKey{characterID, p}] = time.Now()
}

// forget makes parts due on the next tick.
func (f *Feature) forget(characterID int64, ps ...part) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range ps {
		delete(f.fetched, fetchKey{characterID, p})
	}
}

func (f *Feature) stored(ctx context.Context, characterID int64) (*Presence, error) {
	r, err := f.q.Get(ctx, characterID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (f *Feature) online(ctx context.Context, in Input) (bool, error) {
	f.mark(in.CharacterID, partOnline)
	data, err := esi.Check(getcharacterscharacteridonline.Request(ctx, f.esi,
		&getcharacterscharacteridonline.Input{Character: esicharacter.Identifier(in.CharacterID)},
		authentication.WithToken(f.tokens.For(in.CharacterID))))
	if err != nil {
		return false, err
	}
	if data == nil {
		return false, errors.New("presence: empty online response")
	}
	prev, err := f.stored(ctx, in.CharacterID)
	if err != nil {
		return false, err
	}
	next := Presence{
		Online:     b2i(data.Online),
		LastLogin:  unixPtr(data.LastLogin),
		LastLogout: unixPtr(data.LastLogout),
		Logins:     data.Logins,
	}
	if err := f.q.UpsertOnline(ctx, UpsertOnlineParams{
		CharacterID: in.CharacterID,
		Online:      next.Online,
		LastLogin:   next.LastLogin,
		LastLogout:  next.LastLogout,
		Logins:      next.Logins,
		OnlineAt:    ptr(time.Now().Unix()),
	}); err != nil {
		return false, err
	}
	if prev == nil || prev.Online != next.Online {
		// Location and ship switch pace with the online state.
		f.forget(in.CharacterID, partLocation, partShip)
	}
	if prev == nil || prev.Online != next.Online || !eq(prev.LastLogin, next.LastLogin) ||
		!eq(prev.LastLogout, next.LastLogout) || !eq(prev.Logins, next.Logins) {
		f.events.Emit(EventChanged)
	}
	return data.Online, nil
}

func (f *Feature) location(ctx context.Context, in Input) (struct{}, error) {
	f.mark(in.CharacterID, partLocation)
	data, err := esi.Check(getcharacterscharacteridlocation.Request(ctx, f.esi,
		&getcharacterscharacteridlocation.Input{Character: esicharacter.Identifier(in.CharacterID)},
		authentication.WithToken(f.tokens.For(in.CharacterID))))
	if err != nil {
		return struct{}{}, err
	}
	if data == nil {
		return struct{}{}, errors.New("presence: empty location response")
	}
	prev, err := f.stored(ctx, in.CharacterID)
	if err != nil {
		return struct{}{}, err
	}
	next := Presence{
		SolarSystemID: ptr(int64(data.SolarSystem)),
		StationID:     idPtr(data.Station),
		StructureID:   idPtr(data.Structure),
	}
	if err := f.q.UpsertLocation(ctx, UpsertLocationParams{
		CharacterID:   in.CharacterID,
		SolarSystemID: next.SolarSystemID,
		StationID:     next.StationID,
		StructureID:   next.StructureID,
		LocationAt:    ptr(time.Now().Unix()),
	}); err != nil {
		return struct{}{}, err
	}
	if prev == nil || !eq(prev.SolarSystemID, next.SolarSystemID) || !eq(prev.StationID, next.StationID) ||
		!eq(prev.StructureID, next.StructureID) {
		f.events.Emit(EventChanged)
	}
	return struct{}{}, nil
}

func (f *Feature) ship(ctx context.Context, in Input) (struct{}, error) {
	f.mark(in.CharacterID, partShip)
	data, err := esi.Check(getcharacterscharacteridship.Request(ctx, f.esi,
		&getcharacterscharacteridship.Input{Character: esicharacter.Identifier(in.CharacterID)},
		authentication.WithToken(f.tokens.For(in.CharacterID))))
	if err != nil {
		return struct{}{}, err
	}
	if data == nil {
		return struct{}{}, errors.New("presence: empty ship response")
	}
	prev, err := f.stored(ctx, in.CharacterID)
	if err != nil {
		return struct{}{}, err
	}
	next := Presence{
		ShipTypeID: ptr(int64(data.ShipType)),
		ShipItemID: ptr(int64(data.ShipItem)),
		ShipName:   ptr(data.ShipName),
	}
	if err := f.q.UpsertShip(ctx, UpsertShipParams{
		CharacterID: in.CharacterID,
		ShipTypeID:  next.ShipTypeID,
		ShipItemID:  next.ShipItemID,
		ShipName:    next.ShipName,
		ShipAt:      ptr(time.Now().Unix()),
	}); err != nil {
		return struct{}{}, err
	}
	if prev == nil || !eq(prev.ShipTypeID, next.ShipTypeID) || !eq(prev.ShipItemID, next.ShipItemID) ||
		!eq(prev.ShipName, next.ShipName) {
		f.events.Emit(EventChanged)
	}
	return struct{}{}, nil
}

func ptr[T any](v T) *T { return &v }

func eq[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func b2i(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func unixPtr(t *time.Time) *int64 {
	if t == nil {
		return nil
	}
	return ptr(t.Unix())
}

// idPtr converts an optional lib-esi-go identifier to a plain int64 pointer.
func idPtr[T ~int64](v *T) *int64 {
	if v == nil {
		return nil
	}
	return ptr(int64(*v))
}
