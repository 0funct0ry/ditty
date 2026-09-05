package store

import (
	"os"
	"path/filepath"
	"testing"
)

func openTemp(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "auth.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestOpen_EmptyPathRejected(t *testing.T) {
	if _, err := Open(""); err == nil {
		t.Fatal("Open(\"\") should error, not create an unnamed database")
	}
}

func TestOpen_CreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.db")
	if _, err := os.Stat(path); err == nil {
		t.Fatal("file should not exist before Open")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = s.Close() }()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Open should create %s: %v", path, err)
	}
}

func TestOpen_ReopenIsClean(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.db")
	s1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	if _, err := s1.CreateUser("alice", "hunter22", RoleViewer); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := s1.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer func() { _ = s2.Close() }()
	u, found, err := s2.GetUserByUsername("alice")
	if err != nil || !found {
		t.Fatalf("GetUserByUsername after reopen: %v found=%v", err, found)
	}
	if u.Username != "alice" {
		t.Fatalf("Username = %q, want alice", u.Username)
	}
}
