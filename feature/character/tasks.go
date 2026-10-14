package character

import (
	"context"
	"errors"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/eve-online-tools/yulai/core/task"
	"github.com/eve-online-tools/yulai/feature"
	"github.com/eve-online-tools/yulai/identity/sso"
	"github.com/eve-online-tools/yulai/identity/token"
)

// RefreshTokens renews every healthy token at startup, so a revoked refresh token is
// flagged and the last refreshed time is current before the user looks.
var RefreshTokens = task.New((*Service).refreshToken, task.WithStartup())

// Feature is always on: the character list and token upkeep.
func Feature(s *Service) feature.Feature {
	return feature.Feature{
		ID: "character",
		Tasks: []task.Binding{
			RefreshTokens.Bind(s, task.WithParameters(feature.ScopeSeed(s))),
		},
		Services: []application.Service{
			application.NewService(s),
		},
	}
}

func (s *Service) refreshToken(ctx context.Context, in feature.CharacterInput) (struct{}, error) {
	err := s.tokens.Refresh(ctx, in.CharacterID)
	// The token store already marked the character.
	if errors.Is(err, sso.ErrInvalidGrant) || errors.Is(err, token.ErrUnreadable) {
		return struct{}{}, nil
	}
	if err != nil {
		return struct{}{}, err
	}
	s.events.Changed(Topic)
	return struct{}{}, nil
}
