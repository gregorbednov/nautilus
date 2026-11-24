package fins

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"testing"
)

type controlCase struct {
	name    string
	command uint16
	data    []byte
	call    func(*Client, context.Context) error
}

func controlCases() []controlCase {
	return []controlCase{
		{"PROGRAM", 0x0402, []byte{0xff, 0xff}, func(c *Client, ctx context.Context) error { return c.RequestCPUMode(ctx, CPUModeProgram) }},
		{"MONITOR", 0x0401, []byte{0xff, 0xff, 2}, func(c *Client, ctx context.Context) error { return c.RequestCPUMode(ctx, CPUModeMonitor) }},
		{"RUN", 0x0401, []byte{0xff, 0xff, 4}, func(c *Client, ctx context.Context) error { return c.RequestCPUMode(ctx, CPUModeRun) }},
		{"ACQUIRE", 0x0c01, []byte{0xff, 0xff}, func(c *Client, ctx context.Context) error { return c.AcquireProgramAccess(ctx) }},
		{"RELEASE", 0x0c03, []byte{0xff, 0xff}, func(c *Client, ctx context.Context) error { return c.ReleaseProgramAccess(ctx) }},
	}
}
func TestControlRequestsUseExplicitProgramAndMode(t *testing.T) {
	for _, tc := range controlCases() {
		t.Run(tc.name, func(t *testing.T) {
			tr := &writeTransport{exchange: func(req []byte) ([]byte, error) {
				if binary.BigEndian.Uint16(req[10:12]) != tc.command || !bytes.Equal(req[12:], tc.data) {
					t.Fatal("incorrect explicit request", req)
				}
				return reply(req, nil), nil
			}}
			c := New(tr, Route{})
			if err := tc.call(c, context.Background()); err != nil || tr.calls != 1 || tr.closes != 0 {
				t.Fatal(err, tr)
			}
			_ = c.Close()
			if tr.calls != 1 {
				t.Fatal("Close issued cleanup command")
			}
		})
	}
}
func TestControlResponseFailuresTerminateWithoutRetryOrCleanup(t *testing.T) {
	for _, tc := range controlCases() {
		for _, fault := range []string{"transport", "short", "sid", "route", "command", "extra-ACK", "end-code"} {
			t.Run(tc.name+"/"+fault, func(t *testing.T) {
				tr := &writeTransport{exchange: func(req []byte) ([]byte, error) {
					if fault == "transport" {
						return nil, io.ErrUnexpectedEOF
					}
					b := reply(req, nil)
					switch fault {
					case "short":
						b = b[:13]
					case "sid":
						b[9]++
					case "route":
						b[4]++
					case "command":
						b[11]++
					case "extra-ACK":
						b = append(b, 0)
					case "end-code":
						b[12] = 0x21
						b[13] = 2
					}
					return b, nil
				}}
				c := New(tr, Route{})
				err := tc.call(c, context.Background())
				if err == nil || tr.calls != 1 || tr.closes != 1 {
					t.Fatal("failure accepted", err, tr)
				}
				if _, err = c.Status(context.Background()); err == nil || tr.calls != 1 {
					t.Fatal("failed control session reused")
				}
				if err = c.ReleaseProgramAccess(context.Background()); err == nil || tr.calls != 1 {
					t.Fatal("cleanup sent on failed connection")
				}
				if _, err = c.TakeExclusive(); err == nil {
					t.Fatal("failed session transferred")
				}
			})
		}
	}
}
func TestAccessBusyOwnerRequiresCorrelatedCompleteResponse(t *testing.T) {
	for _, fault := range []string{"valid", "missing", "short", "extra", "sid", "route", "command", "success-with-owner"} {
		t.Run(fault, func(t *testing.T) {
			data := []byte{3, 17, 0x80}
			switch fault {
			case "missing":
				data = nil
			case "short":
				data = data[:2]
			case "extra":
				data = append(data, 0)
			}
			tr := &writeTransport{exchange: func(req []byte) ([]byte, error) {
				b := reply(req, data)
				b[12] = 0x30
				b[13] = 1
				switch fault {
				case "sid":
					b[9]++
				case "route":
					b[4]++
				case "command":
					b[11]++
				case "success-with-owner":
					b[12] = 0
					b[13] = 0
				}
				return b, nil
			}}
			c := New(tr, Route{})
			err := c.AcquireProgramAccess(context.Background())
			var busy *AccessRightBusyError
			var end *EndError
			if fault == "valid" {
				if !errors.As(err, &busy) || busy.Owner != (Address{Network: 3, Node: 17, Unit: 0x80}) || !errors.As(err, &end) || end.Code != 0x3001 || end.Command != 0x0c01 {
					t.Fatal("busy owner lost", err)
				}
			} else if errors.As(err, &busy) {
				t.Fatal("unverified owner accepted", err)
			}
			if err == nil || tr.calls != 1 || tr.closes != 1 {
				t.Fatal("busy response not terminal", err)
			}
			_ = c.Close()
			if tr.calls != 1 {
				t.Fatal("busy triggered force/release")
			}
		})
	}
}
func TestControlLocalValidationAndCancellationDoNotSend(t *testing.T) {
	tr := &writeTransport{exchange: func(req []byte) ([]byte, error) { return reply(req, nil), nil }}
	c := New(tr, Route{})
	for _, mode := range []CPUMode{1, 3, 255} {
		if c.RequestCPUMode(context.Background(), mode) == nil {
			t.Fatal("unsupported mode accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tc := range controlCases() {
		if !errors.Is(tc.call(c, ctx), context.Canceled) {
			t.Fatal("cancel not preserved")
		}
	}
	if tr.calls != 0 || tr.closes != 0 {
		t.Fatal("local error performed I/O")
	}
	if err := c.AcquireProgramAccess(context.Background()); err != nil || tr.calls != 1 {
		t.Fatal("local error poisoned client", err)
	}
}
func TestRawControlEndCodesTerminateButReadRangeErrorsDoNot(t *testing.T) {
	for _, command := range []uint16{0x0401, 0x0402, 0x0c01, 0x0c02, 0x0c03, 0x0306} {
		t.Run(fmt.Sprintf("%04X", command), func(t *testing.T) {
			tr := &writeTransport{exchange: func(req []byte) ([]byte, error) { b := reply(req, nil); b[12] = 0x21; b[13] = 2; return b, nil }}
			c := New(tr, Route{})
			_, err := c.Call(context.Background(), command, nil)
			var end *EndError
			if !errors.As(err, &end) || end.Code != 0x2102 {
				t.Fatal("end code lost", err)
			}
			if command == 0x0306 {
				if tr.closes != 0 {
					t.Fatal("read error poisoned client")
				}
				_, _ = c.Call(context.Background(), command, nil)
				if tr.calls != 2 {
					t.Fatal("read behavior changed")
				}
			} else {
				if tr.closes != 1 {
					t.Fatal("raw control not closed")
				}
				_, _ = c.Call(context.Background(), command, nil)
				if tr.calls != 1 {
					t.Fatal("raw control retried")
				}
			}
		})
	}
}
