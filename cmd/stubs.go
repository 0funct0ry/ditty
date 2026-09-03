package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// v1.1+ subcommand names, reserved now so muscle memory doesn't break
// later (SPEC.md §8.2). `users` is real from M12; `serve`, `ls`, `kill`,
// `attach`, `record`, `play`, `doctor` are v1.1 (SPEC.md §12). Each is
// hidden and exits non-zero explaining itself — none has its own flags
// yet, so there is nothing to register per subcommand.
var stubCommands = []struct {
	name      string
	milestone string
}{
	{"users", "M12"},
	{"serve", "M15"},
	{"ls", "M16"},
	{"kill", "M16"},
	{"attach", "M16"},
	{"record", "M19"},
	{"play", "M19"},
	{"doctor", "M16"},
}

func init() {
	for _, s := range stubCommands {
		s := s
		rootCmd.AddCommand(&cobra.Command{
			Use:    s.name,
			Hidden: true,
			RunE: func(cmd *cobra.Command, _ []string) error {
				return fmt.Errorf("ditty %s: not available yet — arrives in %s, see the roadmap in README.md", s.name, s.milestone)
			},
			SilenceUsage: true,
		})
	}
}
