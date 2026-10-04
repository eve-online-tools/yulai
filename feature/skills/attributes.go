package skills

import (
	"context"
	"time"

	esicharacter "github.com/eve-online-tools/lib-esi-go/common/character"
	getattributes "github.com/eve-online-tools/lib-esi-go/esi/getcharacterscharacteridattributes"

	"github.com/eve-online-tools/yulai/core/db"
	"github.com/eve-online-tools/yulai/core/esi"
	"github.com/eve-online-tools/yulai/core/task"
)

var FetchAttributes = task.New(
	(*Feature).attributes,
	task.WithStartup(),
	task.WithInterval(every),
	task.WithTimeout(timeout),
)

func (f *Feature) attributes(ctx context.Context, in Input) (*CharacterAttribute, error) {
	resp, err := getattributes.Request(
		ctx, f.esi,
		&getattributes.Input{
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

	prev, err := f.q.GetAttributes(ctx, in.CharacterID)
	existed, err := db.Found(err)
	if err != nil {
		return nil, err
	}
	row, err := f.q.UpsertAttributes(ctx, UpsertAttributesParams{
		CharacterID:              in.CharacterID,
		Charisma:                 data.Charisma,
		Intelligence:             data.Intelligence,
		Memory:                   data.Memory,
		Perception:               data.Perception,
		Willpower:                data.Willpower,
		BonusRemaps:              data.BonusRemaps,
		LastRemapDate:            data.LastRemapDate,
		AccruedRemapCooldownDate: data.AccruedRemapCooldownDate,
		FetchedAt:                time.Now().UTC(),
	})
	if err != nil {
		return nil, err
	}
	if !existed || db.Changed(prev, row) {
		f.events.Emit(EventChanged)
	}
	return &row, nil
}
