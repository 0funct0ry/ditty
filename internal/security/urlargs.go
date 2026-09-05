package security

import (
	"fmt"
	"regexp"
)

// DefaultArgPattern is --arg-pattern's default (SPEC.md §6.4).
const DefaultArgPattern = `^[A-Za-z0-9._/-]{1,64}$`

// MaxURLArgs caps how many ?arg= values URLArgFilter.Filter will append,
// regardless of how many were given (SPEC.md §6.4).
const MaxURLArgs = 8

// URLArgFilter implements --allow-url-args' argv filter: off by default,
// and narrow even when enabled. Values are appended to argv directly and
// are never passed through a shell.
type URLArgFilter struct {
	enabled bool
	pattern *regexp.Regexp
}

// NewURLArgFilter builds a URLArgFilter. When enabled is false, Filter
// always returns no args regardless of input. pattern overrides
// DefaultArgPattern when non-empty.
func NewURLArgFilter(enabled bool, pattern string) (*URLArgFilter, error) {
	if pattern == "" {
		pattern = DefaultArgPattern
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("security: --arg-pattern: %w", err)
	}
	return &URLArgFilter{enabled: enabled, pattern: re}, nil
}

// Filter returns the subset of values that match the configured pattern, up
// to MaxURLArgs, in order. It returns nil when the filter is disabled.
func (f *URLArgFilter) Filter(values []string) []string {
	if !f.enabled {
		return nil
	}
	var out []string
	for _, v := range values {
		if len(out) >= MaxURLArgs {
			break
		}
		if f.pattern.MatchString(v) {
			out = append(out, v)
		}
	}
	return out
}
