package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/0funct0ry/ditty/internal/buildinfo"
)

// versionCmd prints buildinfo.Version/Commit/Date. It has its own local
// --json flag, not a persistent one (CLAUDE.md).
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the ditty version",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		asJSON, _ := cmd.Flags().GetBool("json")
		if asJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(map[string]string{
				"version": buildinfo.Version,
				"commit":  buildinfo.Commit,
				"date":    buildinfo.Date,
			})
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ditty %s (commit %s, built %s)\n",
			buildinfo.Version, buildinfo.Commit, buildinfo.Date)
		return nil
	},
}

func init() {
	versionCmd.Flags().BoolP("json", "j", false, "print version info as JSON")
	rootCmd.AddCommand(versionCmd)
}
