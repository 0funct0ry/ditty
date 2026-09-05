package cmd

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/spf13/pflag"

	"github.com/0funct0ry/ditty/internal/config"
	"github.com/0funct0ry/ditty/internal/pty"
)

// newTestResolver builds a *config.Resolver bound to every `ditty run`
// flag at its default value, for testing the pure helpers execRun calls.
func newTestResolver(t *testing.T) *config.Resolver {
	t.Helper()
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	registerRunFlags(fs)
	resolver, err := config.New()
	if err != nil {
		t.Fatalf("config.New: %v", err)
	}
	if err := resolver.BindFlagSet(fs); err != nil {
		t.Fatalf("BindFlagSet: %v", err)
	}
	return resolver
}

func TestBuildCommand_DefaultsAndOverrides(t *testing.T) {
	resolver := newTestResolver(t)
	cmd, err := buildCommand(resolver, []string{"bash", "-i"}, "sess1")
	if err != nil {
		t.Fatalf("buildCommand: %v", err)
	}
	if cmd.SessionID != "sess1" {
		t.Errorf("SessionID = %q, want sess1", cmd.SessionID)
	}
	if len(cmd.Argv) != 2 || cmd.Argv[0] != "bash" {
		t.Errorf("Argv = %v", cmd.Argv)
	}
	if cmd.UID != nil || cmd.GID != nil {
		t.Errorf("UID/GID should be nil when --uid/--gid are empty, got %v/%v", cmd.UID, cmd.GID)
	}
	if cmd.KillSignal == nil {
		t.Error("KillSignal should default to a parsed signal, got nil")
	}
}

func TestBuildCommand_BadUIDErrors(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	registerRunFlags(fs)
	resolver, err := config.New()
	if err != nil {
		t.Fatalf("config.New: %v", err)
	}
	if err := fs.Set("uid", "not-a-number"); err != nil {
		t.Fatalf("fs.Set: %v", err)
	}
	if err := resolver.BindFlagSet(fs); err != nil {
		t.Fatalf("BindFlagSet: %v", err)
	}
	if _, err := buildCommand(resolver, []string{"bash"}, "sess1"); err == nil {
		t.Fatal("expected an error for a non-numeric --uid")
	}
}

func TestParseUint32Ptr(t *testing.T) {
	if v, err := parseUint32Ptr(""); err != nil || v != nil {
		t.Fatalf("empty string: v=%v err=%v, want nil, nil", v, err)
	}
	v, err := parseUint32Ptr("1000")
	if err != nil {
		t.Fatalf("parseUint32Ptr(1000): %v", err)
	}
	if v == nil || *v != 1000 {
		t.Fatalf("parseUint32Ptr(1000) = %v, want 1000", v)
	}
	if _, err := parseUint32Ptr("not-a-number"); err == nil {
		t.Fatal("expected an error for a non-numeric value")
	}
}

func TestDetachGrace_ExitOnDetachForcesZero(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	registerRunFlags(fs)
	resolver, err := config.New()
	if err != nil {
		t.Fatalf("config.New: %v", err)
	}
	if err := fs.Set("detach-grace", "30s"); err != nil {
		t.Fatalf("fs.Set: %v", err)
	}
	if err := fs.Set("exit-on-detach", "true"); err != nil {
		t.Fatalf("fs.Set: %v", err)
	}
	if err := resolver.BindFlagSet(fs); err != nil {
		t.Fatalf("BindFlagSet: %v", err)
	}
	if got := detachGrace(resolver); got != 0 {
		t.Errorf("detachGrace with --exit-on-detach = %v, want 0", got)
	}
}

func TestDetachGrace_UsesFlagWhenNotExitOnDetach(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	registerRunFlags(fs)
	resolver, err := config.New()
	if err != nil {
		t.Fatalf("config.New: %v", err)
	}
	if err := fs.Set("detach-grace", "30s"); err != nil {
		t.Fatalf("fs.Set: %v", err)
	}
	if err := resolver.BindFlagSet(fs); err != nil {
		t.Fatalf("BindFlagSet: %v", err)
	}
	if got := detachGrace(resolver); got != 30*time.Second {
		t.Errorf("detachGrace = %v, want 30s", got)
	}
}

func TestBuildProfile_EncodesResolvedFlags(t *testing.T) {
	resolver := newTestResolver(t)
	raw, err := buildProfile(resolver)
	if err != nil {
		t.Fatalf("buildProfile: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded["theme"] != "ditty-dark" {
		t.Errorf("theme = %v, want ditty-dark", decoded["theme"])
	}
}

func TestExpandTitle(t *testing.T) {
	got := expandTitle("{command} — {hostname}", []string{"bash", "-i"})
	if got == "{command} — {hostname}" {
		t.Fatal("expandTitle did not substitute any placeholder")
	}
}

func TestCompileOriginAllow(t *testing.T) {
	if re, err := compileOriginAllow(""); err != nil || re != nil {
		t.Fatalf("empty pattern: re=%v err=%v, want nil, nil", re, err)
	}
	re, err := compileOriginAllow(`^https?://example\.com$`)
	if err != nil {
		t.Fatalf("compileOriginAllow: %v", err)
	}
	if !re.MatchString("https://example.com") {
		t.Error("compiled regex did not match the expected origin")
	}
	if _, err := compileOriginAllow("("); err == nil {
		t.Fatal("expected an error for an invalid regex")
	}
}

func TestRandomSessionID_Unique(t *testing.T) {
	a, err := randomSessionID()
	if err != nil {
		t.Fatalf("randomSessionID: %v", err)
	}
	b, err := randomSessionID()
	if err != nil {
		t.Fatalf("randomSessionID: %v", err)
	}
	if a == b {
		t.Fatalf("two randomSessionID calls returned the same value: %q", a)
	}
	if len(a) == 0 {
		t.Fatal("randomSessionID returned an empty string")
	}
}

func TestStaticInfo_State(t *testing.T) {
	if got := staticInfo("ready").State(); got != "ready" {
		t.Errorf("State() = %q, want ready", got)
	}
}

// TestActiveSessions_CloseAllClosesEveryTrackedProcess exercises the
// registry ditty's default per-Client mode uses so a SIGINT/SIGTERM
// shutdown closes every independent Command still running, not just the
// one the signal happened to interrupt.
func TestActiveSessions_CloseAllClosesEveryTrackedProcess(t *testing.T) {
	sessions := newActiveSessions()

	var procs []*pty.Process
	for i := 0; i < 3; i++ {
		p, err := pty.Spawn(context.Background(), pty.Command{Argv: []string{"sleep", "30"}})
		if err != nil {
			t.Fatalf("Spawn: %v", err)
		}
		procs = append(procs, p)
		sessions.add(string(rune('a'+i)), p)
	}

	sessions.remove("a") // removed before closeAll; must be closed by the caller, not left running
	defer func() { _ = procs[0].Close() }()

	done := make(chan struct{})
	go func() {
		sessions.closeAll()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("closeAll did not return within 10s")
	}

	for _, p := range procs[1:] {
		waitDone := make(chan struct{})
		go func() {
			_, _ = p.Wait()
			close(waitDone)
		}()
		select {
		case <-waitDone:
		case <-time.After(time.Second):
			t.Fatal("Wait did not return promptly after closeAll — the Command was not actually reaped")
		}
	}
}
