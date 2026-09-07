package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCliDocSlug(t *testing.T) {
	cases := map[string]string{
		"ditty.md":               "ditty",
		"ditty_run.md":           "run",
		"ditty_users.md":         "users",
		"ditty_users_add.md":     "users-add",
		"/some/dir/ditty_run.md": "run",
	}
	for in, want := range cases {
		if got := cliDocSlug(in); got != want {
			t.Errorf("cliDocSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGenDocsLinkHandler(t *testing.T) {
	if got, want := genDocsLinkHandler("ditty_run.md"), "/reference/cli/run/"; got != want {
		t.Errorf("genDocsLinkHandler = %q, want %q", got, want)
	}
}

func TestGenDocsCommand(t *testing.T) {
	out := t.TempDir()
	rootCmd.SetArgs([]string{"gen-docs", "--out", out})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("gen-docs failed: %v", err)
	}

	entries, err := os.ReadDir(out)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("gen-docs produced no files")
	}

	names := map[string]bool{}
	for _, e := range entries {
		names[e.Name()] = true
		data, err := os.ReadFile(filepath.Join(out, e.Name()))
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", e.Name(), err)
		}
		if !strings.HasPrefix(string(data), "---\ntitle:") {
			t.Errorf("%s missing Starlight frontmatter", e.Name())
		}
	}

	// Hidden commands (gen-docs itself, and every v1.1 stub) never get a
	// generated page: cobra/doc's tree walk skips Hidden commands outright.
	for _, hidden := range []string{"gen-docs.md", "ditty_gen-docs.md", "serve.md", "ditty_serve.md"} {
		if names[hidden] {
			t.Errorf("gen-docs produced a page for hidden command file %q", hidden)
		}
	}

	if !names["run.md"] {
		t.Error("gen-docs did not produce run.md")
	}
}
