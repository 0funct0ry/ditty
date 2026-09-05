package session

import "sync"

// fakeClient is an in-memory Client used to drive a Hub in tests without a
// real WebSocket.
type fakeClient struct {
	id       string
	label    string
	writable bool

	mu      sync.Mutex
	frames  [][]byte
	closed  bool
	blocked chan struct{} // when non-nil, Send blocks until closed
}

func newFakeClient(id string) *fakeClient {
	return &fakeClient{id: id, label: "Client " + id}
}

func newWritableFakeClient(id string) *fakeClient {
	c := newFakeClient(id)
	c.writable = true
	return c
}

func (c *fakeClient) ID() string     { return c.id }
func (c *fakeClient) Label() string  { return c.label }
func (c *fakeClient) Writable() bool { return c.writable }

func (c *fakeClient) Send(frame []byte) error {
	c.mu.Lock()
	blocked := c.blocked
	c.mu.Unlock()
	if blocked != nil {
		<-blocked
	}

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

// block makes every subsequent Send hang until unblock is called.
func (c *fakeClient) block() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.blocked = make(chan struct{})
}

func (c *fakeClient) unblock() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.blocked != nil {
		close(c.blocked)
		c.blocked = nil
	}
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
