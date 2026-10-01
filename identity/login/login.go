// Package login runs the interactive login: local callback server, code exchange.
// It does not know how the user gets to the authorize URL; the caller shows it.
package login

import (
	"context"

	"github.com/eve-online-tools/yulai/core/todo"
	"github.com/eve-online-tools/yulai/identity/sso"
)

type Result struct {
	sso.Identity
	sso.Tokens
}

type Login struct {
	client   *sso.Client
	verifier *sso.Verifier
}

func New(client *sso.Client, verifier *sso.Verifier) *Login {
	return &Login{client: client, verifier: verifier}
}

// Pending is one login attempt: the listener is up and waiting for the callback.
type Pending struct {
	// URL is where the user's browser must go.
	URL string
}

// Start binds the callback port and returns the authorize URL. Call Wait to finish,
// or Cancel to give up. Only one attempt can be pending at a time (one port).
func (l *Login) Start(ctx context.Context, scopes []string) (*Pending, error) {
	return nil, todo.ErrNotImplemented
}

// Wait blocks until the callback arrived and the code was exchanged and verified.
func (p *Pending) Wait(ctx context.Context) (*Result, error) { return nil, todo.ErrNotImplemented }

// Cancel stops the callback listener and frees the port.
func (p *Pending) Cancel() {}
