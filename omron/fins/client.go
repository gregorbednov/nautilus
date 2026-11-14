package fins

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

// Transport exchanges exactly one FINS frame. It must never retry a write.
type Transport interface {
	Exchange(context.Context, []byte) ([]byte, error)
	Close() error
}

// Client serializes a request and its response, including the SID. It never
// reconnects or retries automatically: the outcome of a broken write is unknown.
type Client struct {
	transport Transport
	route     Route
	mu        sync.Mutex
	sid       byte
	failed    bool
}

func New(t Transport, route Route) *Client { return &Client{transport: t, route: route} }
func (c *Client) Close() error             { return c.transport.Close() }
func (c *Client) Call(ctx context.Context, command uint16, data []byte) ([]byte, error) {
	return c.callValidated(ctx, command, data, nil)
}

// Payload validation belongs to the same serialized exchange as header
// validation, so another caller cannot reuse a malformed session in between.
func (c *Client) callValidated(ctx context.Context, command uint16, data []byte, validate func([]byte) error) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.failed {
		return nil, errors.New("FINS session failed; reconnect explicitly before another request")
	}
	c.sid++
	req := request(c.route, c.sid, command, data)
	b, err := c.transport.Exchange(ctx, req)
	if err != nil {
		c.failed = true
		_ = c.transport.Close()
		return nil, err
	}
	out, err := response(req, b)
	if err == nil && validate != nil {
		err = validate(out)
		if err != nil {
			out = nil
		}
	}
	var end *EndError
	if err != nil && !errors.As(err, &end) {
		c.failed = true
		_ = c.transport.Close()
	}
	return out, err
}

type networkTransport struct {
	conn    net.Conn
	tcp     bool
	timeout time.Duration
}

func (t *networkTransport) Close() error { return t.conn.Close() }

// withDeadline makes context cancellation interrupt an in-flight network read.
type deadlineStream interface {
	io.ReadWriteCloser
	SetDeadline(time.Time) error
}

func withDeadline(ctx context.Context, conn deadlineStream, timeout time.Duration, fn func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	deadline := time.Now().Add(timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return err
	}
	cancelled := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Now()); close(cancelled) })
	err := fn()
	if !stop() {
		<-cancelled
	}
	_ = conn.SetDeadline(time.Time{})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}
func writeAll(w io.Writer, b []byte) error {
	for len(b) > 0 {
		n, err := w.Write(b)
		if n < 0 || n > len(b) {
			return errors.New("invalid write count")
		}
		b = b[n:]
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}
func (t *networkTransport) Exchange(ctx context.Context, req []byte) (out []byte, err error) {
	err = withDeadline(ctx, t.conn, t.timeout, func() error {
		if t.tcp {
			if e := writeAll(t.conn, tcpFrame(2, req)); e != nil {
				return e
			}
			var e error
			out, e = readTCP(t.conn, 2)
			return e
		}
		n, e := t.conn.Write(req)
		if e != nil {
			return e
		}
		if n != len(req) {
			return io.ErrShortWrite
		}
		b := make([]byte, maxTCPSize)
		n, e = t.conn.Read(b)
		if e != nil {
			return e
		}
		out = b[:n]
		return nil
	})
	return
}

// DialUDP requires explicit FINS nodes; their values must not be guessed from
// the IP address. net.Dial binds responses to the configured UDP peer.
func DialUDP(ctx context.Context, endpoint string, route Route, timeout time.Duration) (*Client, error) {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	if route.Source.Node == 0 || route.Source.Node == 255 || route.Destination.Node == 0 || route.Destination.Node == 255 {
		return nil, errors.New("FINS/UDP needs explicit source and destination nodes 1..254")
	}
	conn, err := (&net.Dialer{Timeout: timeout}).DialContext(ctx, "udp", endpoint)
	if err != nil {
		return nil, err
	}
	return New(&networkTransport{conn: conn, timeout: timeout}, route), nil
}

// DialTCP negotiates FINS node numbers. Source.Node=0 asks the server to assign
// a source node. A configured nonzero node must agree with the handshake.
func DialTCP(ctx context.Context, endpoint string, route Route, timeout time.Duration) (*Client, error) {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	conn, err := (&net.Dialer{Timeout: timeout}).DialContext(ctx, "tcp", endpoint)
	if err != nil {
		return nil, err
	}
	err = withDeadline(ctx, conn, timeout, func() error {
		var node [4]byte
		binary.BigEndian.PutUint32(node[:], uint32(route.Source.Node))
		if err := writeAll(conn, tcpFrame(0, node[:])); err != nil {
			return err
		}
		b, err := readTCP(conn, 1)
		if err != nil {
			return err
		}
		if len(b) != 8 {
			return errors.New("invalid FINS/TCP node handshake")
		}
		src, dst := binary.BigEndian.Uint32(b[:4]), binary.BigEndian.Uint32(b[4:])
		if src < 1 || src > 254 || dst < 1 || dst > 254 {
			return errors.New("invalid assigned FINS/TCP nodes")
		}
		if route.Source.Node != 0 && route.Source.Node != byte(src) {
			return errors.New("FINS/TCP source node differs from configuration")
		}
		if route.Destination.Network == 0 && route.Destination.Node != 0 && route.Destination.Node != byte(dst) {
			return errors.New("FINS/TCP destination node differs from configuration")
		}
		route.Source.Node = byte(src)
		if route.Destination.Network == 0 {
			route.Destination.Node = byte(dst)
		} else if route.Destination.Node == 0 {
			return errors.New("routed FINS/TCP requires an explicit destination node")
		}
		return nil
	})
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return New(&networkTransport{conn: conn, tcp: true, timeout: timeout}, route), nil
}

// Identity preserves the full CPU data response for later unit-version decoding.
type Identity struct {
	Model   string `json:"model"`
	Version string `json:"version"`
	Raw     []byte `json:"raw"`
}

func (c *Client) Identity(ctx context.Context) (Identity, error) {
	b, err := c.callValidated(ctx, 0x0501, nil, func(b []byte) error {
		if len(b) < 40 {
			return errors.New("short CPU UNIT DATA response")
		}
		return nil
	})
	if err != nil {
		return Identity{}, err
	}
	trim := func(b []byte) string { return strings.Trim(string(b), " \x00") }
	return Identity{trim(b[:20]), trim(b[20:40]), b}, nil
}

type Status struct {
	Status   byte   `json:"status"`
	Mode     byte   `json:"mode"`
	Fatal    uint16 `json:"fatal"`
	NonFatal uint16 `json:"nonFatal"`
	Raw      []byte `json:"raw"`
}

func (c *Client) Status(ctx context.Context) (Status, error) {
	b, err := c.callValidated(ctx, 0x0601, nil, func(b []byte) error {
		if len(b) < 26 {
			return errors.New("short CPU UNIT STATUS response")
		}
		if b[1] != 0 && b[1] != 2 && b[1] != 4 {
			return fmt.Errorf("unknown CPU mode %02X", b[1])
		}
		return nil
	})
	if err != nil {
		return Status{}, err
	}
	return Status{Status: b[0], Mode: b[1], Fatal: binary.BigEndian.Uint16(b[2:4]), NonFatal: binary.BigEndian.Uint16(b[4:6]), Raw: b}, nil
}
