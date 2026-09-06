package cmd

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// healthcheckTimeout bounds the probe request so a wedged server fails the
// container HEALTHCHECK promptly instead of hanging it.
const healthcheckTimeout = 2 * time.Second

// healthcheckCmd is a hidden, container-only probe: distroless images ship
// no shell, curl or wget, so Docker's HEALTHCHECK CMD has nothing to exec
// except the ditty binary itself (SPEC.md §15). It reads DITTY_PORT and
// DITTY_BASE_PATH directly via os.Getenv rather than the full
// config.Resolver/Viper stack, since this runs as a separate, short-lived
// probe process rather than the Session itself. It assumes /healthz is
// reachable over plain HTTP on 127.0.0.1 — a TLS-only listener isn't
// probed generically by this command; document that limitation alongside
// the Dockerfile's HEALTHCHECK line rather than solving it here.
var healthcheckCmd = &cobra.Command{
	Use:    "__healthcheck",
	Hidden: true,
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runHealthcheck()
	},
	SilenceUsage: true,
}

func init() {
	rootCmd.AddCommand(healthcheckCmd)
}

func runHealthcheck() error {
	port := os.Getenv("DITTY_PORT")
	if port == "" {
		port = "7654"
	}
	basePath := os.Getenv("DITTY_BASE_PATH")
	basePath = strings.TrimSuffix(basePath, "/")

	client := &http.Client{Timeout: healthcheckTimeout}
	resp, err := client.Get("http://127.0.0.1:" + port + basePath + "/healthz")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return &healthcheckError{status: resp.StatusCode}
	}
	return nil
}

// healthcheckError reports a non-200 /healthz response; its message is
// only ever seen in `docker inspect`'s health log, not by an interactive
// user, so it stays terse.
type healthcheckError struct{ status int }

func (e *healthcheckError) Error() string {
	return http.StatusText(e.status)
}
