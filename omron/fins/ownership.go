package fins

import "errors"

var ErrClientTransferred = errors.New("FINS client ownership was transferred")

// ConnectionBinding is observed connection metadata, not device authentication.
// Peer is populated only for built-in network transports; source nodes may
// legitimately differ on a new negotiated TCP session.
type ConnectionBinding struct {
	Transport string `json:"transport"`
	Peer      string `json:"peer"`
	Route     Route  `json:"route"`
}

func (c *Client) Binding() ConnectionBinding {
	if c == nil {
		return ConnectionBinding{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	b := ConnectionBinding{Transport: "custom", Route: c.route}
	if t, ok := c.transport.(*networkTransport); ok {
		b.Transport = "udp"
		if t.tcp {
			b.Transport = "tcp"
		}
		b.Peer = t.conn.RemoteAddr().String()
	}
	return b
}

// TakeExclusive transfers the existing connection and SID sequence to a new
// client, permanently retiring this handle. It waits for an in-flight exchange.
// Old Call/Close handles cannot interleave with or close the new owner. It
// does not reconnect or issue a PLC command. A racing pre-transfer Close may
// still interrupt I/O; it cannot cause an extra request or successful transfer.
func (c *Client) TakeExclusive() (*Client, error) {
	if c == nil {
		return nil, errors.New("nil FINS client")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.transferred.Load() {
		return nil, ErrClientTransferred
	}
	if c.failed || c.closed.Load() || c.transport == nil {
		return nil, errors.New("FINS client is not usable for ownership transfer")
	}
	owned := &Client{transport: c.transport, route: c.route, sid: c.sid}
	c.transferred.Store(true)
	return owned, nil
}
