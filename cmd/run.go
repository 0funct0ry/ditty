package cmd

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/0funct0ry/ditty/internal/browser"
	"github.com/0funct0ry/ditty/internal/buildinfo"
	"github.com/0funct0ry/ditty/internal/config"
	"github.com/0funct0ry/ditty/internal/httpapi"
	"github.com/0funct0ry/ditty/internal/logging"
	"github.com/0funct0ry/ditty/internal/profile"
	"github.com/0funct0ry/ditty/internal/pty"
	"github.com/0funct0ry/ditty/internal/security"
	"github.com/0funct0ry/ditty/internal/session"
	"github.com/0funct0ry/ditty/internal/store"
)

// runCmd starts a Session and serves it over the web: it spawns the
// Command after `--` inside a PTY (internal/pty, M8), drives it through a
// real Hub (internal/session, M9), and serves that Hub over the real HTTP/
// WebSocket transport (internal/httpapi, M10).
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
// p a b o v q f L t F s c k R C e l j d E T U G N i y z Z x W g X K S u m M w
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
	registerTransportFlags(fs)
	registerSecurityFlags(fs)
}

// registerProfileFlags defines the SPEC.md §7 Profile-seeding flags (M6),
// which execRun JSON-encodes into every new Hub's Hello.Profile.
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
	fs.BoolP("focus", "j", false, "hide the chrome bar and status bar entirely, leaving only the terminal")
}

