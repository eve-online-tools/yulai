// Package keyring stores small secrets in the OS credential store.
package keyring

import (
	"errors"

	"github.com/eve-online-tools/yulai/core/todo"
)

var ErrNotFound = errors.New("keyring: not found")

// Store is a named secret slot. Desktop uses the OS keychain; mobile will need another impl.
type Store interface {
	Get() ([]byte, error)
	Set(value []byte) error
}

// OS will be backed by Windows Credential Manager, macOS Keychain or Secret Service
// on Linux (zalando/go-keyring).
type OS struct {
	Service string
	User    string
}

func (k OS) Get() ([]byte, error)   { return nil, todo.ErrNotImplemented }
func (k OS) Set(value []byte) error { return todo.ErrNotImplemented }
