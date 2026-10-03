package charactersheet

import (
	"context"
	"fmt"
	"time"

	"github.com/eve-online-tools/lib-esi-go/common/corporation"
	"github.com/eve-online-tools/lib-esi-go/esi/getcorporationscorporationid"

	"github.com/eve-online-tools/yulai/core/db"
	"github.com/eve-online-tools/yulai/core/esi"
	"github.com/eve-online-tools/yulai/core/task"
)

type CorporationInput struct{ CorporationID corporation.Identifier }

func (in CorporationInput) Subject() string { return fmt.Sprintf("corp:%d", in.CorporationID) }

var FetchCorporation = task.New(
	(*Sheet).corporation,
	task.WithTimeout(timeout),
)

func (s *Sheet) corporation(ctx context.Context, in CorporationInput) (_ Corporation, err error) {
	// The next character tick retries a failed fetch.
	defer func() {
		if err != nil {
			s.release(in.Subject())
		}
	}()

	input := &getcorporationscorporationid.Input{Corporation: in.CorporationID}
	resp, err := getcorporationscorporationid.Request(ctx, s.esi, input)
	if err != nil {
		return Corporation{}, err
	}
	if err := esi.ResponseError(resp); err != nil {
		return Corporation{}, err
	}
	data := resp.Data

	prev, err := s.q.GetCorporation(ctx, in.CorporationID)
	existed, err := db.Found(err)
	if err != nil {
		return Corporation{}, err
	}
	row, err := s.q.UpsertCorporation(ctx, UpsertCorporationParams{
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
	if err != nil {
		return Corporation{}, err
	}
	if !existed || db.Changed(prev, row) {
		s.events.Emit(EventChanged)
	}

	if row.AllianceID == nil {
		return row, nil
	}
	a := AllianceInput{AllianceID: *row.AllianceID}
	if s.claim(a.Subject(), allianceEvery) {
		if err := FetchAlliance.Queue(ctx, a); err != nil {
			s.release(a.Subject())
			return Corporation{}, err
		}
	}
	return row, nil
}
