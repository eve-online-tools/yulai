package presence

import (
	"context"
	"errors"
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

func (f *Feature) ship(ctx context.Context, in Input) (struct{}, error) {
	f.mark(in.CharacterID, partShip)
	data, err := esi.Check(getcharacterscharacteridship.Request(ctx, f.esi,
		&getcharacterscharacteridship.Input{Character: esicharacter.Identifier(in.CharacterID)},
		f.auth(in.CharacterID)))
	if err != nil {
		return struct{}{}, err
	}
	if data == nil {
		return struct{}{}, errors.New("presence: empty ship response")
	}
	prev, err := stored(f.q.GetShip(ctx, in.CharacterID))
	if err != nil {
		return struct{}{}, err
	}
	if err := f.q.UpsertShip(ctx, UpsertShipParams{
		CharacterID: in.CharacterID,
		ShipItemID:  data.ShipItem,
		ShipTypeID:  data.ShipType,
		ShipName:    data.ShipName,
		FetchedAt:   time.Now().UTC(),
	}); err != nil {
		return struct{}{}, err
	}
	if prev == nil || prev.ShipItemID != data.ShipItem || prev.ShipTypeID != data.ShipType ||
		prev.ShipName != data.ShipName {
		f.events.Emit(EventChanged)
	}
	return struct{}{}, nil
}
