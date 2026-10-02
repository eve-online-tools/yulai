// Package db opens the app database and applies migrations. It owns no tables;
// the schema is global and lives in migrations/, table owners write queries.sql.
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	_ "github.com/glebarez/go-sqlite"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Open opens the app database and applies pending migrations.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	conn, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	// Single writer avoids SQLITE_BUSY between goroutines.
	conn.SetMaxOpenConns(1)

	goose.SetBaseFS(migrations)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("sqlite3"); err != nil {
		return nil, err
	}
	if err := goose.UpContext(ctx, conn, "migrations"); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return conn, nil
}
