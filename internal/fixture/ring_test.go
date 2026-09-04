package fixture

import (
	"bytes"
	"testing"
)

func TestSafeScanForward(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		from int
		want int
	}{
		{
			name: "already safe",
			data: []byte("hello"),
			from: 2,
			want: 2,
		},
		{
			name: "split rune skips continuation bytes",
			// "é" is 0xC3 0xA9; trimming into the middle of it must land
			// past both continuation-shaped bytes onto the following 'x'.
			data: append([]byte{0xC3, 0xA9}, 'x'),
			from: 1,
			want: 2,
		},
		{
			name: "split CSI skips to after the final byte",
			data: []byte("\x1b[31mred"),
			from: 2, // inside "31m"
			want: 5, // start of "red"
		},
		{
			name: "split OSC terminated by BEL",
			data: []byte("\x1b]0;title\x07after"),
			from: 3,
			want: 10, // start of "after"
		},
		{
			name: "split OSC terminated by ST",
			data: []byte("\x1b]0;title\x1b\\after"),
			from: 3,
			want: 11,
		},
		{
			name: "truncated escape with no terminator drops the remainder",
			data: []byte("\x1b[3"),
			from: 0,
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := safeScanForward(tt.data, tt.from)
			if got != tt.want {
				t.Fatalf("safeScanForward(%q, %d) = %d, want %d", tt.data, tt.from, got, tt.want)
			}
		})
	}
}

func TestRing_ExactlyFullAndWrapped(t *testing.T) {
	r := newRing(16)
	r.Write(bytes.Repeat([]byte("a"), 16))
	if got := len(r.data); got != 16 {
		t.Fatalf("len after exact fill = %d, want 16", got)
	}

	// Wrap three times over.
	r.Write(bytes.Repeat([]byte("b"), 48))
	if got := len(r.data); got != 16 {
		t.Fatalf("len after wrap = %d, want 16 (cap)", got)
	}
	if !bytes.Equal(r.data, bytes.Repeat([]byte("b"), 16)) {
		t.Fatalf("ring did not retain only the most recent 16 bytes: %q", r.data)
	}
}

func TestRing_SnapshotPrefixesResetSequence(t *testing.T) {
	r := newRing(1024)
	if snap := r.Snapshot(); snap != nil {
		t.Fatalf("empty ring should snapshot to nil, got %q", snap)
	}
	r.Write([]byte("hello"))
	snap := r.Snapshot()
	if !bytes.HasPrefix(snap, []byte(resetSequence)) {
		t.Fatalf("snapshot %q missing reset sequence prefix", snap)
	}
	if !bytes.HasSuffix(snap, []byte("hello")) {
		t.Fatalf("snapshot %q missing written data", snap)
	}
}

func TestRing_ScrollbackZeroKeepsNothing(t *testing.T) {
	r := newRing(0)
	r.Write([]byte("hello"))
	if snap := r.Snapshot(); snap != nil {
		t.Fatalf("--scrollback-bytes 0 should retain nothing, got %q", snap)
	}
}
