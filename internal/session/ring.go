package session

import "sync"

// DefaultScrollbackBytes is the ring's default capacity (SPEC.md §3.2),
// used when --scrollback-bytes is left at its default.
const DefaultScrollbackBytes = 256 * 1024

// resetSequence is prepended to every replay so a truncated frame can't
// leave a Client's emulator in a corrupt state (SPEC.md §3.2).
const resetSequence = "\x1b[2J\x1b[H"

// ring is a byte-oriented ring buffer that never splits a UTF-8 rune or an
// escape sequence at the head it hands out on replay (SPEC.md §3.2). A
// capacity of 0 (--scrollback-bytes 0) retains nothing.
type ring struct {
	mu   sync.Mutex
	data []byte
	cap  int
}

func newRing(capBytes int) *ring {
	return &ring{cap: capBytes}
}

// Write appends p, trimming from the head at a safe boundary if the ring
// would exceed its capacity.
func (r *ring) Write(p []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data = append(r.data, p...)
	if len(r.data) > r.cap {
		trim := safeScanForward(r.data, len(r.data)-r.cap)
		r.data = r.data[trim:]
	}
}

// Snapshot returns the current ring contents prefixed with resetSequence,
// or nil if the ring is empty.
func (r *ring) Snapshot() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.data) == 0 {
		return nil
	}
	out := make([]byte, 0, len(resetSequence)+len(r.data))
	out = append(out, resetSequence...)
	out = append(out, r.data...)
	return out
}

// safeScanForward returns the first index at or after from that is not a
// UTF-8 continuation byte and does not fall inside an in-progress escape
// sequence, so replay never starts mid-rune or mid-CSI/OSC. It walks from
// the start of data (rather than from from itself) because whether from
// lands inside a rune or escape sequence can only be told by knowing where
// that unit began. If an escape sequence starting before the end of data
// never finds its terminator (the data was truncated exactly inside it),
// the whole remainder starting at that escape is dropped rather than
// replayed unsafely.
func safeScanForward(data []byte, from int) int {
	i := 0
	for i < len(data) {
		unitEnd := i + 1
		switch b := data[i]; {
		case b == 0x1b: // ESC: the whole sequence it introduces is one unit
			end := escapeSequenceEnd(data, i)
			if end < 0 {
				return len(data)
			}
			unitEnd = end
		case b&0xC0 == 0xC0: // UTF-8 multi-byte lead byte
			unitEnd = i + utf8SeqLen(b)
			if unitEnd > len(data) {
				unitEnd = len(data)
			}
		}

		if i >= from {
			return i
		}
		if unitEnd > from {
			return unitEnd
		}
		i = unitEnd
	}
	return i
}

// utf8SeqLen returns the total byte length of the UTF-8 sequence led by b.
func utf8SeqLen(b byte) int {
	switch {
	case b&0xE0 == 0xC0:
		return 2
	case b&0xF0 == 0xE0:
		return 3
	case b&0xF8 == 0xF0:
		return 4
	default:
		return 1
	}
}

// escapeSequenceEnd returns the index just past the escape sequence
// starting at data[i] (data[i] must be ESC), or -1 if data ends before the
// sequence's terminator is found.
func escapeSequenceEnd(data []byte, i int) int {
	if i+1 >= len(data) {
		return -1
	}
	switch data[i+1] {
	case '[': // CSI: ESC [ ... final byte in 0x40-0x7E
		for j := i + 2; j < len(data); j++ {
			if data[j] >= 0x40 && data[j] <= 0x7e {
				return j + 1
			}
		}
		return -1
	case ']': // OSC: ESC ] ... terminated by BEL or ST (ESC \)
		for j := i + 2; j < len(data); j++ {
			if data[j] == 0x07 {
				return j + 1
			}
			if data[j] == 0x1b && j+1 < len(data) && data[j+1] == '\\' {
				return j + 2
			}
		}
		return -1
	default: // two-byte escape (e.g. ESC ( B)
		return i + 2
	}
}
