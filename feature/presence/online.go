package presence

import (
	"context"
	"time"

	esicharacter "github.com/eve-online-tools/lib-esi-go/common/character"
	getonline "github.com/eve-online-tools/lib-esi-go/esi/getcharacterscharacteridonline"

	"github.com/eve-online-tools/yulai/core/db"
	"github.com/eve-online-tools/yulai/core/esi"
	"github.com/eve-online-tools/yulai/core/task"
)

var Online = task.New(
	(*Feature).online,
	task.WithStartup(),
	task.WithInterval(tick),
	task.WithTimeout(timeout),
)

func (f *Feature) online(ctx context.Context, in Input) (PresenceOnline, error) {
	f.mark(in.CharacterID, partOnline)

	resp, err := getonline.Request(
		ctx, f.esi,
		&getonline.Input{
			Character: esicharacter.Identifier(in.CharacterID),
		},
		f.auth(in.CharacterID),
	)
	if err != nil {
		return PresenceOnline{}, err
	}
	if err := esi.ResponseError(resp); err != nil {
		return PresenceOnline{}, err
	}
	data := resp.Data

	prev, err := f.q.GetOnline(ctx, in.CharacterID)
	existed, err := db.Found(err)
	if err != nil {
		return PresenceOnline{}, err
	}
	row, err := f.q.UpsertOnline(ctx, UpsertOnlineParams{
		CharacterID: in.CharacterID,
		Online:      data.Online,
		LastLogin:   data.LastLogin,
		LastLogout:  data.LastLogout,
		Logins:      data.Logins,
		FetchedAt:   time.Now().UTC(),
	})
	if err != nil {
		return PresenceOnline{}, err
	}
	if !existed || prev.Online != row.Online {
		// Location and ship switch pace with the online state.
		f.forget(in.CharacterID, partLocation, partShip)
	}
	if !existed || db.Changed(prev, row) {
		f.events.Emit(EventChanged)
	}
	return row, nil
}
