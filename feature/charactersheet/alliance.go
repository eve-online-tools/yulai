package charactersheet

import (
	"context"
	"fmt"
	"time"

	"github.com/eve-online-tools/lib-esi-go/common/alliance"
	"github.com/eve-online-tools/lib-esi-go/esi/getalliancesallianceid"

	"github.com/eve-online-tools/yulai/core/db"
	"github.com/eve-online-tools/yulai/core/esi"
	"github.com/eve-online-tools/yulai/core/task"
)

type AllianceInput struct{ AllianceID alliance.Identifier }

func (in AllianceInput) Subject() string { return fmt.Sprintf("alliance:%d", in.AllianceID) }

var FetchAlliance = task.New(
	(*Sheet).alliance,
	task.WithTimeout(timeout),
)

func (s *Sheet) alliance(ctx context.Context, in AllianceInput) (_ Alliance, err error) {
	// The next corporation fetch retries a failed fetch.
	defer func() {
		if err != nil {
			s.release(in.Subject())
		}
	}()

	input := &getalliancesallianceid.Input{Alliance: in.AllianceID}
	resp, err := getalliancesallianceid.Request(ctx, s.esi, input)
	if err != nil {
		return Alliance{}, err
	}
	if err := esi.ResponseError(resp); err != nil {
		return Alliance{}, err
	}
	data := resp.Data

	prev, err := s.q.GetAlliance(ctx, in.AllianceID)
	existed, err := db.Found(err)
	if err != nil {
		return Alliance{}, err
	}
	row, err := s.q.UpsertAlliance(ctx, UpsertAllianceParams{
		ID:                    in.AllianceID,
		Name:                  data.Name,
		Ticker:                data.Ticker,
		CreatorID:             data.Creator,
		CreatorCorporationID:  data.CreatorCorporation,
		ExecutorCorporationID: data.ExecutorCorporation,
		FactionID:             data.Faction,
		DateFounded:           data.DateFounded,
		FetchedAt:             time.Now().UTC(),
	})
	if err != nil {
		return Alliance{}, err
	}
	if !existed || db.Changed(prev, row) {
		s.events.Emit(EventChanged)
	}
	return row, nil
}
