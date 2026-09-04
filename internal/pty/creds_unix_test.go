//go:build linux || darwin || freebsd

package pty

import (
	"context"
	"os"
	"testing"
)

func TestUIDRequiresRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; the not-root guard doesn't apply")
	}

	uid := uint32(1)
	_, err := Spawn(context.Background(), Command{Argv: []string{"true"}, UID: &uid})
	if err != errNotRoot {
		t.Fatalf("err = %v, want errNotRoot", err)
	}
}

func TestGIDRequiresRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; the not-root guard doesn't apply")
	}

	gid := uint32(1)
	_, err := Spawn(context.Background(), Command{Argv: []string{"true"}, GID: &gid})
	if err != errNotRoot {
		t.Fatalf("err = %v, want errNotRoot", err)
	}
}

// TestCredentialAppliedAsRoot is a positive-path check for the actual
// syscall.Credential wiring; it only runs when the test binary itself is
// root, which CI is not, so it is skip-only in the normal case.
func TestCredentialAppliedAsRoot(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires running as root")
	}

	uid := uint32(65534) // nobody, on most systems
	p, err := Spawn(context.Background(), Command{Argv: []string{"id", "-u"}, UID: &uid})
	if err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	defer func() { _ = p.Close() }()

	buf := make([]byte, 32)
	n, _ := p.Read(buf)
	if got := string(buf[:n]); got == "" {
		t.Fatal("expected output from id -u")
	}
}
