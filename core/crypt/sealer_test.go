package crypt

import (
	"testing"

	"github.com/eve-online-tools/yulai/core/keyring"
)

type memStore struct{ v []byte }

func (m *memStore) Get() ([]byte, error) {
	if m.v == nil {
		return nil, keyring.ErrNotFound
	}
	return m.v, nil
}

func (m *memStore) Set(v []byte) error { m.v = v; return nil }

func TestSealRoundTripAcrossOpens(t *testing.T) {
	store := &memStore{}
	s1, err := Open(store)
	if err != nil {
		t.Fatal(err)
	}
	if store.v == nil {
		t.Fatal("master key not stored on first open")
	}
	blob, err := s1.Seal("refresh-token")
	if err != nil {
		t.Fatal(err)
	}

	s2, err := Open(store)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s2.Unseal(blob)
	if err != nil || got != "refresh-token" {
		t.Fatalf("Unseal = %q, %v", got, err)
	}

	blob[len(blob)-1] ^= 1
	if _, err := s2.Unseal(blob); err == nil {
		t.Fatal("tampered blob unsealed")
	}
}
