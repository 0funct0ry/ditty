package security

import (
	"log/slog"
	"sync"
)

const redactedPlaceholder = "[redacted]"

// redactedKeys are slog attribute keys whose value is always scrubbed,
// regardless of content (SPEC.md §6, CLAUDE.md: never log token values,
// basic-auth credentials, PTY content, or header-env values).
var redactedKeys = map[string]bool{
	"token":         true,
	"password":      true,
	"authorization": true,
	"cookie":        true,
	"basic-auth":    true,
}

// Redactor scrubs known secret values out of log output. Register every
// live secret (the token, the basic-auth pair, header-env values) with it
// at startup; ReplaceAttr then removes both those exact values and any
// attribute under a known-sensitive key, wherever they appear in a log
// record.
type Redactor struct {
	mu      sync.RWMutex
	secrets map[string]bool
}

// NewRedactor returns an empty Redactor.
func NewRedactor() *Redactor {
	return &Redactor{secrets: make(map[string]bool)}
}

// Register adds value to the set of strings ReplaceAttr scrubs. Empty
// strings are ignored (they would otherwise match everything).
func (r *Redactor) Register(value string) {
	if value == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.secrets[value] = true
}

// ReplaceAttr is a slog.HandlerOptions.ReplaceAttr hook: it redacts any
// attribute whose key is known-sensitive, or whose string value exactly
// matches a registered secret.
func (r *Redactor) ReplaceAttr(_ []string, a slog.Attr) slog.Attr {
	if redactedKeys[a.Key] {
		a.Value = slog.StringValue(redactedPlaceholder)
		return a
	}
	if a.Value.Kind() == slog.KindString {
		r.mu.RLock()
		secret := r.secrets[a.Value.String()]
		r.mu.RUnlock()
		if secret {
			a.Value = slog.StringValue(redactedPlaceholder)
		}
	}
	return a
}
