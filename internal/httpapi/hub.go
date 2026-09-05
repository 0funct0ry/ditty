package httpapi

import "github.com/0funct0ry/ditty/internal/session"

// This file is the one place in internal/httpapi allowed to import
// internal/session: it bridges internal/session's real Hub (M9) to the
// httpapi.Hub/httpapi.Client interfaces NewWSHandler is written against, so
// ws.go itself stays free of that import (see Client's doc comment).

// sessionClientAdapter satisfies internal/session's Client interface over
// an httpapi.Client, adding the write capability internal/session's Client
// needs but httpapi.Client does not carry. Real per-Client write capability
// (Grants, roles) arrives in M11/M12; until then every Client's Writable is
// the Session-wide --writable flag.
type sessionClientAdapter struct {
	httpClient Client
	writable   bool
}

func (a sessionClientAdapter) ID() string                    { return a.httpClient.ID() }
func (a sessionClientAdapter) Label() string                 { return a.httpClient.Label() }
func (a sessionClientAdapter) Writable() bool                { return a.writable }
func (a sessionClientAdapter) Send(frame []byte) error       { return a.httpClient.Send(frame) }
func (a sessionClientAdapter) Close(code int, reason string) { a.httpClient.Close(code, reason) }

// sessionHubAdapter bridges *session.Hub to httpapi.Hub.
type sessionHubAdapter struct {
	hub      *session.Hub
	writable bool
}

// NewSessionHub wraps hub as an httpapi.Hub, admitting every attached
// Client with the Session-wide write capability writable (SPEC.md §6.1 I1;
// --writable).
func NewSessionHub(hub *session.Hub, writable bool) Hub {
	return sessionHubAdapter{hub: hub, writable: writable}
}

func (a sessionHubAdapter) Attach(c Client) error {
	return a.hub.Attach(sessionClientAdapter{httpClient: c, writable: a.writable})
}
func (a sessionHubAdapter) Detach(id string)                 { a.hub.Detach(id) }
func (a sessionHubAdapter) Input(id string, data []byte)     { a.hub.Input(id, data) }
func (a sessionHubAdapter) Resize(id string, cols, rows int) { a.hub.Resize(id, cols, rows) }
