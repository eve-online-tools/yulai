package sde

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestDerived(t *testing.T) {
	s, err := Open(t.Context(), t.TempDir(), quiet)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	var builds atomic.Int64
	d := NewDerived(s, func(ctx context.Context, q *Queries) (int64, error) {
		m, err := q.GetMeta(ctx)
		if err != nil {
			return 0, err
		}
		builds.Add(1)
		return m.Build, nil
	})
	if _, err := d.Get(t.Context()); !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("Get before install: %v", err)
	}

	if err := s.Install(t.Context(), writeZip(t, t.TempDir(), fixture(1))); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return builds.Load() == 1 })
	for range 2 {
		got, err := d.Get(t.Context())
		if err != nil || got != 1 {
			t.Fatalf("Get = %d, %v", got, err)
		}
	}
	if n := builds.Load(); n != 1 {
		t.Fatalf("built %d times for one install", n)
	}

	if err := s.Install(t.Context(), writeZip(t, t.TempDir(), fixture(2))); err != nil {
		t.Fatal(err)
	}
	got, err := d.Get(t.Context())
	if err != nil || got != 2 {
		t.Fatalf("Get after reinstall = %d, %v", got, err)
	}
	waitFor(t, func() bool { return builds.Load() == 2 })
}

func TestDerivedWarmsOnInstalledStore(t *testing.T) {
	s, err := Open(t.Context(), t.TempDir(), quiet)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.Install(t.Context(), writeZip(t, t.TempDir(), fixture(1))); err != nil {
		t.Fatal(err)
	}

	var builds atomic.Int64
	NewDerived(s, func(context.Context, *Queries) (struct{}, error) {
		builds.Add(1)
		return struct{}{}, nil
	})
	waitFor(t, func() bool { return builds.Load() == 1 })
}

func waitFor(t *testing.T, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatal("timed out")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
