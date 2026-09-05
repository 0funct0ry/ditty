package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/0funct0ry/ditty/internal/browser"
	"github.com/0funct0ry/ditty/internal/buildinfo"
	"github.com/0funct0ry/ditty/internal/config"
	"github.com/0funct0ry/ditty/internal/httpapi"
	"github.com/0funct0ry/ditty/internal/logging"
)

// runCmd starts a Session and serves it over the web. internal/pty (M8)
// can spawn a Command and internal/session (M9) can drive a real Hub from
// it, but wiring either into a live HTTP/WebSocket transport is internal/
// httpapi's job (M10) — until then this stands up the HTTP shell only:
// /healthz and the embedded UI. Command argv after `--` is accepted but
// not yet run.
var runCmd = &cobra.Command{
	Use:   "run [flags] -- <command> [args...]",
	Short: "Start a Session and share it over the web",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return execRun(cmd, cmd.Flags(), args)
	},
}

func init() {
	registerRunFlags(runCmd.Flags())
	rootCmd.AddCommand(runCmd)
}

// registerRunFlags defines every `ditty run` flag on fs. It is called
// against both runCmd's own flag set and rootCmd's own flag set (CLAUDE.md:
// no persistent flags), so `ditty run -w bash` and the bare `ditty -w bash`
// shortcut (§8.2) parse an identical flag surface without either command
// inheriting the other's flags implicitly. Short letters claimed here:
// p a b o v q f L t F s c k R C e l d E T U G N i y z Z x W g X K S u m M w
// (see SPEC.md §8.1). Note shorthands are case-sensitive single runes, so
// e.g. "t" (profile-theme) and "T" (term) are distinct and do not collide.
func registerRunFlags(fs *pflag.FlagSet) {
	fs.IntP("port", "p", 7654, "port to listen on (0 = random, printed at startup)")
	fs.StringP("address", "a", "127.0.0.1", "address to bind")
	fs.StringP("base-path", "b", "/", "URL path prefix to mount the UI and API under")
	fs.BoolP("open", "o", false, "open the default browser once the server starts")
	fs.CountP("verbose", "v", "increase log verbosity (repeatable)")
	fs.BoolP("quiet", "q", false, "suppress all but warning/error logs")
	fs.StringP("log-format", "f", "text", "log output format: text or json")
	fs.StringP("log-file", "L", "", "write logs to this file instead of stderr")
	registerProfileFlags(fs)
	registerSessionFlags(fs)
	registerCommandFlags(fs)
}

// registerProfileFlags defines the SPEC.md §7 Profile-seeding flags (M6).
// Nothing consumes these yet outside a "fixture" build — internal/session
// (M9) is what will seed the real Hub's Hello.Profile from them.
func registerProfileFlags(fs *pflag.FlagSet) {
	fs.StringP("profile-theme", "t", "ditty-dark",
		"terminal theme: ditty-dark, ditty-light, nord, dracula, solarized-dark, monokai")
	fs.StringP("profile-font-family", "F", "JetBrains Mono, SF Mono, Menlo, monospace", "terminal font family")
	fs.IntP("profile-font-size", "s", 14, "terminal font size in pixels")
	fs.StringP("profile-cursor-style", "c", "block", "cursor style: block, underline, bar")
	fs.BoolP("profile-cursor-blink", "k", true, "whether the cursor blinks")
	fs.StringP("profile-renderer", "R", "webgl", "terminal renderer: webgl, canvas")
	fs.BoolP("profile-copy-on-select", "C", true, "copy selected text to the clipboard automatically")
	fs.StringP("profile-bell", "e", "none", "bell style: none, sound, visual")
	fs.BoolP("profile-lock", "l", false, "force these Profile values and hide the settings drawer entirely")
}

func execRun(cmd *cobra.Command, fs *pflag.FlagSet, args []string) error {
	resolver, err := config.New()
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	if err := resolver.BindFlagSet(fs); err != nil {
		return fmt.Errorf("run: %w", err)
	}

	logger, closer, err := logging.New(logging.Options{
		Verbosity: resolver.Int("verbose"),
		Quiet:     resolver.Bool("quiet"),
		Format:    resolver.String("log-format"),
		File:      resolver.String("log-file"),
	})
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	defer func() { _ = closer.Close() }()

	if len(args) > 0 {
		logger.Debug("Command argv accepted but not yet run — Session/Hub wiring arrives in M9", "argv", args)
	}

	basePath := resolver.String("base-path")
	// ws stays nil until internal/httpapi's real transport (M10) drives an
	// internal/session Hub; internal/session itself already exists (M9).
	handler, err := httpapi.NewHandler(basePath, nil)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}

	addr := net.JoinHostPort(resolver.String("address"), strconv.Itoa(resolver.Int("port")))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("run: listen on %s: %w", addr, err)
	}

	url := fmt.Sprintf("http://%s%s", ln.Addr().String(), basePath)
	logger.Info("ditty starting", "version", buildinfo.Version, "url", url)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ditty  %s\n  url  %s\n", buildinfo.Version, url)

	if resolver.Bool("open") {
		if err := browser.Open(url); err != nil {
			logger.Warn("could not open browser", "error", err)
		}
	}

	server := &http.Server{Handler: handler}
	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve(ln) }()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
