// Package browser launches the platform default browser for --open, with
// no dependency beyond os/exec.
package browser

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Open launches the default browser at url. It does not wait for the
// browser process and treats a failure to launch as non-fatal to the
// caller — the caller should log it, not exit.
func Open(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("browser: launch %s: %w", runtime.GOOS, err)
	}
	return nil
}
