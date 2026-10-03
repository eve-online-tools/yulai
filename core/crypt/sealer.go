// Package crypt encrypts values at rest with a master key held in a keyring.Store.
package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"

	"github.com/eve-online-tools/yulai/core/keyring"
)

// Sealer does AES-GCM with a 256-bit key.
type Sealer struct {
	aead cipher.AEAD
}

// Open loads the master key from the store, creating one on first run.
func Open(store keyring.Store) (*Sealer, error) {
	raw, err := store.Get()
	var key []byte
	switch {
	case errors.Is(err, keyring.ErrNotFound):
		// Expected on first run. Otherwise tokens sealed with the lost key can no
		// longer be read and those characters must log in again.
		slog.Warn("crypt: no master key in keyring, creating a new one")
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		if err := store.Set([]byte(base64.StdEncoding.EncodeToString(key))); err != nil {
			return nil, fmt.Errorf("crypt: store master key: %w", err)
		}
	case err != nil:
		return nil, fmt.Errorf("crypt: load master key: %w", err)
	default:
		key, err = base64.StdEncoding.DecodeString(string(raw))
		if err != nil {
			return nil, fmt.Errorf("crypt: master key is not base64: %w", err)
		}
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Sealer{aead: aead}, nil
}

func (s *Sealer) Seal(plain string) ([]byte, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return s.aead.Seal(nonce, nonce, []byte(plain), nil), nil
}

func (s *Sealer) Unseal(blob []byte) (string, error) {
	n := s.aead.NonceSize()
	if len(blob) < n {
		return "", errors.New("crypt: ciphertext too short")
	}
	out, err := s.aead.Open(nil, blob[:n], blob[n:], nil)
	if err != nil {
		return "", err
	}
	return string(out), nil
}
