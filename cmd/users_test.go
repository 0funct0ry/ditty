package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// withPasswordPrompt overrides passwordPrompter for the duration of a test,
// so users add/passwd can be exercised without a real terminal.
func withPasswordPrompt(t *testing.T, password string) {
	t.Helper()
	prev := passwordPrompter
	passwordPrompter = func(*cobra.Command) (string, error) { return password, nil }
	t.Cleanup(func() { passwordPrompter = prev })
}

func runUsersCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs(append([]string{"users"}, args...))
	err := rootCmd.Execute()
	return buf.String(), err
}

func TestUsersAddListPasswdDisable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "auth.db")
	withPasswordPrompt(t, "hunter22ok")

	if _, err := runUsersCLI(t, "add", "alice", "--role", "operator", "--auth-db", dbPath); err != nil {
		t.Fatalf("users add: %v", err)
	}

	out, err := runUsersCLI(t, "list", "--auth-db", dbPath)
	if err != nil {
		t.Fatalf("users list: %v", err)
	}
	if !strings.Contains(out, "alice") || !strings.Contains(out, "operator") {
		t.Fatalf("users list output missing expected fields:\n%s", out)
	}

	withPasswordPrompt(t, "newpassword1")
	if _, err := runUsersCLI(t, "passwd", "alice", "--auth-db", dbPath); err != nil {
		t.Fatalf("users passwd: %v", err)
	}

	if _, err := runUsersCLI(t, "disable", "alice", "--auth-db", dbPath); err != nil {
		t.Fatalf("users disable: %v", err)
	}
	out, err = runUsersCLI(t, "list", "--auth-db", dbPath)
	if err != nil {
		t.Fatalf("users list after disable: %v", err)
	}
	if !strings.Contains(out, "true") {
		t.Fatalf("expected disabled=true in output:\n%s", out)
	}
}

func TestUsersAdd_RequiresAuthDB(t *testing.T) {
	withPasswordPrompt(t, "hunter22ok")
	if _, err := runUsersCLI(t, "add", "alice"); err == nil {
		t.Fatal("expected an error when --auth-db is not given")
	}
}

func TestUsersAdd_InvalidRoleRejected(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "auth.db")
	withPasswordPrompt(t, "hunter22ok")
	if _, err := runUsersCLI(t, "add", "alice", "--role", "admin", "--auth-db", dbPath); err == nil {
		t.Fatal("expected an error for an unknown role")
	}
}
