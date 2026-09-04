package fixture

import "sync"

// fakeClient is an in-memory Client used to drive a Hub in tests without a
// real WebSocket.
type fakeClient struct {
	id    string
	label string

	mu     sync.Mutex
	frames [][]byte
	closed bool
}

func newFakeClient(id string) *fakeClient {
	return &fakeClient{id: id, label: "Client " + id}
}

func (c *fakeClient) ID() string    { return c.id }
func (c *fakeClient) Label() string { return c.label }

func (c *fakeClient) Send(frame []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := make([]byte, len(frame))
	copy(cp, frame)
	c.frames = append(c.frames, cp)
	return nil
}

func (c *fakeClient) Close(int, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
}

func (c *fakeClient) snapshot() [][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([][]byte, len(c.frames))
	copy(out, c.frames)
	return out
}

func (c *fakeClient) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}
