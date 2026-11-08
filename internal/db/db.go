// Package db opens SQLite and creates the schema. Given code: read it, don't change it.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	_ "github.com/mattn/go-sqlite3" // registers the "sqlite3" driver with database/sql
)

// DBTX is the small slice of database/sql that repositories need. Both *sql.DB and *sql.Tx satisfy it, so the same
// repository code can run inside or outside a transaction. ("Accept interfaces, return structs.")
type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Open opens (creating if needed) the SQLite database file at path.
//
// Notice that *sql.DB is a POOL of connections, not a single connection: it is safe for concurrent use and is meant
// to be created once and shared. Options in the DSN apply to every pooled connection:
//
//	_busy_timeout=5000    wait up to 5s for a lock instead of failing at once with "database is locked"
//	_journal_mode=WAL     readers and one writer can work at the same time
//	_txlock=immediate     db.BeginTx starts "BEGIN IMMEDIATE": a writer takes the write lock up-front, so concurrent
//	                      transactions queue up politely instead of failing halfway through
func Open(path string) (*sql.DB, error) {
	return sql.Open("sqlite3", path+"?_busy_timeout=5000&_journal_mode=WAL&_txlock=immediate")
}

// Initialize runs the schema script and, if seedPath is not empty, the seed script.
func Initialize(ctx context.Context, d *sql.DB, schemaPath, seedPath string) error {
	for _, path := range []string{schemaPath, seedPath} {
		if path == "" {
			continue
		}
		script, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if _, err := d.ExecContext(ctx, string(script)); err != nil {
			return fmt.Errorf("run %s: %w", path, err)
		}
	}
	return nil
}
