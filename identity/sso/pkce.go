package sso

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// PKCE holds one login attempt's verifier, challenge and state.
type PKCE struct {
	Verifier  string
	Challenge string
	State     string
}

func NewPKCE() (*PKCE, error) {
	verifier, err := randomString(32)
	if err != nil {
		return nil, err
	}
	state, err := randomString(24)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(verifier))
	return &PKCE{
		Verifier:  verifier,
		Challenge: base64.RawURLEncoding.EncodeToString(sum[:]),
		State:     state,
	}, nil
}

func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