func execRun(cmd *cobra.Command, fs *pflag.FlagSet, args []string) error {
	resolver, err := config.New()
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	if err := resolver.BindFlagSet(fs); err != nil {
		return fmt.Errorf("run: %w", err)
	}

	secCfg, secrets, err := resolveSecurityFlags(resolver)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	redactor := security.NewRedactor()
	for _, s := range secrets {
		redactor.Register(s)
	}

	logger, closer, err := logging.New(logging.Options{
		Verbosity:   resolver.Int("verbose"),
		Quiet:       resolver.Bool("quiet"),
		Format:      resolver.String("log-format"),
		File:        resolver.String("log-file"),
		ReplaceAttr: redactor.ReplaceAttr,
	})
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	defer func() { _ = closer.Close() }()

	var authStore *store.Store
	if secCfg.authDBPath != "" {
		authStore, err = store.Open(secCfg.authDBPath)
		if err != nil {
			return fmt.Errorf("run: --auth-db: %w", err)
		}
		defer func() { _ = authStore.Close() }()
	}

	if len(args) == 0 {
		return fmt.Errorf("run: no Command given; usage: %s", cmd.Use)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	profileJSON, err := buildProfile(resolver)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}

	sessionID, err := randomSessionID()
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}
	name := resolver.String("name")
	if name == "" {
		name = shortSessionName(sessionID)
	}
	title := expandTitle(resolver.String("title"), args)
	writable := resolver.Bool("writable")
	shared := resolver.Bool("shared")
	once := resolver.Bool("once")

	// Every piece of state the startup banner needs — address, Grants, TLS,
	// the listener itself — is resolved and validated up front, so the
	// banner can print before anything else happens: no Command is spawned
	// and no log line is emitted until after it's on screen (see
	// printBanner's own doc comment). This also means a bind-guard failure
	// is now caught before --shared would otherwise have already spawned a
	// Command for nothing.
	originAllow, err := compileOriginAllow(resolver.String("origin-allow"))
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}

	basePath := resolver.String("base-path")
	addr := net.JoinHostPort(resolver.String("address"), strconv.Itoa(resolver.Int("port")))

	grants, tokenGrant, jwtGrant, deferredWarnings, err := buildGrants(secCfg, basePath, addr, authStore)
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}

	if err := security.BindGuard(addr, false, args[0], len(grants) > 0, secCfg.insecureNoAuth); err != nil {
		cmd.SilenceErrors = true
		cmd.PrintErrln(err.Error())
		return errSilent
	}

	var tlsConfig *tls.Config
	if secCfg.tlsCert != "" || secCfg.tlsKey != "" {
		if secCfg.tlsCert == "" || secCfg.tlsKey == "" {
			return fmt.Errorf("run: --tls-cert and --tls-key must be given together")
		}
		tlsConfig, err = security.TLSConfig(secCfg.tlsCert, secCfg.tlsKey, secCfg.clientCA)
		if err != nil {
			return fmt.Errorf("run: %w", err)
		}
	}

	var ln net.Listener
	if tlsConfig != nil {
		ln, err = tls.Listen("tcp", addr, tlsConfig)
	} else {
		ln, err = net.Listen("tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("run: listen on %s: %w", addr, err)
	}

	normalizedBasePath := basePath
	if normalizedBasePath == "" {
		normalizedBasePath = "/"
	}
	if !strings.HasSuffix(normalizedBasePath, "/") {
		normalizedBasePath += "/"
	}
	scheme := "http"
	if tlsConfig != nil {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s%s", scheme, ln.Addr().String(), normalizedBasePath)
	if tokenGrant != nil {
		// SPEC.md §4.1: the banner's URL is the complete shareable one,
		// token included, so zero-config stays zero-config (§6.1 I3).
		url = fmt.Sprintf("%s://%s%st/%s", scheme, ln.Addr().String(), normalizedBasePath, tokenGrant.Token())
	}

	// The banner is the one thing printed before ditty does anything else.
	// Every client count is 0 here — this prints before the listener has
	// accepted its first connection.
	printBanner(cmd.OutOrStdout(), bannerInfo{
		version:   buildinfo.Version,
		sessionID: sessionID,
		command:   args[0],
		url:       url,
		access:    accessSummary(writable, grantNames(grants), 0),
	})
	for _, w := range deferredWarnings {
		logger.Warn(w.msg, w.args...)
	}
	if secCfg.insecureNoAuth {
		go warnInsecureNoAuth(ctx, logger)
	}

	// onceDone fires once --once's single Client's Command has ended,
	// regardless of --shared, so ditty itself can exit the way gotty's
	// --once does — not just close that one Session and keep the HTTP
	// server running for nobody.
	onceDone := make(chan struct{})
	var onceDoneOnce sync.Once
	signalOnceDone := func() { onceDoneOnce.Do(func() { close(onceDone) }) }

	var server *http.Server
	var shutdown func() error
	var hubFactory httpapi.HubFactory
	var info httpapi.SessionInfo

	if shared {
		// Today's pre-M10 model: one Command, one Hub, shared by every
		// Client for the whole `ditty run` invocation (SPEC.md §1 — opt in
		// with --shared for real terminal sharing/pairing).
		command, err := buildCommand(resolver, args, sessionID)
		if err != nil {
			return fmt.Errorf("run: %w", err)
		}
		process, err := pty.Spawn(ctx, command)
		if err != nil {
			return fmt.Errorf("run: spawn Command: %w", err)
		}
		logger.Info("Command spawned", "argv", args, "session", sessionID, "shared", true)

		hub := session.NewHub(session.Options{
			Process:         process,
			ID:              sessionID,
			Name:            name,
			Title:           title,
			Shared:          true,
			Cols:            resolver.Int("cols"),
			Rows:            resolver.Int("rows"),
			Server:          buildinfo.Version,
			ScrollbackBytes: resolver.Int("scrollback-bytes"),
			ChunkBytes:      resolver.Int("chunk-bytes"),
			FlushInterval:   resolver.Duration("flush-interval"),
			MaxClients:      resolver.Int("max-clients"),
			Once:            once,
			WaitForClient:   resolver.Duration("wait-for-client"),
			DetachGrace:     detachGrace(resolver),
			ExitOnDetach:    resolver.Bool("exit-on-detach"),
			Profile:         profileJSON,
			ProfileLock:     resolver.Bool("profile-lock"),
			Focus:           resolver.Bool("focus"),
			OnWriteDenied:   auditWriteDeniedFunc(authStore),
		})
		info = hub
		auditRecord(authStore, "-", store.AuditStart)
		// --header-env has no single request to resolve against in --shared
		// mode (the one Command is already spawned above, before any Client
		// connects), so it applies only in ditty's default per-Client mode
		// below — consistent with every other per-Client Command knob
		// (--uid, --cwd, ...) also being fixed for --shared's one Command.
		hubFactory = func(_ []string, identity security.Identity) (httpapi.Hub, error) {
			return httpapi.NewSessionHub(hub, writable && security.RoleAllowsWrite(identity.Role)), nil
		}
		go func() {
			<-hub.Done()
			auditRecord(authStore, "-", store.AuditExit)
			if once {
				signalOnceDone()
			}
		}()
		shutdown = func() error { return gracefulShutdown(server, process, hub) }
	} else {
		// ditty's default: every Client gets its own fresh Command, spawned
		// on connect and torn down the moment that Client disconnects —
		// one Client's Command exiting never touches any other Client, and
		// a page refresh always starts a brand-new Command (no reconnect/
		// replay across a fresh connection). --once still governs whether
		// ditty accepts more than one Client, ever, across the whole run.
		info = staticInfo("ready")
		sessions := newActiveSessions()
		var used atomic.Bool
		hubFactory = func(headerEnv []string, identity security.Identity) (httpapi.Hub, error) {
			if once && !used.CompareAndSwap(false, true) {
				return nil, errors.New("ditty: --once already served its one Client")
			}
			connID, err := randomSessionID()
			if err != nil {
				return nil, err
			}
			command, err := buildCommand(resolver, args, connID)
			if err != nil {
				return nil, err
			}
			command.Env = append(command.Env, headerEnv...)
			process, err := pty.Spawn(ctx, command)
			if err != nil {
				return nil, fmt.Errorf("spawn Command: %w", err)
			}
			logger.Info("Command spawned", "argv", args, "session", connID, "shared", false)

			hub := session.NewHub(session.Options{
				Process:         process,
				ID:              connID,
				Name:            name,
				Title:           title,
				Cols:            resolver.Int("cols"),
				Rows:            resolver.Int("rows"),
				Server:          buildinfo.Version,
				ScrollbackBytes: resolver.Int("scrollback-bytes"),
				ChunkBytes:      resolver.Int("chunk-bytes"),
				FlushInterval:   resolver.Duration("flush-interval"),
				// Always tear down once this Client leaves: per-Client mode
				// has no reconnect-to-the-same-Command semantics.
				Once: true,
				// Shared is left false (the zero value): this branch only
				// runs without --shared.
				Profile:       profileJSON,
				ProfileLock:   resolver.Bool("profile-lock"),
				Focus:         resolver.Bool("focus"),
				OnWriteDenied: auditWriteDeniedFunc(authStore),
			})
			auditRecord(authStore, identity.Label, store.AuditStart)
			sessions.add(connID, process)
			go func() {
				<-hub.Done()
				auditRecord(authStore, identity.Label, store.AuditExit)
				sessions.remove(connID)
				if once {
					signalOnceDone()
				}
			}()
			return httpapi.NewSessionHub(hub, writable && security.RoleAllowsWrite(identity.Role)), nil
		}
		shutdown = func() error {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			err := server.Shutdown(shutdownCtx)
			sessions.closeAll()
			return err
		}
	}

	handler, err := httpapi.NewRouter(httpapi.Options{
		BasePath:     basePath,
		HubFactory:   hubFactory,
		Info:         info,
		PingInterval: resolver.Duration("ping-interval"),
		OriginAllow:  originAllow,
		AllowIframe:  resolver.String("allow-iframe"),
		SessionID:    sessionID,
		SessionName:  name,
		SessionTitle: title,
		Server:       buildinfo.Version,
		Writable:     writable,
		Profile:      profileJSON,
		Grants:       grants,
		TokenGrant:   tokenGrant,
		JWTGrant:     jwtGrant,
		HeaderEnv:    secCfg.headerEnvMappings,
		OnAttach:     auditAttachFunc(authStore),
		OnDetach:     auditDetachFunc(authStore),
	})
	if err != nil {
		return fmt.Errorf("run: %w", err)
	}

	logger.Info("ditty starting", "version", buildinfo.Version, "url", url)

	if resolver.Bool("open") {
		if err := browser.Open(url); err != nil {
			logger.Warn("could not open browser", "error", err)
		}
	}

	server = &http.Server{Handler: handler}
	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve(ln) }()

	select {
	case <-ctx.Done():
		logger.Info("shutting down")
		return shutdown()
	case <-onceDone:
		logger.Info("shutting down: --once served its one Client")
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

// staticInfo reports a fixed lifecycle state for /api/session and /healthz
// in ditty's default per-Client mode, where no single Session's state
// speaks for the whole run — there may be zero, one, or many independent
// Commands alive at once, each its own Session.
type staticInfo string

func (s staticInfo) State() string { return string(s) }

// activeSessions tracks every per-Client mode Command currently running,
// so a SIGINT/SIGTERM shutdown can close all of them rather than leaking
// child processes when ditty itself exits.
type activeSessions struct {
	mu    sync.Mutex
	procs map[string]*pty.Process
}

func newActiveSessions() *activeSessions {
	return &activeSessions{procs: make(map[string]*pty.Process)}
}

func (a *activeSessions) add(id string, p *pty.Process) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.procs[id] = p
}

func (a *activeSessions) remove(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.procs, id)
}

