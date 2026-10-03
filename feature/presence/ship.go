package presence

import (
	"context"
	"time"

	esicharacter "github.com/eve-online-tools/lib-esi-go/common/character"
	"github.com/eve-online-tools/lib-esi-go/esi/getcharacterscharacteridship"

	"github.com/eve-online-tools/yulai/core/esi"
	"github.com/eve-online-tools/yulai/core/task"
)

var Ship = task.New(
	(*Feature).ship,
	task.WithStartup(),
	task.WithInterval(tick),
	task.WithTimeout(timeout),
)

func (f *Feature) ship(ctx context.Context, in Input) (PresenceShip, error) {
	f.mark(in.CharacterID, partShip)

	input := &getcharacterscharacteridship.Input{Character: esicharacter.Identifier(in.CharacterID)}
	resp, err := getcharacterscharacteridship.Request(ctx, f.esi, input, f.auth(in.CharacterID))
	if err != nil {
		return PresenceShip{}, err
	}
	if err := esi.ResponseError(resp); err != nil {
		return PresenceShip{}, err
	}
	data := resp.Data

	prev, err := stored(f.q.GetShip(ctx, in.CharacterID))
	if err != nil {
		return PresenceShip{}, err
	}
	row, err := f.q.UpsertShip(ctx, UpsertShipParams{
		CharacterID: in.CharacterID,
		ShipItemID:  data.ShipItem,
		ShipTypeID:  data.ShipType,
		ShipName:    data.ShipName,
		FetchedAt:   time.Now().UTC(),
	})
	if err != nil {
		return PresenceShip{}, err
	}
	if prev == nil || prev.ShipItemID != row.ShipItemID || prev.ShipTypeID != row.ShipTypeID ||
		prev.ShipName != row.ShipName {
		f.events.Emit(EventChanged)
	}
	return row, nil
}
