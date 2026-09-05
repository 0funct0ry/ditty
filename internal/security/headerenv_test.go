package security

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseHeaderEnvMapping_BlocksDangerousTargets(t *testing.T) {
	blocked := []string{
		"X-Foo:PATH",
		"X-Foo:LD_PRELOAD",
		"X-Foo:LD_LIBRARY_PATH",
		"X-Foo:DYLD_INSERT_LIBRARIES",
		"X-Foo:IFS",
		"X-Foo:BASH_ENV",
		"X-Foo:ENV",
		"X-Foo:SHELL",
		"X-Foo:PROMPT_COMMAND",
	}
	for _, spec := range blocked {
		if _, err := ParseHeaderEnvMapping(spec); err == nil {
			t.Errorf("ParseHeaderEnvMapping(%q) = nil error, want error", spec)
		}
	}
}

func TestParseHeaderEnvMapping_NamePattern(t *testing.T) {
	bad := []string{"X-Foo:lower", "X-Foo:1START", "X-Foo:HAS-DASH", "X-Foo:"}
	for _, spec := range bad {
		if _, err := ParseHeaderEnvMapping(spec); err == nil {
			t.Errorf("ParseHeaderEnvMapping(%q) = nil error, want error", spec)
		}
	}

	m, err := ParseHeaderEnvMapping("X-Foo:DITTY_FOO")
	if err != nil {
		t.Fatalf("ParseHeaderEnvMapping(valid) = %v, want nil", err)
	}
	if m.Header != "X-Foo" || m.Target != "DITTY_FOO" {
		t.Fatalf("ParseHeaderEnvMapping(valid) = %+v", m)
	}
}

func TestResolve_TruncatesLongValues(t *testing.T) {
	m, err := ParseHeaderEnvMapping("X-Foo:DITTY_FOO")
	if err != nil {
		t.Fatalf("ParseHeaderEnvMapping: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Foo", strings.Repeat("a", headerEnvValueLimit+100))

	env := Resolve([]HeaderEnvMapping{m}, req)
	if len(env) != 1 {
		t.Fatalf("Resolve() = %v, want one entry", env)
	}
	value := strings.TrimPrefix(env[0], "DITTY_FOO=")
	if len(value) != headerEnvValueLimit {
		t.Fatalf("Resolve() value length = %d, want %d", len(value), headerEnvValueLimit)
	}
}