// closeAll closes every currently tracked Command. Process.Close blocks
// until its Command is reaped, so this is called with the lock released.
func (a *activeSessions) closeAll() {
	a.mu.Lock()
	procs := make([]*pty.Process, 0, len(a.procs))
	for _, p := range a.procs {
		procs = append(procs, p)
	}
	a.mu.Unlock()
	for _, p := range procs {
		_ = p.Close()
	}
}

// gracefulShutdown stops server from accepting new connections, then closes
// process — which drives internal/session's existing exit path: it
// broadcasts Exit to every still-attached Client before the Session itself
// closes (SPEC.md §3), so shutdown needs no separate Notice-then-close API
// of its own. It returns once both the listener and the Session have
// finished closing.
func gracefulShutdown(server *http.Server, process *pty.Process, hub *session.Hub) error {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	shutdownErr := server.Shutdown(shutdownCtx)
	_ = process.Close()
	<-hub.Done()
	return shutdownErr
}

// buildCommand assembles a pty.Command from the resolved Command flags
// (SPEC.md §8.1).
func buildCommand(resolver *config.Resolver, argv []string, sessionID string) (pty.Command, error) {
	uid, err := parseUint32Ptr(resolver.String("uid"))
	if err != nil {
		return pty.Command{}, fmt.Errorf("--uid: %w", err)
	}
	gid, err := parseUint32Ptr(resolver.String("gid"))
	if err != nil {
		return pty.Command{}, fmt.Errorf("--gid: %w", err)
	}
	killSignal, err := pty.ParseSignal(resolver.String("kill-signal"))
	if err != nil {
		return pty.Command{}, fmt.Errorf("--kill-signal: %w", err)
	}

	return pty.Command{
		Argv:       argv,
		Cwd:        resolver.String("cwd"),
		Env:        resolver.StringSlice("env"),
		Term:       resolver.String("term"),
		UID:        uid,
		GID:        gid,
		KillSignal: killSignal,
		Cols:       uint16(resolver.Int("cols")),
		Rows:       uint16(resolver.Int("rows")),
		SessionID:  sessionID,
	}, nil
}

