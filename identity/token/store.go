// Package token persists SSO tokens per character, remembers what scopes the user
// consented to, and hands out refreshable tokens for ESI requests.
package token

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/eve-online-tools/lib-esi-go/middleware/authentication"

	"github.com/eve-online-tools/yulai/core/crypt"
	"github.com/eve-online-tools/yulai/identity/sso"
)

// refreshMargin is how early before expiry we refresh.
const refreshMargin = 60 * time.Second

// ErrReplaced means the stored token changed during a refresh, by a new login or
// removal. The next use loads the stored one.
var ErrReplaced = errors.New("token: replaced during refresh")

type Store struct {
	q        *Queries
	sso      *sso.Client
	verifier *sso.Verifier
	sealer   *crypt.Sealer

	mu    sync.Mutex
	cache map[int64]*refreshable
}

func NewStore(conn *sql.DB, client *sso.Client, verifier *sso.Verifier, sealer *crypt.Sealer) *Store {
	return &Store{q: New(conn), sso: client, verifier: verifier, sealer: sealer, cache: map[int64]*refreshable{}}
}

// Save persists tokens and their scopes for a character. The character row must exist.
func (s *Store) Save(ctx context.Context, characterID int64, t *sso.Tokens, id *sso.Identity) error {
	if err := s.persist(ctx, characterID, t, id); err != nil {
		return err
	}
	s.Forget(characterID)
	return nil
}

func (s *Store) persist(ctx context.Context, characterID int64, t *sso.Tokens, id *sso.Identity) error {
	enc, err := s.sealer.Seal(t.RefreshToken)
	if err != nil {
		return err
	}
	return s.q.UpsertToken(ctx, UpsertTokenParams{
		CharacterID:     characterID,
		AccessToken:     t.AccessToken,
		RefreshTokenEnc: enc,
		ExpiresAt:       t.ExpiresAt.Unix(),
		Scopes:          strings.Join(id.Scopes, " "),
		IssuedAt:        id.IssuedAt.Unix(),
	})
}

// Scopes returns what the character's current token is allowed to do.
func (s *Store) Scopes(ctx context.Context, characterID int64) ([]string, error) {
	raw, err := s.q.GetScopes(ctx, characterID)
	if err != nil {
		return nil, err
	}
	return strings.Fields(raw), nil
}

func (s *Store) load(ctx context.Context, characterID int64) (*Token, *sso.Tokens, error) {
	row, err := s.q.GetToken(ctx, characterID)
	if err != nil {
		return nil, nil, err
	}
	refresh, err := s.sealer.Unseal(row.RefreshTokenEnc)
	if err != nil {
		return nil, nil, fmt.Errorf("token: decrypt refresh token: %w", err)
	}
	return &row, &sso.Tokens{
		AccessToken:  row.AccessToken,
		RefreshToken: refresh,
		ExpiresAt:    time.Unix(row.ExpiresAt, 0),
	}, nil
}

// rotate stores refreshed tokens if the row still holds prev, and returns the new
// sealed refresh token.
func (s *Store) rotate(ctx context.Context, characterID int64, prev []byte, t *sso.Tokens) ([]byte, error) {
	enc, err := s.sealer.Seal(t.RefreshToken)
	if err != nil {
		return nil, err
	}
	n, err := s.q.RotateToken(ctx, RotateTokenParams{
		AccessToken:         t.AccessToken,
		RefreshTokenEnc:     enc,
		ExpiresAt:           t.ExpiresAt.Unix(),
		CharacterID:         characterID,
		PrevRefreshTokenEnc: prev,
	})
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrReplaced
	}
	return enc, nil
}

// For returns the shared refreshable token for a character, for use with
// authentication.WithToken on ESI requests.
func (s *Store) For(characterID int64) authentication.RefreshableToken {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.cache[characterID]
	if !ok {
		t = &refreshable{store: s, characterID: characterID}
		s.cache[characterID] = t
	}
	return t
}

// Refresh renews the character's token now, regardless of expiry.
func (s *Store) Refresh(ctx context.Context, characterID int64) error {
	return s.For(characterID).(*refreshable).refresh(ctx, true)
}

// Forget drops the in-memory token so the next use reloads from the database.
func (s *Store) Forget(characterID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.cache, characterID)
}

// refreshable implements lib-esi-go's RefreshableToken and ScopedToken. The mutex
// stops concurrent workers from refreshing the same token twice.
type refreshable struct {
	store       *Store
	characterID int64

	mu     sync.Mutex
	tokens *sso.Tokens
	scopes []string
	// sealed is the stored refresh token blob, compared on rotate.
	sealed []byte
}

func (t *refreshable) Owner() int64 { return t.characterID }

func (t *refreshable) Token() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.tokens == nil {
		return ""
	}
	return t.tokens.AccessToken
}

// Scopes lets lib-esi-go refuse a call locally when the token lacks the scope,
// instead of spending an ESI request on a 403.
func (t *refreshable) Scopes() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.scopes
}

func (t *refreshable) RefreshIfNeeded(ctx context.Context) error { return t.refresh(ctx, false) }

func (t *refreshable) refresh(ctx context.Context, force bool) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.tokens == nil {
		row, tk, err := t.store.load(ctx, t.characterID)
		if err != nil {
			return err
		}
		t.tokens, t.scopes, t.sealed = tk, strings.Fields(row.Scopes), row.RefreshTokenEnc
	}
	if !force && time.Until(t.tokens.ExpiresAt) > refreshMargin {
		return nil
	}

	fresh, err := t.store.sso.Refresh(ctx, t.tokens.RefreshToken)
	if err != nil {
		return err
	}
	// SSO may have rotated the refresh token, so the old one could be dead already.
	// Store the new one before anything else can fail, even if ctx ends.
	sealed, err := t.store.rotate(context.WithoutCancel(ctx), t.characterID, t.sealed, fresh)
	if errors.Is(err, ErrReplaced) {
		t.tokens, t.scopes, t.sealed = nil, nil, nil
		return err
	}
	if err != nil {
		return err
	}
	t.tokens, t.sealed = fresh, sealed

	// Scopes ride on the access token; re-read them so consent stays accurate. On
	// failure the stored scopes stay and the next refresh tries again.
	id, err := t.store.verifier.Verify(ctx, fresh.AccessToken)
	if err != nil {
		slog.Warn("token: verify refreshed token", "character", t.characterID, "err", err)
		return nil
	}
	if err := t.store.q.SetScopes(context.WithoutCancel(ctx), SetScopesParams{
		Scopes:          strings.Join(id.Scopes, " "),
		IssuedAt:        id.IssuedAt.Unix(),
		CharacterID:     t.characterID,
		RefreshTokenEnc: sealed,
	}); err != nil {
		slog.Warn("token: store scopes", "character", t.characterID, "err", err)
		return nil
	}
	t.scopes = id.Scopes
	return nil
}
