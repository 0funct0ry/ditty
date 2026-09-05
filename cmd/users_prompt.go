package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// minPasswordLength is a floor against empty/trivial passwords; SPEC.md §9
// leaves password policy unspecified beyond bcrypt cost 12, so this is a
// minimal sanity check, not a full policy.
const minPasswordLength = 8

// promptNewPassword reads a new password twice (enter, confirm) from stdin
// without echoing it, refusing to run at all when stdin is not a terminal
// (SPEC.md §9: users add/passwd never accept a --password flag).
func promptNewPassword(cmd *cobra.Command) (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", fmt.Errorf("stdin is not a terminal; passwords must be entered interactively, not passed as a flag")
	}

	cmd.Print("Password: ")
	first, err := term.ReadPassword(fd)
	cmd.Println()
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	if len(first) < minPasswordLength {
		return "", fmt.Errorf("password must be at least %d characters", minPasswordLength)
	}

	cmd.Print("Confirm password: ")
	second, err := term.ReadPassword(fd)
	cmd.Println()
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	if string(first) != string(second) {
		return "", fmt.Errorf("passwords do not match")
	}
	return string(first), nil
}
