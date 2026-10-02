// Package keyring stores small secrets in the OS credential store.
package keyring

import (
	"errors"

	"github.com/zalando/go-keyring"
)

var ErrNotFound = errors.New("keyring: not found")

// Store is a named secret slot. Desktop uses the OS keychain; mobile will need another impl.
type Store interface {
	Get() ([]byte, error)
	Set(value []byte) error
}

// OS is backed by Windows Credential Manager, macOS Keychain or Secret Service on Linux.
type OS struct {
	Service string
	User    string
}

func (k OS) Get() ([]byte, error) {
	s, err := keyring.Get(k.Service, k.User)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return []byte(s), nil
}

func (k OS) Set(value []byte) error {
	return keyring.Set(k.Service, k.User, string(value))
}
