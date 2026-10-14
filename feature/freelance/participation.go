package freelance

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	esicharacter "github.com/eve-online-tools/lib-esi-go/common/character"
	"github.com/eve-online-tools/lib-esi-go/common/uuid"
	getparticipation "github.com/eve-online-tools/lib-esi-go/esi/getcharactersfreelancejobsparticipation"

	"github.com/eve-online-tools/yulai/core/db"
	"github.com/eve-online-tools/yulai/core/esi"
	"github.com/eve-online-tools/yulai/core/task"
)

type ParticipationInput struct {
	CharacterID int64
	JobID       uuid.UUID
}

func (in ParticipationInput) Subject() string {
	return fmt.Sprintf("char:%d:freelance:%s", in.CharacterID, in.JobID)
}

var FetchParticipation = task.New(
	(*Freelance).participation,
	task.WithTimeout(timeout),
)

// participation returns nil when the character has none in the job, such as a job
// their corporation owns.
func (f *Freelance) participation(ctx context.Context, in ParticipationInput) (*CharacterFreelanceParticipation, error) {
	resp, err := getparticipation.Request(
		ctx, f.esi,
		&getparticipation.Input{
			Character: esicharacter.Identifier(in.CharacterID),
			JobId:     in.JobID,
		},
		f.auth(in.CharacterID),
	)
	if err != nil {
		return nil, err
	}
	err = esi.ResponseError(resp)
	if e := (*esi.Error)(nil); errors.As(err, &e) && e.Status == http.StatusNotFound {
		return nil, f.noParticipation(ctx, in)
	}
	if err != nil {
		return nil, err
	}
	data := resp.Data

	prev, err := f.q.GetParticipation(ctx, GetParticipationParams{CharacterID: in.CharacterID, JobID: in.JobID})
	existed, err := db.Found(err)
	if err != nil {
		return nil, err
	}
	row, err := f.q.UpsertParticipation(ctx, UpsertParticipationParams{
		CharacterID:  in.CharacterID,
		JobID:        in.JobID,
		Contributed:  data.Contributed,
		State:        data.State,
		LastModified: data.LastModified.UTC(),
		FetchedAt:    time.Now().UTC(),
	})
	if err != nil {
		return nil, err
	}
	if !existed || db.Changed(prev, row) {
		f.events.Changed(Topic)
	}
	return &row, nil
}

// noParticipation drops a stored participation ESI no longer has and remembers
// the 404 against the job's current version.
func (f *Freelance) noParticipation(ctx context.Context, in ParticipationInput) error {
	job, err := f.q.GetCharacterJob(ctx, GetCharacterJobParams{CharacterID: in.CharacterID, JobID: in.JobID})
	found, err := db.Found(err)
	if err != nil {
		return err
	}
	if found {
		f.mu.Lock()
		f.none[in] = job.LastModified
		f.mu.Unlock()
	}
	n, err := f.q.DeleteParticipation(ctx, DeleteParticipationParams{CharacterID: in.CharacterID, JobID: in.JobID})
	if err != nil {
		return err
	}
	if n > 0 {
		f.events.Changed(Topic)
	}
	return nil
}
