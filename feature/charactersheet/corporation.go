package charactersheet

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/eve-online-tools/lib-esi-go/common/corporation"
	"github.com/eve-online-tools/lib-esi-go/esi/getcorporationscorporationid"

	"github.com/eve-online-tools/yulai/core/esi"
	"github.com/eve-online-tools/yulai/core/task"
)

type CorporationInput struct{ CorporationID corporation.Identifier }

func (in CorporationInput) Subject() string { return fmt.Sprintf("corp:%d", in.CorporationID) }

var FetchCorporation = task.New(
	(*Sheet).corporation,
	task.WithTimeout(timeout),
)

func (s *Sheet) corporation(ctx context.Context, in CorporationInput) (struct{}, error) {
	data, err := esi.Check(getcorporationscorporationid.Request(ctx, s.esi,
		&getcorporationscorporationid.Input{Corporation: in.CorporationID}))
	if err == nil && data == nil {
		err = errors.New("charactersheet: empty corporation response")
	}
	if err == nil {
		err = s.q.UpsertCorporation(ctx, UpsertCorporationParams{
			ID:                in.CorporationID,
			Name:              data.Name,
			Ticker:            data.Ticker,
			AllianceID:        data.Alliance,
			CeoID:             data.Ceo,
			CreatorID:         data.Creator,
			EnlistedFactionID: data.EnlistedFaction,
			HomeStationID:     data.HomeStation,
			DateFounded:       data.DateFounded,
			MemberCount:       data.MemberCount,
			TaxRate:           data.TaxRates.Isk,
			WarEligible:       data.WarEligible,
			Url:               data.Url,
			Description:       data.Description,
			FetchedAt:         time.Now().UTC(),
		})
	}
	if err != nil {
		s.release(in.Subject())
		return struct{}{}, err
	}
	s.events.Emit(EventChanged)

	if data.Alliance == nil {
		return struct{}{}, nil
	}
	a := AllianceInput{AllianceID: *data.Alliance}
	if s.claim(a.Subject(), allianceEvery) {
		if err := FetchAlliance.Queue(ctx, a); err != nil {
			s.release(a.Subject())
			return struct{}{}, err
		}
	}
	return struct{}{}, nil
}
