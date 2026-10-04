package skills

import (
	"context"
	"time"

	esicharacter "github.com/eve-online-tools/lib-esi-go/common/character"
	getskills "github.com/eve-online-tools/lib-esi-go/esi/getcharacterscharacteridskills"

	"github.com/eve-online-tools/yulai/core/db"
	"github.com/eve-online-tools/yulai/core/esi"
	"github.com/eve-online-tools/yulai/core/task"
)

// SkillList is the stored skills response.
type SkillList struct {
	Totals SkillTotal `json:"totals"`
	Skills []Skill    `json:"skills"`
}

var FetchSkills = task.New(
	(*Feature).skills,
	task.WithStartup(),
	task.WithInterval(every),
	task.WithTimeout(timeout),
)

func (f *Feature) skills(ctx context.Context, in Input) (*SkillList, error) {
	resp, err := getskills.Request(
		ctx, f.esi,
		&getskills.Input{
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

	var out SkillList
	changed := false
	err = f.inTx(ctx, func(q *Queries) error {
		prevTotals, err := q.GetTotals(ctx, in.CharacterID)
		existed, err := db.Found(err)
		if err != nil {
			return err
		}
		prevSkills, err := q.ListSkills(ctx, in.CharacterID)
		if err != nil {
			return err
		}
		out.Totals, err = q.UpsertTotals(ctx, UpsertTotalsParams{
			CharacterID:   in.CharacterID,
			TotalSp:       data.TotalSp,
			UnallocatedSp: data.UnallocatedSp,
			FetchedAt:     time.Now().UTC(),
		})
		if err != nil {
			return err
		}
		if err := q.DeleteSkills(ctx, in.CharacterID); err != nil {
			return err
		}
		for _, s := range data.Skills {
			_, err := q.InsertSkill(ctx, InsertSkillParams{
				CharacterID:        in.CharacterID,
				SkillID:            s.SkillId,
				ActiveSkillLevel:   s.ActiveSkillLevel,
				TrainedSkillLevel:  s.TrainedSkillLevel,
				SkillpointsInSkill: s.SkillpointsInSkill,
			})
			if err != nil {
				return err
			}
		}
		// Read back so both lists compare in skill_id order.
		out.Skills, err = q.ListSkills(ctx, in.CharacterID)
		if err != nil {
			return err
		}
		changed = !existed || db.Changed(prevTotals, out.Totals) || listChanged(prevSkills, out.Skills)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if changed {
		f.events.Emit(EventChanged)
	}
	return &out, nil
}
