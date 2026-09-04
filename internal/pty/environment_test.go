package pty

import (
	"strings"
	"testing"
)

func lookupEnv(env []string, key string) (string, bool) {
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok && k == key {
			return v, true
		}
	}
	return "", false
}

func countKey(env []string, key string) int {
	n := 0
	for _, kv := range env {
		if k, _, ok := strings.Cut(kv, "="); ok && k == key {
			n++
		}
	}
	return n
}

func TestBuildEnvStripsDittyVars(t *testing.T) {
	t.Setenv("DITTY_PORT", "9999")
	env := buildEnv(Command{SessionID: "s1"})
	if _, ok := lookupEnv(env, "DITTY_PORT"); ok {
		t.Fatal("DITTY_PORT leaked into the child environment")
	}
}

func TestBuildEnvSetsSessionID(t *testing.T) {
	env := buildEnv(Command{SessionID: "abc123"})
	v, ok := lookupEnv(env, "DITTY_SESSION")
	if !ok || v != "abc123" {
		t.Fatalf("DITTY_SESSION = %q, %v; want %q, true", v, ok, "abc123")
	}
}

func TestBuildEnvDefaultTerm(t *testing.T) {
	env := buildEnv(Command{})
	v, ok := lookupEnv(env, "TERM")
	if !ok || v != "xterm-256color" {
		t.Fatalf("TERM = %q, %v; want %q, true", v, ok, "xterm-256color")
	}
}

func TestBuildEnvDedupesTerm(t *testing.T) {
	t.Setenv("TERM", "vt100")

	env := buildEnv(Command{Env: []string{"TERM=screen-256color"}})

	if n := countKey(env, "TERM"); n != 1 {
		t.Fatalf("TERM appears %d times in child env, want exactly 1", n)
	}
	v, _ := lookupEnv(env, "TERM")
	if v != "screen-256color" {
		t.Fatalf("TERM = %q, want %q (Command.Env should win over the parent's)", v, "screen-256color")
	}
}

func TestBuildEnvEnvOverridesParent(t *testing.T) {
	t.Setenv("MY_VAR", "parent")
	env := buildEnv(Command{Env: []string{"MY_VAR=child"}})
	if n := countKey(env, "MY_VAR"); n != 1 {
		t.Fatalf("MY_VAR appears %d times, want exactly 1", n)
	}
	v, _ := lookupEnv(env, "MY_VAR")
	if v != "child" {
		t.Fatalf("MY_VAR = %q, want %q", v, "child")
	}
}