// parseUint32Ptr parses s as a uint32, returning nil for an empty string
// ("don't set this credential").
func parseUint32Ptr(s string) (*uint32, error) {
	if s == "" {
		return nil, nil
	}
	v, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return nil, err
	}
	v32 := uint32(v)
	return &v32, nil
}

// detachGrace resolves --detach-grace, honouring --exit-on-detach's
// shorthand for a zero grace period (SPEC.md §3.1).
func detachGrace(resolver *config.Resolver) time.Duration {
	if resolver.Bool("exit-on-detach") {
		return 0
	}
	return resolver.Duration("detach-grace")
}

// buildProfile applies the resolved --profile-* flags onto
// internal/profile's default Profile and JSON-encodes it into the shape
// internal/session seeds every Hello.Profile with (SPEC.md §7).
func buildProfile(resolver *config.Resolver) (json.RawMessage, error) {
	p := profile.Default()
	p.Theme = resolver.String("profile-theme")
	p.FontFamily = resolver.String("profile-font-family")
	p.FontSize = resolver.Int("profile-font-size")
	p.CursorStyle = resolver.String("profile-cursor-style")
	p.CursorBlink = resolver.Bool("profile-cursor-blink")
	p.Renderer = resolver.String("profile-renderer")
	p.CopyOnSelect = resolver.Bool("profile-copy-on-select")
	p.BellStyle = resolver.String("profile-bell")
	return p.Marshal()
}

// expandTitle expands --title's {command}/{hostname} placeholders. args is
// the Command's own argv, i.e. everything after `--`.
func expandTitle(template string, args []string) string {
	hostname, _ := os.Hostname()
	replacer := strings.NewReplacer(
		"{command}", strings.Join(args, " "),
		"{hostname}", hostname,
	)
	return replacer.Replace(template)
}

// compileOriginAllow compiles --origin-allow's regex, or returns nil for an
// empty pattern (the same-host default).
func compileOriginAllow(pattern string) (*regexp.Regexp, error) {
	if pattern == "" {
		return nil, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("--origin-allow: %w", err)
	}
	return re, nil
}

// randomSessionID generates a Session's own ID (SPEC.md §1): 16 hex
// characters, used unconditionally (regardless of --name) as the
// DITTY_SESSION env var, the Hub's Hello.session.id, log correlation, and,
// in ditty's default per-Client mode, the key each connection's spawned
// Command is tracked under while it's attached — every one of those needs
// the full entropy above to actually avoid collisions.
func randomSessionID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// shortSessionName derives the fallback Hello.session.name shown in the UI
// when --name is not given: the first 4 characters of the Session's own ID,
// uppercased for readability (e.g. "5a72427b8c6ad5c1" -> "5A72"). This is
// display-only — a prefix of the real ID, not a second identifier — so a
// coincidental match between two unrelated ditty processes' short names is
// purely cosmetic (two browser tabs could show the same label) and affects
// nothing the real, full-entropy sessionID is relied on for.
func shortSessionName(sessionID string) string {
	const shortNameLen = 4
	if len(sessionID) < shortNameLen {
		return strings.ToUpper(sessionID)
	}
	return strings.ToUpper(sessionID[:shortNameLen])
}
