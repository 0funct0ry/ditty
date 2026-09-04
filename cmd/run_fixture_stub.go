//go:build !fixture

package cmd

import (
	"net/http"

	"github.com/spf13/pflag"

	"github.com/0funct0ry/ditty/internal/config"
)

// registerFixtureFlags is a no-op in a release build: --fixture does not
// exist outside the "fixture" build tag (see run_fixture.go), so it never
// appears in --help and internal/fixture is never linked (SPEC.md §12 M3).
func registerFixtureFlags(*pflag.FlagSet) {}

// fixtureWSHandler always returns a nil handler outside the "fixture"
// build.
func fixtureWSHandler(*config.Resolver) (http.Handler, error) { return nil, nil }
