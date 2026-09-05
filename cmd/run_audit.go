package cmd

import (
	"github.com/0funct0ry/ditty/internal/security"
	"github.com/0funct0ry/ditty/internal/store"
)

// auditRecord writes one audit row when authStore is configured (--auth-db)
// and is a no-op otherwise, so every audit call site in execRun stays a
// single unconditional line regardless of whether --auth-db was passed.
func auditRecord(authStore *store.Store, actor, action string) {
	if authStore == nil {
		return
	}
	_ = authStore.RecordAudit(actor, action, "")
}

// auditWriteDeniedFunc adapts auditRecord to internal/session's
// OnWriteDenied hook (SPEC.md §9's write_denied audit action).
func auditWriteDeniedFunc(authStore *store.Store) func(clientID, label string) {
	if authStore == nil {
		return nil
	}
	return func(_, label string) { auditRecord(authStore, label, store.AuditWriteDenied) }
}

// auditAttachFunc and auditDetachFunc adapt auditRecord to
// internal/httpapi's WSOptions.OnAttach/OnDetach hooks (SPEC.md §9's
// attach/detach audit actions).
func auditAttachFunc(authStore *store.Store) func(security.Identity) {
	if authStore == nil {
		return nil
	}
	return func(identity security.Identity) { auditRecord(authStore, identity.Label, store.AuditAttach) }
}

func auditDetachFunc(authStore *store.Store) func(security.Identity) {
	if authStore == nil {
		return nil
	}
	return func(identity security.Identity) { auditRecord(authStore, identity.Label, store.AuditDetach) }
}
