package charactersheet

import (
	"context"
	"errors"
	"fmt"
	"time"

	esicharacter "github.com/eve-online-tools/lib-esi-go/common/character"
	"github.com/eve-online-tools/lib-esi-go/esi/getcharacterscharacterid"

	"github.com/eve-online-tools/yulai/core/esi"
	"github.com/eve-online-tools/yulai/core/task"
)

type CharacterInput struct{ CharacterID int64 }

func (in CharacterInput) Subject() string { return fmt.Sprintf("char:%d", in.CharacterID) }

// Corporations and alliances are only reached by fan-out from here.
var FetchCharacter = task.New(
	(*Sheet).character,
	task.WithStartup(),
	task.WithInterval(characterEvery),
	task.WithTimeout(timeout),
)

func (s *Sheet) characters(ctx context.Context) ([]CharacterInput, error) {
	ids, err := s.chars.IDs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]CharacterInput, len(ids))
	for i, id := range ids {
		out[i] = CharacterInput{CharacterID: id}
	}
	return out, nil
}

func (s *Sheet) character(ctx context.Context, in CharacterInput) (struct{}, error) {
	data, err := esi.Check(getcharacterscharacterid.Request(ctx, s.esi,
		&getcharacterscharacterid.Input{Character: esicharacter.Identifier(in.CharacterID)}))
	if err != nil {
		return struct{}{}, err
	}
	if data == nil {
		return struct{}{}, errors.New("charactersheet: empty character response")
	}
	if err := s.q.UpsertCharacter(ctx, UpsertCharacterParams{
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
	}); err != nil {
		return struct{}{}, err
	}
	if err := s.chars.SetAffiliation(ctx, in.CharacterID, int64(data.Corporation), (*int64)(data.Alliance)); err != nil {
		return struct{}{}, err
	}
	s.events.Emit(EventChanged)

	corp := CorporationInput{CorporationID: data.Corporation}
	if s.claim(corp.Subject(), corporationEvery) {
		if err := FetchCorporation.Queue(ctx, corp); err != nil {
			s.release(corp.Subject())
			return struct{}{}, err
		}
	}
	return struct{}{}, nil
}
