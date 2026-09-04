package fixture

import (
	"fmt"
	"time"
)

// EndingKind is how a Scenario's Command finishes.
type EndingKind int

// Ending kinds a Scenario can script.
const (
	EndingNeverending EndingKind = iota
	EndingExit
	EndingSignal
)

// Ending describes how and when a Scenario finishes. At is measured from
// scenario start; zero means "immediately after the last Chunk".
type Ending struct {
	Kind   EndingKind
	At     time.Duration
	Code   int
	Signal string
}

// Chunk is one piece of scripted PTY output, emitted At a fixed offset from
// scenario start.
type Chunk struct {
	At   time.Duration
	Data []byte
}

// Scenario is a timed transcript a Hub plays back in place of a live PTY.
type Scenario struct {
	Name   string
	Cols   int
	Rows   int
	Chunks []Chunk
	Ending Ending
	// DropAt, when non-zero, is when the Hub force-closes every attached
	// Client to simulate a network drop. Chunks scheduled after DropAt
	// keep accumulating into the ring so a reconnecting Client sees them
	// on replay (flaky only).
	DropAt time.Duration
}

// Scenarios returns the four fixture scenarios, keyed by name.
func Scenarios() map[string]Scenario {
	return map[string]Scenario{
		deployScenario.Name:    deployScenario,
		htopScenario.Name:      htopScenario,
		flakyScenario.Name:     flakyScenario,
		quickexitScenario.Name: quickexitScenario,
	}
}

func line(s string) []byte { return []byte(s + "\r\n") }

// deploy is a long, colourful build-and-rollout log. It exercises plain
// append-only output with ANSI colour, the case Phase 2's terminal core
// (M4) leans on most for a first demo.
var deployScenario = Scenario{
	Name: "deploy",
	Cols: 120,
	Rows: 34,
	Chunks: []Chunk{
		{At: 0, Data: line("\x1b[36m==> Building ditty v1.0.0\x1b[0m")},
		{At: 300 * time.Millisecond, Data: line("\x1b[90m$ go build ./...\x1b[0m")},
		{At: 900 * time.Millisecond, Data: line("\x1b[32m✓ compiled in 1.8s\x1b[0m")},
		{At: 1200 * time.Millisecond, Data: line("\x1b[36m==> Running tests\x1b[0m")},
		{At: 2000 * time.Millisecond, Data: line("\x1b[32m✓ 142 tests passed\x1b[0m")},
		{At: 2300 * time.Millisecond, Data: line("\x1b[36m==> Pushing image ghcr.io/0funct0ry/ditty:v1.0.0\x1b[0m")},
		{At: 2600 * time.Millisecond, Data: line("\x1b[90mThe push refers to repository [ghcr.io/0funct0ry/ditty]\x1b[0m")},
		{At: 3400 * time.Millisecond, Data: line("\x1b[32m✓ pushed sha256:9f3ac1d0e77b\x1b[0m")},
		{At: 3700 * time.Millisecond, Data: line("\x1b[36m==> Rolling out to production\x1b[0m")},
		{At: 4200 * time.Millisecond, Data: line("\x1b[33m→ 1/3 replicas updated\x1b[0m")},
		{At: 4700 * time.Millisecond, Data: line("\x1b[33m→ 2/3 replicas updated\x1b[0m")},
		{At: 5200 * time.Millisecond, Data: line("\x1b[32m✓ 3/3 replicas updated · rollout complete\x1b[0m")},
	},
	Ending: Ending{Kind: EndingNeverending},
}

// htop drives continuous full-screen redraws — cursor homing and a screen
// clear on every frame — to stress the renderer against something other
// than append-only output.
var htopScenario = Scenario{
	Name:   "htop",
	Cols:   100,
	Rows:   30,
	Chunks: htopChunks(20, 500*time.Millisecond),
	Ending: Ending{Kind: EndingNeverending},
}

func htopChunks(n int, interval time.Duration) []Chunk {
	chunks := make([]Chunk, n)
	for i := range n {
		chunks[i] = Chunk{At: time.Duration(i) * interval, Data: htopFrame(i)}
	}
	return chunks
}

func htopFrame(tick int) []byte {
	cpu := 10.0 + float64(tick%10)*3.5
	mem := 20.0 + float64((tick*7)%15)
	s := "\x1b[2J\x1b[H"
	s += fmt.Sprintf("top - %02d:%02d:%02d up 1 day, load average: 0.42, 0.38, 0.31\r\n", tick/3600, (tick/60)%60, tick%60)
	s += "Tasks: 118 total,   1 running, 117 sleeping\r\n\r\n"
	s += "  PID USER      PR  NI    VIRT    RES  %CPU  %MEM COMMAND\r\n"
	s += fmt.Sprintf("%5d root      20   0  123456  45678  %4.1f  %4.1f ditty\r\n", 1000+tick, cpu, mem)
	s += fmt.Sprintf("%5d root      20   0   98765  23456  %4.1f  %4.1f bash\r\n", 2000+tick, cpu/2, mem/2)
	return []byte(s)
}

// flaky drops the connection on a timer and resumes emitting afterward, so
// M5 can build the reconnecting state against a real disconnect instead of
// a UI-simulated one.
var flakyScenario = Scenario{
	Name: "flaky",
	Cols: 80,
	Rows: 24,
	Chunks: []Chunk{
		{At: 0, Data: line("\x1b[36mconnected — streaming logs\x1b[0m")},
		{At: 1 * time.Second, Data: line("log line 1")},
		{At: 2 * time.Second, Data: line("log line 2")},
		{At: 4 * time.Second, Data: line("\x1b[33mreconnected — resuming\x1b[0m")},
		{At: 5 * time.Second, Data: line("log line 3")},
		{At: 6 * time.Second, Data: line("log line 4")},
	},
	DropAt: 3 * time.Second,
	Ending: Ending{Kind: EndingNeverending},
}

// quickexit exits non-zero five seconds in, so M5 can build the closed
// state against a real Exit frame.
var quickexitScenario = Scenario{
	Name: "quickexit",
	Cols: 100,
	Rows: 30,
	Chunks: []Chunk{
		{At: 0, Data: line("\x1b[36m==> Building ditty v1.0.0\x1b[0m")},
		{At: 1 * time.Second, Data: line("\x1b[90m$ go build ./...\x1b[0m")},
		{At: 3 * time.Second, Data: line("\x1b[31m✗ build failed: exit status 1\x1b[0m")},
	},
	Ending: Ending{Kind: EndingExit, At: 5 * time.Second, Code: 1},
}
