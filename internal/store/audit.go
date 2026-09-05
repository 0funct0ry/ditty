package store

import (
	"fmt"
	"time"
)

// Audit actions (SPEC.md §9): login, login_failed, attach, detach,
// write_denied, start, exit.
const (
	AuditLogin       = "login"
	AuditLoginFailed = "login_failed"
	AuditAttach      = "attach"
	AuditDetach      = "detach"
	AuditWriteDenied = "write_denied"
	AuditStart       = "start"
	AuditExit        = "exit"
)

// RecordAudit appends one row to the audit table (SPEC.md §9). actor is a
// username, grant label, or "-"; detail is a JSON blob (or "" for none) and
// must never contain a password, token, or cookie value (CLAUDE.md).
func (s *Store) RecordAudit(actor, action, detail string) error {
	if actor == "" {
		actor = "-"
	}
	_, err := s.db.Exec(
		`INSERT INTO audit (ts, actor, action, detail) VALUES (?, ?, ?, ?)`,
		time.Now().UTC().Format(time.RFC3339), actor, action, detail,
	)
	if err != nil {
		return fmt.Errorf("store: record audit: %w", err)
	}
	return nil
}

// AllAuditDetails returns every audit row's detail blob, for tests scanning
// for accidentally-logged secrets (CLAUDE.md: never log token values,
// basic-auth credentials, PTY content, or header-env values).
func (s *Store) AllAuditDetails() ([]string, error) {
	rows, err := s.db.Query(`SELECT detail FROM audit`)
	if err != nil {
		return nil, fmt.Errorf("store: list audit details: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var details []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, fmt.Errorf("store: scan audit detail: %w", err)
		}
		details = append(details, d)
	}
	return details, rows.Err()
}

// CountAuditAction counts audit rows matching actor (or "-" for any actor
// when actor == "") and action, for tests asserting a given event was
// recorded.
func (s *Store) CountAuditAction(actor, action string) (int, error) {
	var n int
	var err error
	if actor == "" {
		err = s.db.QueryRow(`SELECT COUNT(*) FROM audit WHERE action = ?`, action).Scan(&n)
	} else {
		err = s.db.QueryRow(`SELECT COUNT(*) FROM audit WHERE actor = ? AND action = ?`, actor, action).Scan(&n)
	}
	if err != nil {
		return 0, fmt.Errorf("store: count audit: %w", err)
	}
	return n, nil
}
