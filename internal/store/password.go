package store

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

// productionBcryptCost is SPEC.md §9's fixed cost for every password hash.
const productionBcryptCost = 12

// bcryptCost returns SPEC.md §9's fixed cost in a real binary, and
// bcrypt.MinCost under `go test`. Every package that exercises a login path
// (this one, internal/security, internal/httpapi, cmd) creates users and
// verifies passwords repeatedly across many tests; at cost 12 each hash or
// compare costs low-hundreds of milliseconds, which adds up to tens of
// seconds across a suite — none of that expense is testing anything SPEC.md
// §9 cares about (the cost itself), so tests skip it via testing.Testing()
// (stdlib, added for exactly this kind of case) rather than a hand-rolled
// seam duplicated in every package that touches a login flow.
func bcryptCost() int {
	if testing.Testing() {
		return bcrypt.MinCost
	}
	return productionBcryptCost
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost())
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func passwordMatches(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
