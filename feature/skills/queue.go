package skills

import (
	"context"
	"time"

	esicharacter "github.com/eve-online-tools/lib-esi-go/common/character"
	getqueue "github.com/eve-online-tools/lib-esi-go/esi/getcharacterscharacteridskillqueue"

	"github.com/eve-online-tools/yulai/core/esi"
	"github.com/eve-online-tools/yulai/core/task"
)

var FetchQueue = task.New(
	(*Feature).queue,
	task.WithStartup(),
	task.WithInterval(every),
	task.WithTimeout(timeout),
)

// queue returns the stored queue in position order, empty when nothing is queued.
func (f *Feature) queue(ctx context.Context, in Input) ([]SkillQueue, error) {
	resp, err := getqueue.Request(
		ctx, f.esi,
		&getqueue.Input{
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

	var rows []SkillQueue
	changed := false
	now := time.Now().UTC()
	err = f.inTx(ctx, func(q *Queries) error {
		prev, err := q.ListQueue(ctx, in.CharacterID)
		if err != nil {
			return err
		}
		if err := q.DeleteQueue(ctx, in.CharacterID); err != nil {
			return err
		}
		for _, e := range data {
			_, err := q.InsertQueueEntry(ctx, InsertQueueEntryParams{
				CharacterID:     in.CharacterID,
				QueuePosition:   e.QueuePosition,
				SkillID:         e.Skill,
				FinishedLevel:   e.FinishedLevel,
				StartDate:       e.StartDate,
				FinishDate:      e.FinishDate,
				TrainingStartSp: e.TrainingStartSp,
				LevelStartSp:    e.LevelStartSp,
				LevelEndSp:      e.LevelEndSp,
				FetchedAt:       now,
			})
			if err != nil {
				return err
			}
		}
		rows, err = q.ListQueue(ctx, in.CharacterID)
		if err != nil {
			return err
		}
		changed = listChanged(prev, rows)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if changed {
		f.events.Emit(EventChanged)
	}
	return rows, nil
}
