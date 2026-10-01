// Package db opens the app database and applies migrations. It owns no tables;
// the schema is global and lives in migrations/, table owners write queries.sql.
package db

import (
	"context"
	"database/sql"

	"github.com/eve-online-tools/yulai/core/todo"
)

// Open opens the sqlite database at path and applies pending goose migrations.
//
// Planned: glebarez/go-sqlite (pure Go), WAL, foreign keys, busy_timeout, a single
// writer connection, migrations embedded from migrations/*.sql.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	return nil, todo.ErrNotImplemented
}
