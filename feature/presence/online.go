package presence

import (
	"context"
	"errors"
	"time"

	esicharacter "github.com/eve-online-tools/lib-esi-go/common/character"
	"github.com/eve-online-tools/lib-esi-go/esi/getcharacterscharacteridonline"

	"github.com/eve-online-tools/yulai/core/esi"
	"github.com/eve-online-tools/yulai/core/task"
)

var Online = task.New(
	(*Feature).online,
	task.WithStartup(),
	task.WithInterval(tick),
	task.WithTimeout(timeout),
)

func (f *Feature) online(ctx context.Context, in Input) (bool, error) {
	f.mark(in.CharacterID, partOnline)
	data, err := esi.Check(getcharacterscharacteridonline.Request(ctx, f.esi,
		&getcharacterscharacteridonline.Input{Character: esicharacter.Identifier(in.CharacterID)},
		f.auth(in.CharacterID)))
	if err != nil {
		return false, err
	}
	if data == nil {
		return false, errors.New("presence: empty online response")
	}
	prev, err := stored(f.q.GetOnline(ctx, in.CharacterID))
	if err != nil {
		return false, err
	}
	if err := f.q.UpsertOnline(ctx, UpsertOnlineParams{
		CharacterID: in.CharacterID,
		Online:      data.Online,
		LastLogin:   data.LastLogin,
		LastLogout:  data.LastLogout,
		Logins:      data.Logins,
		FetchedAt:   time.Now().UTC(),
	}); err != nil {
		return false, err
	}
	if prev == nil || prev.Online != data.Online {
		// Location and ship switch pace with the online state.
		f.forget(in.CharacterID, partLocation, partShip)
	}
	if prev == nil || prev.Online != data.Online || !eq(prev.LastLogin, data.LastLogin) ||
		!eq(prev.LastLogout, data.LastLogout) || !eq(prev.Logins, data.Logins) {
		f.events.Emit(EventChanged)
	}
	return data.Online, nil
}
