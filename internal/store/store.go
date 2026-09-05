// Package store implements SPEC.md §9: the SQLite persistence layer behind
// --auth-db. No file under this package is ever created, opened, or
// touched unless --auth-db is passed — Open is the only entry point, and
// cmd/run.go and cmd/users.go are the only callers (CLAUDE.md: internal/store
// is imported only by cmd and security).
package store

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Store is a handle onto one --auth-db SQLite file: users, durable grants
// (v1.1) and the audit log (SPEC.md §9).
type Store struct {
	db *sql.DB
}

// pragmas are applied immediately after opening the database, before any
// migration runs (SPEC.md §9).
var pragmas = []string{
	"PRAGMA journal_mode=WAL",
	"PRAGMA busy_timeout=5000",
	"PRAGMA foreign_keys=ON",
	"PRAGMA synchronous=NORMAL",
}

// Open opens (creating if necessary) the SQLite file at path, applies the
// SPEC.md §9 pragmas, and migrates it to the current schema version. path
// must be non-empty; callers gate this on --auth-db being set so no file is
// ever created when auth is not configured.
func Open(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("store: path must not be empty")
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	// SQLite only tolerates one writer at a time; a single connection avoids
	// SQLITE_BUSY races that busy_timeout alone cannot fully paper over.
	db.SetMaxOpenConns(1)

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("store: apply %q: %w", pragma, err)
		}
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the underlying database handle.
func (s *Store) Close() error {
	return s.db.Close()
}
