// Package crypt encrypts values at rest with a master key held in a keyring.Store.
package crypt

import (
	"github.com/eve-online-tools/yulai/core/keyring"
	"github.com/eve-online-tools/yulai/core/todo"
)

// Sealer will do AES-GCM with a 256-bit key.
type Sealer struct{}

// Open loads the master key from the store, creating one on first run.
func Open(store keyring.Store) (*Sealer, error) { return &Sealer{}, nil }

func (s *Sealer) Seal(plain string) ([]byte, error)  { return nil, todo.ErrNotImplemented }
func (s *Sealer) Unseal(blob []byte) (string, error) { return "", todo.ErrNotImplemented }
