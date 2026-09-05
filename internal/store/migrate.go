package store

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// migration is one forward-only, embedded-SQL schema step (SPEC.md §9's
// "versioned migration runner").
type migration struct {
	version int
	name    string
	sql     string
}

// loadMigrations reads every embedded migrations/NNNN_*.sql file, sorted by
// its numeric prefix.
func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, fmt.Errorf("store: read migrations: %w", err)
	}
	migrations := make([]migration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		prefix, _, ok := strings.Cut(entry.Name(), "_")
		if !ok {
			return nil, fmt.Errorf("store: migration filename %q missing NNNN_ prefix", entry.Name())
		}
		version, err := strconv.Atoi(prefix)
		if err != nil {
			return nil, fmt.Errorf("store: migration filename %q has a non-numeric prefix: %w", entry.Name(), err)
		}
		body, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return nil, fmt.Errorf("store: read %s: %w", entry.Name(), err)
		}
		migrations = append(migrations, migration{version: version, name: entry.Name(), sql: string(body)})
	}
	sort.Slice(migrations, func(i, j int) bool { return migrations[i].version < migrations[j].version })
	return migrations, nil
}

// currentVersion reads schema_version, treating a missing table (a brand
// new file) as version 0.
func currentVersion(s *Store) (int, error) {
	var version int
	err := s.db.QueryRow("SELECT version FROM schema_version").Scan(&version)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return 0, nil
		}
		return 0, fmt.Errorf("store: read schema_version: %w", err)
	}
	return version, nil
}

// migrate runs every embedded migration whose version is greater than the
// database's current schema_version, forward-only: a migration at or below
// the current version is never re-run (SPEC.md §9).
func (s *Store) migrate() error {
	migrations, err := loadMigrations()
	if err != nil {
		return err
	}
	current, err := currentVersion(s)
	if err != nil {
		return err
	}
	for _, m := range migrations {
		if m.version <= current {
			continue
		}
		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("store: begin migration %s: %w", m.name, err)
		}
		if _, err := tx.Exec(m.sql); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("store: apply migration %s: %w", m.name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("store: commit migration %s: %w", m.name, err)
		}
	}
	return nil
}
