package charactersheet

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/eve-online-tools/lib-esi-go/common/alliance"
	"github.com/eve-online-tools/lib-esi-go/esi/getalliancesallianceid"

	"github.com/eve-online-tools/yulai/core/esi"
	"github.com/eve-online-tools/yulai/core/task"
)

type AllianceInput struct{ AllianceID alliance.Identifier }

func (in AllianceInput) Subject() string { return fmt.Sprintf("alliance:%d", in.AllianceID) }

var FetchAlliance = task.New(
	(*Sheet).alliance,
	task.WithTimeout(timeout),
)

func (s *Sheet) alliance(ctx context.Context, in AllianceInput) (struct{}, error) {
	data, err := esi.Check(getalliancesallianceid.Request(ctx, s.esi,
		&getalliancesallianceid.Input{Alliance: in.AllianceID}))
	if err == nil && data == nil {
		err = errors.New("charactersheet: empty alliance response")
	}
	if err == nil {
		err = s.q.UpsertAlliance(ctx, UpsertAllianceParams{
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
	}
	if err != nil {
		s.release(in.Subject())
		return struct{}{}, err
	}
	s.events.Emit(EventChanged)
	return struct{}{}, nil
}
