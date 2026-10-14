package feature

import (
	"context"
	"fmt"

	"github.com/eve-online-tools/yulai/core/task"
)

// CharacterInput is the input of per-character tasks.
type CharacterInput struct {
	CharacterID int64
}

func (in CharacterInput) Subject() string { return CharacterSubject(in.CharacterID) }

// CharacterSubject is the task subject of a character, for task.Scheduler.Refresh.
func CharacterSubject(characterID int64) string { return fmt.Sprintf("char:%d", characterID) }

// CharacterSeed seeds a task with the characters list returns.
func CharacterSeed(list func(ctx context.Context) ([]int64, error)) task.Seed[CharacterInput] {
	return func(ctx context.Context) ([]CharacterInput, error) {
		ids, err := list(ctx)
		if err != nil {
			return nil, err
		}
		out := make([]CharacterInput, len(ids))
		for i, id := range ids {
			out[i] = CharacterInput{CharacterID: id}
		}
		return out, nil
	}
}

// ScopeSeed seeds a task with the characters whose working token covers every scope.
func ScopeSeed(c Characters, scopes ...string) task.Seed[CharacterInput] {
	return CharacterSeed(func(ctx context.Context) ([]int64, error) {
		return c.WithScopes(ctx, scopes...)
	})
}
