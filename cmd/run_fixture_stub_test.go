//go:build !fixture

package cmd

import "testing"

// A release build never compiles run_fixture.go (the "fixture" build tag
// gates it), so --fixture must not exist on the normal binary's flag
// surface, and internal/fixture must not be linked (SPEC.md §12 M3).
func TestFixtureFlagAbsentWithoutBuildTag(t *testing.T) {
	if f := rootCmd.Flags().Lookup("fixture"); f != nil {
		t.Fatalf("--fixture must not be registered without the fixture build tag, got %+v", f)
	}
}
