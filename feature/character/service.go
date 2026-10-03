// Package character owns the characters table and the add-character flow.
package character

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/url"
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

// Enroller is implemented by the sync scheduler. Kept as an interface so character
// does not import sync.
type Enroller interface {
	Enroll(ctx context.Context, characterID int64) error
}

// Browser opens URLs in the system browser. Implemented by app with Wails.
type Browser interface {
	OpenURL(url string) error
}

// Emitter sends app events. Implemented by app with Wails.
type Emitter interface {
	Emit(name string)
}

// FeatureInfo is what the frontend needs to show opt-in choices.
type FeatureInfo struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

type Service struct {
	q        *Queries
	loginURL string
	browser  Browser
	events   Emitter
	tokens   *token.Store
	esi      *http.Client
	features []feature.Feature
	enroller Enroller
}

// loginURL is the login server's picker page.
func NewService(conn *sql.DB, loginURL string, browser Browser, events Emitter, tokens *token.Store, esiClient *http.Client, features []feature.Feature, enroller Enroller) *Service {
	return &Service{q: New(conn), loginURL: loginURL, browser: browser, events: events, tokens: tokens, esi: esiClient, features: features, enroller: enroller}
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

// WithFeature lists characters with a working token that unlocks f.
//
//wails:ignore
func (s *Service) WithFeature(ctx context.Context, f feature.Feature) ([]int64, error) {
	return s.WithScopes(ctx, f.Scopes()...)
}

// Features lists every opt-in feature and the scopes it needs. The frontend diffs
// them against a character's token scopes to show what is enabled.
func (s *Service) Features() []FeatureInfo {
	out := make([]FeatureInfo, 0, len(s.features))
	for _, f := range s.features {
		out = append(out, FeatureInfo{Name: f.Name(), Scopes: f.Scopes()})
	}
	return out
}

// AddCharacter opens the login server's feature picker in the system browser.
// Completed logins arrive through Store and are reported through EventChanged.
func (s *Service) AddCharacter() error { return s.browser.OpenURL(s.loginURL) }

// Relogin opens the feature picker with the character's current features checked,
// to replace a token that stopped working.
func (s *Service) Relogin(ctx context.Context, characterID int64) error {
	scopes, err := s.tokens.Scopes(ctx, characterID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	q := url.Values{"feature": {""}}
	for _, f := range s.features {
		if feature.Covers(scopes, f.Scopes()) {
			q.Add("feature", f.Name())
		}
	}
	return s.browser.OpenURL(s.loginURL + "?" + q.Encode())
}

// Store upserts the character and saves its tokens, replacing earlier scopes, then
// enrolls it. It is the login server's handler.
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
	if err := s.enroller.Enroll(ctx, r.CharacterID); err != nil {
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
