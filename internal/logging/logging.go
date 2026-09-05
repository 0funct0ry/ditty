// Package logging builds the log/slog.Logger every command uses, honouring
// -v/-q, --log-format and --log-file (SPEC.md §8.1). It is a plain function
// call from each command's RunE — there is no persistent root flag or
// PersistentPreRun to hang it off (CLAUDE.md: no persistent root flags).
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
)

// Options controls logger construction. Format is "text" or "json";
// anything else is a usage error. File is a path, or "" for stderr.
type Options struct {
	Verbosity int // repeat count of -v; 0 = info, 1+ = debug
	Quiet     bool
	Format    string
	File      string
	// ReplaceAttr, when non-nil, is installed as the slog.HandlerOptions
	// hook of the same name — internal/security's Redactor uses this to
	// scrub secret values out of every log record (CLAUDE.md: never log
	// token values, basic-auth credentials, PTY content, or header-env
	// values).
	ReplaceAttr func(groups []string, a slog.Attr) slog.Attr
}

// New builds a slog.Logger per Options and returns it along with the
// io.Closer for --log-file (a no-op closer for stderr), so the caller can
// defer Close().
func New(opts Options) (*slog.Logger, io.Closer, error) {
	level := slog.LevelInfo
	switch {
	case opts.Quiet:
		level = slog.LevelWarn
	case opts.Verbosity > 0:
		level = slog.LevelDebug
	}

	var out io.Writer = os.Stderr
	var closer io.Closer = nopCloser{}
	if opts.File != "" {
		f, err := os.OpenFile(opts.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, nil, fmt.Errorf("logging: open --log-file %q: %w", opts.File, err)
		}
		out = f
		closer = f
	}

	handlerOpts := &slog.HandlerOptions{Level: level, ReplaceAttr: opts.ReplaceAttr}
	var handler slog.Handler
	switch opts.Format {
	case "", "text":
		handler = slog.NewTextHandler(out, handlerOpts)
	case "json":
		handler = slog.NewJSONHandler(out, handlerOpts)
	default:
		return nil, nil, fmt.Errorf("logging: --log-format must be text or json, got %q", opts.Format)
	}

	return slog.New(handler), closer, nil
}

type nopCloser struct{}

func (nopCloser) Close() error { return nil }
