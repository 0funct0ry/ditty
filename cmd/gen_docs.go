package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// genDocsCmd renders the CLI reference into Starlight-ready markdown
// (SPEC.md §12 M14). It is a build-time tool, not a user-facing feature —
// hidden, like gen-docs itself and the v1.1 stubs in cmd/stubs.go.
// cobra/doc's tree walk already skips every Hidden command (Cobra's
// IsAvailableCommand excludes them), so gen-docs and the stubs never get a
// generated page without any extra filtering here.
//
// cobra/doc names each file after the full command path ("ditty_run.md"),
// which would route Starlight pages under /reference/cli/ditty_run/ — the
// leading "ditty_" is noise once every page already lives under
// reference/cli/. Generate into a scratch directory first, then rename
// each file to its Starlight slug ("run.md") before moving it into --out.
var genDocsCmd = &cobra.Command{
	Use:    "gen-docs",
	Hidden: true,
	Short:  "Generate CLI reference markdown from the Cobra command tree",
	Args:   cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		out, _ := cmd.Flags().GetString("out")
		scratch, err := os.MkdirTemp("", "ditty-gen-docs-*")
		if err != nil {
			return err
		}
		defer func() { _ = os.RemoveAll(scratch) }()

		if err := doc.GenMarkdownTreeCustom(rootCmd, scratch, genDocsFilePrepender, genDocsLinkHandler); err != nil {
			return err
		}

		if err := os.MkdirAll(out, 0o755); err != nil {
			return err
		}
		existing, err := filepath.Glob(filepath.Join(out, "*.md"))
		if err != nil {
			return err
		}
		for _, f := range existing {
			if err := os.Remove(f); err != nil {
				return err
			}
		}

		entries, err := os.ReadDir(scratch)
		if err != nil {
			return err
		}
		for _, e := range entries {
			slug := cliDocSlug(e.Name())
			data, err := os.ReadFile(filepath.Join(scratch, e.Name()))
			if err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(out, slug+".md"), data, 0o644); err != nil {
				return err
			}
		}
		return nil
	},
}

func init() {
	genDocsCmd.Flags().String("out", "docs/src/content/docs/reference/cli", "output directory for generated markdown")
	rootCmd.AddCommand(genDocsCmd)
}

func genDocsFilePrepender(filename string) string {
	name := cliDocSlug(filename)
	title := "ditty " + strings.ReplaceAll(name, "-", " ")
	if name == "ditty" {
		title = "ditty"
	}
	return fmt.Sprintf("---\ntitle: %s\ndescription: CLI reference for %s, generated from the Cobra command tree.\n---\n\n", title, title)
}

func genDocsLinkHandler(name string) string {
	return "/reference/cli/" + cliDocSlug(name) + "/"
}

// cliDocSlug turns cobra/doc's "ditty_run_users_add.md" filename into a
// Starlight-friendly slug: strips the leading "ditty" segment and the
// extension, joins the rest with hyphens. The root command itself
// ("ditty.md") slugs to "ditty" since there's nothing left to strip.
func cliDocSlug(filename string) string {
	name := strings.TrimSuffix(filepath.Base(filename), ".md")
	parts := strings.Split(name, "_")
	if len(parts) > 0 && parts[0] == "ditty" {
		parts = parts[1:]
	}
	if len(parts) == 0 {
		return "ditty"
	}
	return strings.Join(parts, "-")
}
