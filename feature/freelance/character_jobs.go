package freelance

import (
	"context"
	"time"

	esicharacter "github.com/eve-online-tools/lib-esi-go/common/character"
	"github.com/eve-online-tools/lib-esi-go/common/uuid"
	getcharjobs "github.com/eve-online-tools/lib-esi-go/esi/getcharactersfreelancejobslisting"

	"github.com/eve-online-tools/yulai/core/esi"
	"github.com/eve-online-tools/yulai/core/task"
)

var FetchCharacterJobs = task.New(
	(*Freelance).characterJobs,
	task.WithStartup(),
	task.WithInterval(every),
	task.WithTimeout(timeout),
)

// characterJobs replaces the character's job list. It queues participation for
// jobs that changed or have none stored, since a contribution moves the job's
// progress and so its last_modified.
func (f *Freelance) characterJobs(ctx context.Context, in Input) ([]CharacterFreelanceJob, error) {
	resp, err := getcharjobs.Request(
		ctx, f.esi,
		&getcharjobs.Input{
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

	var rows []CharacterFreelanceJob
	var stale []uuid.UUID
	changed := false
	now := time.Now().UTC()
	err = f.inTx(ctx, func(q *Queries) error {
		prev, err := q.ListCharacterJobs(ctx, in.CharacterID)
		if err != nil {
			return err
		}
		parts, err := q.ListParticipation(ctx, in.CharacterID)
		if err != nil {
			return err
		}
		if err := q.DeleteCharacterJobs(ctx, in.CharacterID); err != nil {
			return err
		}
		for _, j := range data.FreelanceJobs {
			p := InsertCharacterJobParams{
				CharacterID:     in.CharacterID,
				JobID:           j.Id,
				Name:            j.Name,
				State:           j.State,
				ProgressCurrent: j.Progress.Current,
				ProgressDesired: j.Progress.Desired,
				LastModified:    j.LastModified.UTC(),
				FetchedAt:       now,
			}
			if j.Reward != nil {
				p.RewardInitial = &j.Reward.Initial
				p.RewardRemaining = &j.Reward.Remaining
			}
			if _, err := q.InsertCharacterJob(ctx, p); err != nil {
				return err
			}
		}
		if err := q.DeleteStaleParticipation(ctx, in.CharacterID); err != nil {
			return err
		}
		rows, err = q.ListCharacterJobs(ctx, in.CharacterID)
		if err != nil {
			return err
		}
		changed = listChanged(prev, rows)
		stale = f.needParticipation(in.CharacterID, prev, parts, rows)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if changed {
		f.events.Changed(Topic)
	}
	for _, id := range stale {
		err := FetchParticipation.Queue(ctx, ParticipationInput{CharacterID: in.CharacterID, JobID: id})
		if err != nil {
			return nil, err
		}
	}
	return rows, nil
}

// needParticipation lists the jobs in rows that are new, changed since prev, or
// have no participation stored and were not answered 404 at their current version.
func (f *Freelance) needParticipation(characterID int64, prev []CharacterFreelanceJob, parts []CharacterFreelanceParticipation, rows []CharacterFreelanceJob) []uuid.UUID {
	modified := map[uuid.UUID]time.Time{}
	for _, j := range prev {
		modified[j.JobID] = j.LastModified
	}
	have := map[uuid.UUID]bool{}
	for _, p := range parts {
		have[p.JobID] = true
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []uuid.UUID
	for _, j := range rows {
		last, ok := modified[j.JobID]
		if ok && last.Equal(j.LastModified) && have[j.JobID] {
			continue
		}
		key := ParticipationInput{CharacterID: characterID, JobID: j.JobID}
		if at, ok := f.none[key]; ok && at.Equal(j.LastModified) && !have[j.JobID] {
			continue
		}
		out = append(out, j.JobID)
	}
	return out
}
