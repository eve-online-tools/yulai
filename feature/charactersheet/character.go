package charactersheet

import (
	"context"
	"time"

	esicharacter "github.com/eve-online-tools/lib-esi-go/common/character"
	getcharacter "github.com/eve-online-tools/lib-esi-go/esi/getcharacterscharacterid"

	"github.com/eve-online-tools/yulai/core/db"
	"github.com/eve-online-tools/yulai/core/esi"
	"github.com/eve-online-tools/yulai/core/task"
	"github.com/eve-online-tools/yulai/feature"
)

// Corporations and alliances are only reached by fan-out from here.
var FetchCharacter = task.New(
	(*Sheet).character,
	task.WithStartup(),
	task.WithInterval(characterEvery),
	task.WithTimeout(timeout),
)

func (s *Sheet) character(ctx context.Context, in feature.CharacterInput) (*CharacterSheet, error) {
	resp, err := getcharacter.Request(
		ctx, s.esi,
		&getcharacter.Input{
			Character: esicharacter.Identifier(in.CharacterID),
		},
	)
	if err != nil {
		return nil, err
	}
	if err := esi.ResponseError(resp); err != nil {
		return nil, err
	}
	data := resp.Data

	prev, err := s.q.GetCharacter(ctx, in.CharacterID)
	existed, err := db.Found(err)
	if err != nil {
		return nil, err
	}
	row, err := s.q.UpsertCharacter(ctx, UpsertCharacterParams{
		CharacterID:    in.CharacterID,
		CorporationID:  data.Corporation,
		AllianceID:     data.Alliance,
		FactionID:      data.Faction,
		Birthday:       data.Birthday,
		BloodlineID:    data.Bloodline,
		RaceID:         data.Race,
		Gender:         data.Gender,
		SecurityStatus: data.SecurityStatus,
		Title:          data.Title,
		Description:    data.Description,
		FetchedAt:      time.Now().UTC(),
	})
	if err != nil {
		return nil, err
	}
	err = s.chars.SetAffiliation(ctx, in.CharacterID, int64(row.CorporationID), (*int64)(row.AllianceID))
	if err != nil {
		return nil, err
	}
	if !existed || db.Changed(prev, row) {
		s.events.Emit(EventChanged)
	}

	corp := CorporationInput{CorporationID: row.CorporationID}
	if s.claim(corp.Subject(), corporationEvery) {
		if err := FetchCorporation.Queue(ctx, corp); err != nil {
			s.release(corp.Subject())
			return nil, err
		}
	}
	return &row, nil
}
