package character

import (
	"context"
	"errors"
	"fmt"

	"github.com/eve-online-tools/yulai/core/task"
	"github.com/eve-online-tools/yulai/identity/sso"
)

// Input is one character for per-character tasks.
type Input struct {
	CharacterID int64
}

func (in Input) Subject() string { return fmt.Sprintf("char:%d", in.CharacterID) }

// RefreshTokens renews every healthy token at startup, so a revoked refresh token is
// flagged and the last refreshed time is current before the user looks.
var RefreshTokens = task.New((*Service).refreshToken, task.WithStartup(), task.Pausable("character.refresh-token"))

// Tasks binds the character tasks to this service.
//
//wails:ignore
func (s *Service) Tasks() []task.Binding {
	return []task.Binding{RefreshTokens.Bind(s, task.WithParameters(s.withToken))}
}

// withToken lists characters with a stored token that still works.
func (s *Service) withToken(ctx context.Context) ([]Input, error) {
	rows, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	var out []Input
	for _, r := range rows {
		if r.Status == StatusOK && r.TokenExpiresAt != nil {
			out = append(out, Input{CharacterID: r.ID})
		}
	}
	return out, nil
}

func (s *Service) refreshToken(ctx context.Context, in Input) (struct{}, error) {
	err := s.tokens.Refresh(ctx, in.CharacterID)
	if errors.Is(err, sso.ErrInvalidGrant) {
		return struct{}{}, s.MarkNeedsLogin(ctx, in.CharacterID, err.Error())
	}
	if err != nil {
		return struct{}{}, err
	}
	s.events.Emit(EventChanged)
	return struct{}{}, nil
}
