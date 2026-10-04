package sde

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"

	"github.com/eve-online-tools/yulai/core/task"
)

var ErrNotInstalled = errors.New("sde: static data not installed yet")

const fileName = "sde.sqlite"

// Store holds the installed SDE database. Read takes a read lock and install a
// write lock, so a swap never closes the database under a query.
type Store struct {
	dir string
	log *slog.Logger

	mu   sync.RWMutex
	conn *sql.DB
	q    *Queries
	meta *Meta
	// gen counts opens, so Derived values know when the database changed.
	gen uint64
	// derived builds the registered Derived values for a new install.
	derived []func()
}

// Open opens the SDE in dir, if one is installed, and removes leftovers of
// interrupted updates. A database that cannot be read is removed.
func Open(ctx context.Context, dir string, log *slog.Logger) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, log: log}
	if err := os.Remove(s.path() + ".tmp"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	err := s.open(ctx)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		log.Warn("sde: unreadable, removing", "err", err)
		if err := os.Remove(s.path()); err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Store) path() string { return filepath.Join(s.dir, fileName) }

// open opens the database file. Callers hold mu or own s exclusively.
func (s *Store) open(ctx context.Context) error {
	if _, err := os.Stat(s.path()); err != nil {
		return err
	}
	conn, err := sql.Open("sqlite", s.path()+"?_pragma=query_only(1)")
	if err != nil {
		return err
	}
	q := New(conn)
	meta, err := q.GetMeta(ctx)
	if err != nil {
		conn.Close()
		return fmt.Errorf("sde: read meta: %w", err)
	}
	s.conn, s.q, s.meta = conn, q, &meta
	s.gen++
	return nil
}

// Read runs fn with the installed SDE. It returns ErrNotInstalled before the
// first install.
func (s *Store) Read(ctx context.Context, fn func(q *Queries) error) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.q == nil {
		return ErrNotInstalled
	}
	return fn(s.q)
}

// Meta describes the installed SDE, nil before the first install.
func (s *Store) Meta() *Meta {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.meta
}

// Outdated reports whether b differs from the installed build, or the installed
// database has another schema than this binary.
func (s *Store) Outdated(b Build) bool {
	m := s.Meta()
	return m == nil || m.Build != b.Number || m.Schema != schemaHash
}

// Install builds a database from the export zip at src and swaps it in.
func (s *Store) Install(ctx context.Context, src string) error {
	tmp := s.path() + ".tmp"
	if err := build(ctx, src, tmp, s.log); err != nil {
		os.Remove(tmp)
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn != nil {
		// Windows cannot rename over an open file.
		if err := s.conn.Close(); err != nil {
			return err
		}
		s.conn, s.q, s.meta = nil, nil, nil
	}
	ctx = context.WithoutCancel(ctx)
	if err := os.Rename(tmp, s.path()); err != nil {
		return errors.Join(err, s.open(ctx))
	}
	if err := s.open(ctx); err != nil {
		return err
	}
	// They wait for the read lock, so they start once this returns.
	for _, warm := range s.derived {
		go warm()
	}
	return nil
}

// Close closes the database.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn == nil {
		return nil
	}
	err := s.conn.Close()
	s.conn, s.q, s.meta = nil, nil, nil
	return err
}

// Installed is a task condition that holds once an SDE is installed.
func (s *Store) Installed() task.Condition { return installed{s} }

type installed struct{ s *Store }

func (i installed) Check(context.Context) task.Verdict {
	if i.s.Meta() == nil {
		return task.Deny("static data not installed yet")
	}
	return task.Allow()
}
