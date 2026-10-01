// Package token persists SSO tokens per character, remembers what scopes the user
// consented to, and hands out refreshable tokens for ESI requests.
package token

import (
	"context"
	"database/sql"

	"github.com/eve-online-tools/lib-esi-go/middleware/authentication"

	"github.com/eve-online-tools/yulai/core/crypt"
	"github.com/eve-online-tools/yulai/core/todo"
	"github.com/eve-online-tools/yulai/identity/sso"
)

// Store will own the tokens table. Refresh tokens are sealed with crypt; the scopes
// column is the scp claim of the last access token and is the only record of consent.
type Store struct {
	conn     *sql.DB
	sso      *sso.Client
	verifier *sso.Verifier
	sealer   *crypt.Sealer
}

func NewStore(conn *sql.DB, client *sso.Client, verifier *sso.Verifier, sealer *crypt.Sealer) *Store {
	return &Store{conn: conn, sso: client, verifier: verifier, sealer: sealer}
}

// Save persists tokens and their scopes for a character. The character row must exist.
func (s *Store) Save(ctx context.Context, characterID int64, t *sso.Tokens, id *sso.Identity) error {
	return todo.ErrNotImplemented
}

// Scopes returns what the character's token was granted.
func (s *Store) Scopes(ctx context.Context, characterID int64) ([]string, error) {
	return nil, todo.ErrNotImplemented
}

// For returns a token that refreshes itself, for lib-esi-go's authentication middleware.
func (s *Store) For(characterID int64) authentication.RefreshableToken { return nil }

// Forget drops any cached token for the character.
func (s *Store) Forget(characterID int64) {}
