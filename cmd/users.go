package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/0funct0ry/ditty/internal/config"
	"github.com/0funct0ry/ditty/internal/store"
)

// usersCmd manages --auth-db users (SPEC.md §9), real from M12. It replaces
// the stub entry cmd/stubs.go carried for it through M1-M11.
var usersCmd = &cobra.Command{
	Use:   "users",
	Short: "Manage --auth-db users",
}

func init() {
	usersCmd.AddCommand(usersAddCmd, usersListCmd, usersPasswdCmd, usersDisableCmd)
	rootCmd.AddCommand(usersCmd)
}

// registerAuthDBFlag defines --auth-db on fs. Every users subcommand owns
// its own local flag set (CLAUDE.md), so this is called once per
// subcommand rather than shared via a persistent flag.
func registerAuthDBFlag(fs *pflag.FlagSet) {
	fs.String("auth-db", "", "path to the --auth-db SQLite file (required)")
}

// passwordPrompter is promptNewPassword by default; tests override it to
// supply a password without a real terminal.
var passwordPrompter = promptNewPassword

func openAuthDB(resolver *config.Resolver) (*store.Store, error) {
	path := resolver.String("auth-db")
	if path == "" {
		return nil, fmt.Errorf("--auth-db is required")
	}
	return store.Open(path)
}

var usersAddCmd = &cobra.Command{
	Use:   "add <username>",
	Short: "Add a new user, prompting for a password",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		resolver, err := config.New()
		if err != nil {
			return fmt.Errorf("users add: %w", err)
		}
		if err := resolver.BindFlagSet(cmd.Flags()); err != nil {
			return fmt.Errorf("users add: %w", err)
		}
		role := resolver.String("role")
		if role != store.RoleViewer && role != store.RoleOperator {
			return fmt.Errorf("--role must be %q or %q, got %q", store.RoleViewer, store.RoleOperator, role)
		}
		password, err := passwordPrompter(cmd)
		if err != nil {
			return fmt.Errorf("users add: %w", err)
		}
		s, err := openAuthDB(resolver)
		if err != nil {
			return fmt.Errorf("users add: %w", err)
		}
		defer func() { _ = s.Close() }()
		if _, err := s.CreateUser(args[0], password, role); err != nil {
			return fmt.Errorf("users add: %w", err)
		}
		cmd.Printf("user %q added (role: %s)\n", args[0], role)
		return nil
	},
}

func init() {
	fs := usersAddCmd.Flags()
	registerAuthDBFlag(fs)
	fs.StringP("role", "r", store.RoleViewer, "role: viewer or operator")
}

var usersListCmd = &cobra.Command{
	Use:   "list",
	Short: "List every user",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		resolver, err := config.New()
		if err != nil {
			return fmt.Errorf("users list: %w", err)
		}
		if err := resolver.BindFlagSet(cmd.Flags()); err != nil {
			return fmt.Errorf("users list: %w", err)
		}
		s, err := openAuthDB(resolver)
		if err != nil {
			return fmt.Errorf("users list: %w", err)
		}
		defer func() { _ = s.Close() }()
		users, err := s.ListUsers()
		if err != nil {
			return fmt.Errorf("users list: %w", err)
		}
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(w, "USERNAME\tROLE\tDISABLED\tLAST LOGIN"); err != nil {
			return err
		}
		for _, u := range users {
			lastLogin := "-"
			if u.LastLoginAt != nil {
				lastLogin = u.LastLoginAt.Format("2006-01-02T15:04:05Z")
			}
			if _, err := fmt.Fprintf(w, "%s\t%s\t%v\t%s\n", u.Username, u.Role, u.Disabled, lastLogin); err != nil {
				return err
			}
		}
		return w.Flush()
	},
}

func init() {
	registerAuthDBFlag(usersListCmd.Flags())
}

var usersPasswdCmd = &cobra.Command{
	Use:   "passwd <username>",
	Short: "Change a user's password, prompting for it",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		resolver, err := config.New()
		if err != nil {
			return fmt.Errorf("users passwd: %w", err)
		}
		if err := resolver.BindFlagSet(cmd.Flags()); err != nil {
			return fmt.Errorf("users passwd: %w", err)
		}
		password, err := passwordPrompter(cmd)
		if err != nil {
			return fmt.Errorf("users passwd: %w", err)
		}
		s, err := openAuthDB(resolver)
		if err != nil {
			return fmt.Errorf("users passwd: %w", err)
		}
		defer func() { _ = s.Close() }()
		if err := s.SetPassword(args[0], password); err != nil {
			return fmt.Errorf("users passwd: %w", err)
		}
		cmd.Printf("password updated for %q\n", args[0])
		return nil
	},
}

func init() {
	registerAuthDBFlag(usersPasswdCmd.Flags())
}

var usersDisableCmd = &cobra.Command{
	Use:   "disable <username>",
	Short: "Disable a user's account",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		resolver, err := config.New()
		if err != nil {
			return fmt.Errorf("users disable: %w", err)
		}
		if err := resolver.BindFlagSet(cmd.Flags()); err != nil {
			return fmt.Errorf("users disable: %w", err)
		}
		s, err := openAuthDB(resolver)
		if err != nil {
			return fmt.Errorf("users disable: %w", err)
		}
		defer func() { _ = s.Close() }()
		if err := s.SetDisabled(args[0], true); err != nil {
			return fmt.Errorf("users disable: %w", err)
		}
		cmd.Printf("user %q disabled\n", args[0])
		return nil
	},
}

func init() {
	registerAuthDBFlag(usersDisableCmd.Flags())
}
