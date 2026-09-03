// Package cmd holds ditty's Cobra commands (cobra-cli layout): one file
// per command, no logic beyond flags, validation and a call into
// internal/ (CLAUDE.md).
package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// rootCmd doubles as `ditty run` (SPEC.md §8.2): `ditty [flags] <command>
// [args...]` is a real shortcut into run's own logic, not separately
// parsed sugar. It carries no PersistentFlags — its local flag set is
// registered by the same registerRunFlags function runCmd uses, so both
// forms accept an identical flag surface (CLAUDE.md: no persistent root
// flags).
var rootCmd = &cobra.Command{
	Use:   "ditty [flags] <command> [args...]",
	Short: "Share a terminal over the web from a single binary",
	Long: `ditty shares a terminal over the web from a single Go binary.
One command, one PTY, many browsers. Read-only by default.

  ditty htop                  # share htop, read-only, over 127.0.0.1
  ditty run -- bash -lc '...' # the escape hatch for argv with its own flags

"ditty <command>" is a shortcut for "ditty run -- <command>": it resolves
whenever the first argument is not itself a registered subcommand name.`,
	Args:          cobra.ArbitraryArgs,
	SilenceUsage:  true,
	SilenceErrors: false,
	RunE: func(cmd *cobra.Command, args []string) error {
		return execRun(cmd, cmd.Flags(), args)
	},
}

func init() {
	registerRunFlags(rootCmd.Flags())
}

// Execute runs the root command. Called once from main.main().
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
