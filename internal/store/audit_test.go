package store

import "testing"

func TestRecordAudit(t *testing.T) {
	s := openTemp(t)
	if err := s.RecordAudit("alice", AuditLogin, `{"role":"operator"}`); err != nil {
		t.Fatalf("RecordAudit: %v", err)
	}

	var actor, action, detail string
	row := s.db.QueryRow(`SELECT actor, action, detail FROM audit ORDER BY id DESC LIMIT 1`)
	if err := row.Scan(&actor, &action, &detail); err != nil {
		t.Fatalf("scan audit row: %v", err)
	}
	if actor != "alice" || action != AuditLogin || detail != `{"role":"operator"}` {
		t.Fatalf("got (%q, %q, %q)", actor, action, detail)
	}
}

func TestRecordAudit_EmptyActorBecomesDash(t *testing.T) {
	s := openTemp(t)
	if err := s.RecordAudit("", AuditStart, ""); err != nil {
		t.Fatalf("RecordAudit: %v", err)
	}
	var actor string
	row := s.db.QueryRow(`SELECT actor FROM audit ORDER BY id DESC LIMIT 1`)
	if err := row.Scan(&actor); err != nil {
		t.Fatalf("scan audit row: %v", err)
	}
	if actor != "-" {
		t.Fatalf("actor = %q, want -", actor)
	}
}
