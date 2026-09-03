package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/0funct0ry/ditty/internal/config"
)

// configCmd groups `ditty config init|show`. Both subcommands own their
// own local flags (CLAUDE.md: no persistent flags) — configCmd itself
// takes none.
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage ditty configuration",
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Write a fully commented ditty.yaml to the current directory",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		path, _ := cmd.Flags().GetString("output")
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("config init: %s already exists — remove it first", path)
		}
		return os.WriteFile(path, []byte(configTemplate), 0o644)
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print the effective configuration",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		resolved, _ := cmd.Flags().GetBool("resolved")

		// A fresh flag set carrying the same definitions `run` uses, so
		// `config show` reports the same keys `ditty run` would resolve —
		// registerRunFlags is the one place those keys are defined.
		runFlags := runCmd.Flags()
		resolver, err := config.New()
		if err != nil {
			return fmt.Errorf("config show: %w", err)
		}
		if err := resolver.BindFlagSet(runFlags); err != nil {
			return fmt.Errorf("config show: %w", err)
		}

		tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
		for _, key := range resolver.Keys() {
			if resolved {
				_, _ = fmt.Fprintf(tw, "%s\t%v\t(%s)\n", key, resolverValue(resolver, key), resolver.Origin(key))
			} else {
				_, _ = fmt.Fprintf(tw, "%s\t%v\n", key, resolverValue(resolver, key))
			}
		}
		return tw.Flush()
	},
}

func resolverValue(r *config.Resolver, key string) any {
	f := runCmd.Flags().Lookup(key)
	if f == nil {
		return r.String(key)
	}
	switch f.Value.Type() {
	case "int", "count":
		return r.Int(key)
	case "bool":
		return r.Bool(key)
	default:
		return r.String(key)
	}
}

func init() {
	configInitCmd.Flags().StringP("output", "O", "ditty.yaml", "path to write")
	configShowCmd.Flags().BoolP("resolved", "r", false, "show each value's origin: default, config file, env, or flag")
	configCmd.AddCommand(configInitCmd, configShowCmd)
	rootCmd.AddCommand(configCmd)
}

const configTemplate = `# ditty configuration — see https://github.com/0funct0ry/ditty for docs.
# Precedence (lowest to highest): these defaults -> this file -> environment
# (DITTY_ prefix) -> command-line flags. Every key here has an env twin,
# e.g. "port" -> DITTY_PORT.

# port: 7654
# address: 127.0.0.1
# base-path: /
# open: false
# log-format: text
# log-file: ""
`
