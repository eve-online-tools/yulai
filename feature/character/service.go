// Package character owns the characters table and the add-character flow.
package character

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/eve-online-tools/yulai/core/todo"
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

// LoginUI is the window the login happens in. Implemented by app with Wails windows.
type LoginUI interface {
	// OpenPicker shows the feature picker, or focuses it if already open.
	OpenPicker()
	// ShowSSO navigates the picker window to the SSO and calls onClosed if the user
	// closes it before the login completes.
	ShowSSO(url string, onClosed func())
	// Fail returns the window to the picker with an error message.
	Fail(msg string)
	Close()
}

// FeatureInfo is what the frontend needs to show opt-in choices.
type FeatureInfo struct {
	Name   string   `json:"name"`
	Scopes []string `json:"scopes"`
}

// ListRow is one character as the UI sees it. sqlc will generate this from
// queries.sql once the characters table exists.
type ListRow struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	CorporationID  int64   `json:"corporationId"`
	AllianceID     *int64  `json:"allianceId"`
	Status         string  `json:"status"`
	StatusError    *string `json:"statusError"`
	Scopes         string  `json:"scopes"`
	TokenExpiresAt *int64  `json:"tokenExpiresAt"`
	TokenIssuedAt  *int64  `json:"tokenIssuedAt"`
}

type Service struct {
	conn     *sql.DB
	login    *login.Login
	ui       LoginUI
	tokens   *token.Store
	esi      *http.Client
	features []feature.Feature
	enroller Enroller
}

func NewService(conn *sql.DB, l *login.Login, ui LoginUI, tokens *token.Store, esiClient *http.Client, features []feature.Feature, enroller Enroller) *Service {
	return &Service{conn: conn, login: l, ui: ui, tokens: tokens, esi: esiClient, features: features, enroller: enroller}
}

func (s *Service) ServiceName() string { return "CharacterService" }

func (s *Service) List(ctx context.Context) ([]ListRow, error) {
	return []ListRow{}, nil
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

// OpenAddWindow shows the feature picker window.
func (s *Service) OpenAddWindow() { s.ui.OpenPicker() }

// BeginLogin is called from the picker window. It will start the SSO flow for the
// named features and send that window to the SSO; completion is reported through
// EventChanged.
func (s *Service) BeginLogin(features []string) error {
	return todo.ErrNotImplemented
}

func (s *Service) Remove(ctx context.Context, characterID int64) error {
	return todo.ErrNotImplemented
}

// MarkNeedsLogin flags a character whose refresh token stopped working.
//
//wails:ignore
func (s *Service) MarkNeedsLogin(ctx context.Context, characterID int64, reason string) error {
	return todo.ErrNotImplemented
}
