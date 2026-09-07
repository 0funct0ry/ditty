package wire

import "encoding/json"

// Resize is the Client -> Server payload for OpResize.
type Resize struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

// Resize clamp bounds per §5.1.
const (
	MinCols = 10
	MaxCols = 1000
	MinRows = 5
	MaxRows = 500
)

// Clamp returns r with Cols and Rows clamped into the valid range.
func (r Resize) Clamp() Resize {
	return Resize{Cols: clampInt(r.Cols, MinCols, MaxCols), Rows: clampInt(r.Rows, MinRows, MaxRows)}
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// Hello is the Server -> Client payload for OpHello, always the first
// frame sent after a successful upgrade.
type Hello struct {
	Protocol string          `json:"protocol"`
	Server   string          `json:"server"`
	Session  HelloSession    `json:"session"`
	Client   HelloClient     `json:"client"`
	Policy   HelloPolicy     `json:"policy"`
	Profile  json.RawMessage `json:"profile,omitempty"`
}

// HelloSession describes the Session a Hello frame reports on.
type HelloSession struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Title     string `json:"title"`
	Cols      int    `json:"cols"`
	Rows      int    `json:"rows"`
	State     string `json:"state"`
	StartedAt string `json:"startedAt"`
	// Shared reports whether this Session is running in --shared mode (one
	// Command/PTY fanned out to many Clients) rather than ditty's default
	// of one fresh Command per Client (SPEC.md §1). The UI uses this to
	// decide whether a Client-count badge is meaningful at all: in
	// non-shared mode every Session has exactly one Client, always, so
	// there is nothing a count could ever tell the Client that isn't
	// already implied by being connected.
	Shared bool `json:"shared"`
}

// HelloClient describes the receiving Client itself.
type HelloClient struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Writable bool   `json:"writable"`
	Sizing   bool   `json:"sizing"`
}

// HelloPolicy carries session-wide policy the Client should honor.
type HelloPolicy struct {
	Writable          bool   `json:"writable"`
	Reconnect         bool   `json:"reconnect"`
	ReconnectInterval string `json:"reconnectInterval"`
	UnloadWarning     bool   `json:"unloadWarning"`
	MaxClients        int    `json:"maxClients"`
	ProfileLock       bool   `json:"profileLock"`
	// Focus reports --focus: hide the chrome bar and status bar entirely,
	// leaving only the terminal itself. Unlike ProfileLock (which hides one
	// control), this is a full-chrome policy, so the UI must hide both bars
	// outright rather than disabling something inside them.
	Focus bool `json:"focus"`
}

// Roster is the Server -> Client payload for OpRoster.
type Roster struct {
	Clients []RosterClient `json:"clients"`
	Count   int            `json:"count"`
}

// RosterClient describes one Client in a Roster frame.
type RosterClient struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Writable bool   `json:"writable"`
	JoinedAt string `json:"joinedAt"`
}

// Session state values valid in State.State.
const (
	SessionStateLive     = "live"
	SessionStateDetached = "detached"
	SessionStateClosed   = "closed"
)

// State is the Server -> Client payload for OpState.
type State struct {
	Writable bool   `json:"writable"`
	Sizing   bool   `json:"sizing"`
	State    string `json:"state"`
	Reason   string `json:"reason"`
}

// Exit is the Server -> Client payload for OpExit.
type Exit struct {
	Code    int    `json:"code"`
	Signal  string `json:"signal"`
	Message string `json:"message"`
}

// Notice levels valid in Notice.Level.
const (
	NoticeLevelInfo  = "info"
	NoticeLevelWarn  = "warn"
	NoticeLevelError = "error"
)

// Notice is the Server -> Client payload for OpNotice.
type Notice struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}
