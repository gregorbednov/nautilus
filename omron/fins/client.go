package fins

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
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
	transport   Transport
	route       Route
	mu          sync.Mutex
	sid         byte
	failed      bool
	transferred atomic.Bool
	closed      atomic.Bool
}

func New(t Transport, route Route) *Client { return &Client{transport: t, route: route} }
func (c *Client) Close() error {
	if c.transferred.Load() {
		return ErrClientTransferred
	}
	if c.closed.Swap(true) {
		return nil
	}
	return c.transport.Close()
}
func (c *Client) Call(ctx context.Context, command uint16, data []byte) ([]byte, error) {
	return c.callValidated(ctx, command, data, nil)
}

// Payload validation belongs to the same serialized exchange as header
// validation, so another caller cannot reuse a malformed session in between.
func (c *Client) callValidated(ctx context.Context, command uint16, data []byte, validate func([]byte) error) ([]byte, error) {
	return c.callWithValidation(ctx, command, data, validate, 0)
}

func (c *Client) callWithValidation(ctx context.Context, command uint16, data []byte, validate func([]byte) error, nonTerminalEnd uint16) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.transferred.Load() {
		return nil, ErrClientTransferred
	}
	if c.failed || c.closed.Load() {
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
	// The CJ1 CPU can include complete 0601 DATA with its alarm end code.
	// Decode it for diagnostics while retaining the failed exchange for callers.
	if errors.As(err, &end) && command == 0x0601 && end.Code == 0x0080 {
		status, decodeErr := DecodeStatusData(b[14:])
		if decodeErr == nil {
			err = &StatusAlarm{End: end, Status: status}
		}
	}
	// Only decode owner data after response() verified route/SID/command/header.
	if errors.As(err, &end) && command == 0x0c01 && end.Code == 0x3001 {
		err = accessRightBusyError(end, b[14:])
	}
	// A mutating control/write end code terminates the session too. Reads retain
	// ordinary end-code handling, e.g. bounded 0306 range fallback.
	if err != nil && (end == nil || (terminalCommand(command) && (nonTerminalEnd == 0 || end.Code != nonTerminalEnd))) {
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
	var identity Identity
	_, err := c.callValidated(ctx, 0x0501, nil, func(raw []byte) error {
		var err error
		identity, err = DecodeIdentityData(raw)
		return err
	})
	if err != nil {
		return Identity{}, err
	}
	return identity, nil
}

type Status struct {
	Status   byte   `json:"status"`
	Mode     byte   `json:"mode"`
	Fatal    uint16 `json:"fatal"`
	NonFatal uint16 `json:"nonFatal"`
	Raw      []byte `json:"raw"`
}

// Executing reports the user program execution bit in CPU UNIT STATUS READ.
func (s Status) Executing() bool { return s.Status&1 != 0 }

// StatusAlarm carries validated CPU status data supplied with an alarm end code.
type StatusAlarm struct {
	End    *EndError
	Status Status
}

func (a *StatusAlarm) Unwrap() error { return a.End }

func (a *StatusAlarm) Error() string {
	s := a.Status
	return fmt.Sprintf("%v (status=%02X mode=%02X fatal=%04X nonFatal=%04X errorCode=%04X executing=%t)",
		a.End, s.Status, s.Mode, s.Fatal, s.NonFatal, binary.BigEndian.Uint16(s.Raw[8:10]), s.Executing())
}

func (c *Client) Status(ctx context.Context) (Status, error) {
	var status Status
	_, err := c.callValidated(ctx, 0x0601, nil, func(raw []byte) error {
		var err error
		status, err = DecodeStatusData(raw)
		return err
	})
	if err != nil {
		return Status{}, err
	}
	return status, nil
}
