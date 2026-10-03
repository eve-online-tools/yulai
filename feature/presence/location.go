package presence

import (
	"context"
	"time"

	esicharacter "github.com/eve-online-tools/lib-esi-go/common/character"
	getlocation "github.com/eve-online-tools/lib-esi-go/esi/getcharacterscharacteridlocation"

	"github.com/eve-online-tools/yulai/core/db"
	"github.com/eve-online-tools/yulai/core/esi"
	"github.com/eve-online-tools/yulai/core/task"
)

var Location = task.New(
	(*Feature).location,
	task.WithStartup(),
	task.WithInterval(tick),
	task.WithTimeout(timeout),
)

func (f *Feature) location(ctx context.Context, in Input) (*PresenceLocation, error) {
	f.mark(in.CharacterID, partLocation)

	resp, err := getlocation.Request(
		ctx, f.esi,
		&getlocation.Input{
			Character: esicharacter.Identifier(in.CharacterID),
		},
		f.auth(in.CharacterID),
	)
	if err != nil {
		return nil, err
	}
	if err := esi.ResponseError(resp); err != nil {
		return nil, err
	}
	data := resp.Data

	prev, err := f.q.GetLocation(ctx, in.CharacterID)
	existed, err := db.Found(err)
	if err != nil {
		return nil, err
	}
	row, err := f.q.UpsertLocation(ctx, UpsertLocationParams{
		CharacterID:   in.CharacterID,
		SolarSystemID: data.SolarSystem,
		StationID:     data.Station,
		StructureID:   data.Structure,
		FetchedAt:     time.Now().UTC(),
	})
	if err != nil {
		return nil, err
	}
	if !existed || db.Changed(prev, row) {
		f.events.Emit(EventChanged)
	}
	return &row, nil
}
