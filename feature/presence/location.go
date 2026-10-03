package presence

import (
	"context"
	"errors"
	"time"

	esicharacter "github.com/eve-online-tools/lib-esi-go/common/character"
	"github.com/eve-online-tools/lib-esi-go/esi/getcharacterscharacteridlocation"

	"github.com/eve-online-tools/yulai/core/esi"
	"github.com/eve-online-tools/yulai/core/task"
)

var Location = task.New(
	(*Feature).location,
	task.WithStartup(),
	task.WithInterval(tick),
	task.WithTimeout(timeout),
)

func (f *Feature) location(ctx context.Context, in Input) (struct{}, error) {
	f.mark(in.CharacterID, partLocation)
	data, err := esi.Check(getcharacterscharacteridlocation.Request(ctx, f.esi,
		&getcharacterscharacteridlocation.Input{Character: esicharacter.Identifier(in.CharacterID)},
		f.auth(in.CharacterID)))
	if err != nil {
		return struct{}{}, err
	}
	if data == nil {
		return struct{}{}, errors.New("presence: empty location response")
	}
	prev, err := stored(f.q.GetLocation(ctx, in.CharacterID))
	if err != nil {
		return struct{}{}, err
	}
	if err := f.q.UpsertLocation(ctx, UpsertLocationParams{
		CharacterID:   in.CharacterID,
		SolarSystemID: data.SolarSystem,
		StationID:     data.Station,
		StructureID:   data.Structure,
		FetchedAt:     time.Now().UTC(),
	}); err != nil {
		return struct{}{}, err
	}
	if prev == nil || prev.SolarSystemID != data.SolarSystem || !eq(prev.StationID, data.Station) ||
		!eq(prev.StructureID, data.Structure) {
		f.events.Emit(EventChanged)
	}
	return struct{}{}, nil
}
