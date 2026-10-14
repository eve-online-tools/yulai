package character

import (
	"context"
	"errors"
	"fmt"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/eve-online-tools/yulai/core/task"
	"github.com/eve-online-tools/yulai/feature"
	"github.com/eve-online-tools/yulai/identity/sso"
	"github.com/eve-online-tools/yulai/identity/token"
)

// Input is one character for per-character tasks.
type Input struct {
	CharacterID int64
}

func (in Input) Subject() string { return fmt.Sprintf("char:%d", in.CharacterID) }

// RefreshTokens renews every healthy token at startup, so a revoked refresh token is
// flagged and the last refreshed time is current before the user looks.
var RefreshTokens = task.New((*Service).refreshToken, task.WithStartup(), task.Pausable("character.refresh-token"))

// Feature is always on: the character list and token upkeep.
func Feature(s *Service) feature.Feature {
	return feature.Feature{
		ID: "character",
		Tasks: []task.Binding{
			RefreshTokens.Bind(s, task.WithParameters(s.withToken)),
		},
		Services: []application.Service{
			application.NewService(s),
		},
	}
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
	// The token store already marked the character.
	if errors.Is(err, sso.ErrInvalidGrant) || errors.Is(err, token.ErrUnreadable) {
		return struct{}{}, nil
	}
	if err != nil {
		return struct{}{}, err
	}
	s.events.Emit(EventChanged)
	return struct{}{}, nil
}
