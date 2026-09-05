package store

import (
	"path/filepath"
	"testing"
)

func TestMigrate_EmptyFileEndsAtCurrentVersion(t *testing.T) {
	s := openTemp(t)
	v, err := currentVersion(s)
	if err != nil {
		t.Fatalf("currentVersion: %v", err)
	}
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations: %v", err)
	}
	want := migrations[len(migrations)-1].version
	if v != want {
		t.Fatalf("version = %d, want %d", v, want)
	}
}

func TestMigrate_AlreadyAtVersionIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.db")
	s1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	v1, err := currentVersion(s1)
	if err != nil {
		t.Fatalf("currentVersion: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	s2, err := Open(path) // already at v1's schema_version; must not re-run 0001_init.sql
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer func() { _ = s2.Close() }()
	v2, err := currentVersion(s2)
	if err != nil {
		t.Fatalf("currentVersion: %v", err)
	}
	if v2 != v1 {
		t.Fatalf("version after reopen = %d, want %d", v2, v1)
	}
}

func TestLoadMigrations_SortedByVersion(t *testing.T) {
	migrations, err := loadMigrations()
	if err != nil {
		t.Fatalf("loadMigrations: %v", err)
	}
	if len(migrations) == 0 {
		t.Fatal("expected at least one migration")
	}
	for i := 1; i < len(migrations); i++ {
		if migrations[i-1].version >= migrations[i].version {
			t.Fatalf("migrations not strictly increasing at index %d: %d >= %d",
				i, migrations[i-1].version, migrations[i].version)
		}
	}
}
