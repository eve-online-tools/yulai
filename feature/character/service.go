// Package character owns the characters table and the add-character flow.
package character

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"

	esicharacter "github.com/eve-online-tools/lib-esi-go/common/character"
	getcharacter "github.com/eve-online-tools/lib-esi-go/esi/getcharacterscharacterid"

	"github.com/eve-online-tools/yulai/core/esi"
	"github.com/eve-online-tools/yulai/feature"
	"github.com/eve-online-tools/yulai/identity/login"
	"github.com/eve-online-tools/yulai/identity/token"
)

const EventChanged = "character:changed"

const (
	StatusOK         = "ok"
	StatusNeedsLogin = "needs_login"
)

// Emitter sends app events. Implemented by app with Wails.
type Emitter interface {
	Emit(name string)
}

type Service struct {
	q      *Queries
	events Emitter
	tokens *token.Store
	esi    *http.Client
}

func NewService(conn *sql.DB, events Emitter, tokens *token.Store, esiClient *http.Client) *Service {
	return &Service{q: New(conn), events: events, tokens: tokens, esi: esiClient}
}

func (s *Service) ServiceName() string { return "CharacterService" }

func (s *Service) List(ctx context.Context) ([]ListRow, error) {
	rows, err := s.q.List(ctx)
	if err != nil {
		return nil, err
	}
	if rows == nil {
		rows = []ListRow{}
	}
	return rows, nil
}

// WithScopes lists characters with a working token that covers every scope.
//
//wails:ignore
func (s *Service) WithScopes(ctx context.Context, scopes ...string) ([]int64, error) {
	rows, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	var out []int64
	for _, r := range rows {
		if r.Status == StatusOK && r.TokenExpiresAt != nil && feature.Covers(strings.Fields(r.Scopes), scopes) {
			out = append(out, r.ID)
		}
	}
	return out, nil
}

// Store upserts the character and saves its tokens, replacing earlier scopes. It is
// the login server's handler.
//
//wails:ignore
func (s *Service) Store(ctx context.Context, r *login.Result) error {
	// Public info; no token needed.
	resp, err := getcharacter.Request(
		ctx, s.esi,
		&getcharacter.Input{
			Character: esicharacter.Identifier(r.CharacterID),
		},
	)
	if err != nil {
		return err
	}
	if err := esi.ResponseError(resp); err != nil {
		return err
	}
	info := resp.Data
	var allianceID *int64
	if info.Alliance != nil {
		v := int64(*info.Alliance)
		allianceID = &v
	}

	now := time.Now().Unix()
	if err := s.q.Upsert(ctx, UpsertParams{
		ID:            r.CharacterID,
		Name:          r.Name,
		OwnerHash:     r.OwnerHash,
		CorporationID: int64(info.Corporation),
		AllianceID:    allianceID,
		AddedAt:       now,
		UpdatedAt:     now,
	}); err != nil {
		return err
	}
	if err := s.tokens.Save(ctx, r.CharacterID, &r.Tokens, &r.Identity); err != nil {
		return err
	}
	s.events.Emit(EventChanged)
	return nil
}

func (s *Service) Remove(ctx context.Context, characterID int64) error {
	if err := s.q.Delete(ctx, characterID); err != nil {
		return err
	}
	s.tokens.Forget(characterID)
	s.events.Emit(EventChanged)
	return nil
}

// MarkNeedsLogin flags a character whose refresh token stopped working.
//
//wails:ignore
func (s *Service) MarkNeedsLogin(ctx context.Context, characterID int64, reason string) error {
	if err := s.q.SetStatus(ctx, SetStatusParams{
		Status: StatusNeedsLogin, StatusError: &reason, UpdatedAt: time.Now().Unix(), ID: characterID,
	}); err != nil {
		return err
	}
	s.events.Emit(EventChanged)
	return nil
}

// IDs lists every character, whatever its token status.
//
//wails:ignore
func (s *Service) IDs(ctx context.Context) ([]int64, error) {
	rows, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]int64, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out, nil
}

// SetAffiliation records a character's current corporation and alliance.
//
//wails:ignore
func (s *Service) SetAffiliation(ctx context.Context, characterID, corporationID int64, allianceID *int64) error {
	n, err := s.q.SetAffiliation(ctx, SetAffiliationParams{
		CorporationID: corporationID, AllianceID: allianceID, UpdatedAt: time.Now().Unix(), ID: characterID,
	})
	if err != nil {
		return err
	}
	if n > 0 {
		s.events.Emit(EventChanged)
	}
	return nil
}
