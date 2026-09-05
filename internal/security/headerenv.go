package security

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// headerEnvValueLimit truncates a header-mapped value before it reaches the
// Command's environment (SPEC.md §6.4).
const headerEnvValueLimit = 4096

// headerEnvNamePattern is the required shape of a --header-env target
// (SPEC.md §6.4).
var headerEnvNamePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// blockedEnvTargets are hard-blocked --header-env targets (SPEC.md §6.4):
// naming any of these, or anything with a blocked prefix, is a
// configuration error, not a silently-ignored mapping.
var blockedEnvTargets = map[string]bool{
	"PATH":            true,
	"LD_PRELOAD":      true,
	"LD_LIBRARY_PATH": true,
	"IFS":             true,
	"BASH_ENV":        true,
	"ENV":             true,
	"SHELL":           true,
	"PROMPT_COMMAND":  true,
}

var blockedEnvPrefixes = []string{"DYLD_"}

// HeaderEnvMapping maps one request header to one Command environment
// variable (--header-env X-Foo:DITTY_FOO).
type HeaderEnvMapping struct {
	Header string
	Target string
}

// ParseHeaderEnvMapping validates and parses a single --header-env value.
func ParseHeaderEnvMapping(spec string) (HeaderEnvMapping, error) {
	header, target, ok := strings.Cut(spec, ":")
	if !ok || header == "" || target == "" {
		return HeaderEnvMapping{}, fmt.Errorf("security: --header-env must be Header:TARGET, got %q", spec)
	}
	if !headerEnvNamePattern.MatchString(target) {
		return HeaderEnvMapping{}, fmt.Errorf(
			"security: --header-env target %q must match %s", target, headerEnvNamePattern.String())
	}
	if isBlockedEnvTarget(target) {
		return HeaderEnvMapping{}, fmt.Errorf("security: --header-env target %q is not allowed", target)
	}
	return HeaderEnvMapping{Header: header, Target: target}, nil
}

func isBlockedEnvTarget(target string) bool {
	if blockedEnvTargets[target] {
		return true
	}
	for _, prefix := range blockedEnvPrefixes {
		if strings.HasPrefix(target, prefix) {
			return true
		}
	}
	return false
}

// Resolve reads every mapped header from r and returns the resulting
// KEY=VALUE environment additions, truncating each value to
// headerEnvValueLimit bytes. A header that is absent contributes nothing.
func Resolve(mappings []HeaderEnvMapping, r *http.Request) []string {
	var env []string
	for _, m := range mappings {
		v := r.Header.Get(m.Header)
		if v == "" {
			continue
		}
		if len(v) > headerEnvValueLimit {
			v = v[:headerEnvValueLimit]
		}
		env = append(env, m.Target+"="+v)
	}
	return env
}
