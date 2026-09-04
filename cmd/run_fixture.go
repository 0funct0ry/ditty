//go:build fixture

package cmd

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/spf13/pflag"

	"github.com/0funct0ry/ditty/internal/config"
	"github.com/0funct0ry/ditty/internal/fixture"
	"github.com/0funct0ry/ditty/internal/httpapi"
)

// registerFixtureFlags adds the dev-only --fixture flag. It is gated by the
// "fixture" build tag (see run_fixture_stub.go) so a release build never
// links internal/fixture and never shows --fixture in --help (SPEC.md §12
// M3).
func registerFixtureFlags(fs *pflag.FlagSet) {
	fs.String("fixture", "", "dev only: serve a scripted fixture scenario instead of a real PTY ("+scenarioNames()+")")
}

// fixtureWSHandler builds the /ws handler for --fixture, or returns a nil
// handler when the flag is unset.
func fixtureWSHandler(resolver *config.Resolver) (http.Handler, error) {
	name := resolver.String("fixture")
	if name == "" {
		return nil, nil
	}
	scenario, ok := fixture.Scenarios()[name]
	if !ok {
		return nil, fmt.Errorf("unknown fixture scenario %q (available: %s)", name, scenarioNames())
	}
	hub := fixture.NewHub(scenario)
	return httpapi.NewWSHandler(fixtureHubAdapter{hub: hub}), nil
}

// fixtureHubAdapter bridges *fixture.Hub to httpapi.Hub. The two packages
// declare structurally-identical Client interfaces but neither imports the
// other (httpapi must not link internal/fixture outside this build-tagged
// file), so this adapter is what lets an httpapi.Client value flow into
// fixture.Hub's fixture.Client-typed methods.
type fixtureHubAdapter struct{ hub *fixture.Hub }

func (a fixtureHubAdapter) Attach(c httpapi.Client) error    { return a.hub.Attach(c) }
func (a fixtureHubAdapter) Detach(id string)                 { a.hub.Detach(id) }
func (a fixtureHubAdapter) Input(id string, data []byte)     { a.hub.Input(id, data) }
func (a fixtureHubAdapter) Resize(id string, cols, rows int) { a.hub.Resize(id, cols, rows) }

func scenarioNames() string {
	scenarios := fixture.Scenarios()
	names := make([]string, 0, len(scenarios))
	for name := range scenarios {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
