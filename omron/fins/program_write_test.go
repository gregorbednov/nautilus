package fins

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"testing"

	"github.com/joyautomation/nautilus/omron/transfer"
)

func writePayload(offset uint32, n int, final bool) []byte {
	p := make([]byte, 8+n)
	binary.BigEndian.PutUint16(p, 0xffff)
	binary.BigEndian.PutUint32(p[2:], offset)
	count := uint16(n)
	if final {
		count |= 0x8000
	}
	binary.BigEndian.PutUint16(p[6:], count)
	for i := 8; i < len(p); i++ {
		p[i] = byte(i)
	}
	return p
}
func writeACK(req []byte) []byte {
	data := bytes.Clone(req[12:20])
	data[6] &= 0x7f
	return reply(req, data)
}

type writeTransport struct {
	exchange      func([]byte) ([]byte, error)
	calls, closes int
	capacity      int
}

func (t *writeTransport) Exchange(_ context.Context, req []byte) ([]byte, error) {
	t.calls++
	return t.exchange(req)
}
func (t *writeTransport) Close() error { t.closes++; return nil }
func (t *writeTransport) MaxRequestData() int {
	if t.capacity == 0 {
		return 1004
	}
	return t.capacity
}

func TestProgramWriteBlockOwnsPayloadAndAcceptsOnlyExactACK(t *testing.T) {
	payload := writePayload(100, 4, true)
	var captured []byte
	transport := &writeTransport{exchange: func(req []byte) ([]byte, error) {
		captured = bytes.Clone(req)
		if binary.BigEndian.Uint16(req[10:12]) != 0x0307 {
			t.Fatal("wrong write command")
		}
		// Synchronous caller-side mutation while the exchange is running must not
		// change the private request used for ACK validation.
		payload[7] = 8
		return writeACK(req), nil
	}}
	c := New(transport, Route{Source: Address{Node: 1}, Destination: Address{Node: 2}})
	ack, err := c.WriteProgramBlock(context.Background(), payload)
	if err != nil || ack != (transfer.ProgramWriteACK{OffsetBytes: 100, DataBytes: 4, NextOffsetBytes: 104, FinalRequested: true}) || transport.calls != 1 || transport.closes != 0 || captured[19] != 4 {
		t.Fatal("incorrect write/ownership", ack, err)
	}
}
func TestProgramWriteErrorsPermanentlyCloseSessionWithoutRetry(t *testing.T) {
	for _, name := range []string{"short-count", "wrong-offset", "wrong-program", "echo-final", "extra", "sid", "route", "command", "end-code", "transport"} {
		t.Run(name, func(t *testing.T) {
			transport := &writeTransport{exchange: func(req []byte) ([]byte, error) {
				if name == "transport" {
					return nil, io.ErrUnexpectedEOF
				}
				frame := writeACK(req)
				switch name {
				case "short-count":
					frame[21] = 0
				case "wrong-offset":
					frame[19] ^= 4
				case "wrong-program":
					frame[14] = 0
				case "echo-final":
					frame[20] |= 0x80
				case "extra":
					frame = append(frame, 0)
				case "sid":
					frame[9] ^= 1
				case "route":
					frame[3] ^= 1
				case "command":
					frame[11] = 0x17
				case "end-code":
					frame[12] = 0x11
					frame[13] = 4
				}
				return frame, nil
			}}
			c := New(transport, Route{})
			ack, err := c.WriteProgramBlock(context.Background(), writePayload(0, 4, true))
			if err == nil || ack != (transfer.ProgramWriteACK{}) {
				t.Fatal("bad write acknowledged", ack, err)
			}
			if name == "end-code" {
				var end *EndError
				if !errors.As(err, &end) || end.Code != 0x1104 {
					t.Fatal("PLC end code lost", err)
				}
			}
			if _, err = c.WriteProgramBlock(context.Background(), writePayload(0, 4, true)); err == nil {
				t.Fatal("write retried")
			}
			if _, err = c.Identity(context.Background()); err == nil {
				t.Fatal("failed session reused")
			}
			if transport.calls != 1 || transport.closes != 1 {
				t.Fatal("session not terminal", transport.calls, transport.closes)
			}
		})
	}
}
func TestProgramWriteLocalRejectionOrPrecancelDoesNotSend(t *testing.T) {
	transport := &writeTransport{capacity: 538, exchange: func(req []byte) ([]byte, error) {
		if binary.BigEndian.Uint16(req[10:12]) == 0x0307 {
			return writeACK(req), nil
		}
		return reply(req, make([]byte, 40)), nil
	}}
	c := New(transport, Route{})
	if c.ProgramWriteBlockLimit() != 528 || New(&HostLink{}, Route{}).ProgramWriteBlockLimit() != 528 || (*Client)(nil).ProgramWriteBlockLimit() != 0 {
		t.Fatal("wrong transport bound")
	}
	invalid := [][]byte{nil, writePayload(0, 996, false), writePayload(1, 4, true), writePayload(0, 3, true)}
	wrongProgram := writePayload(0, 4, true)
	wrongProgram[0] = 0
	invalid = append(invalid, wrongProgram)
	overflow := writePayload(0xfffffffc, 4, true)
	invalid = append(invalid, overflow)
	for _, p := range invalid {
		if _, err := c.WriteProgramBlock(context.Background(), p); err == nil {
			t.Fatal("invalid local request accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.WriteProgramBlock(ctx, writePayload(0, 4, true)); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation lost", err)
	}
	if transport.calls != 0 || transport.closes != 0 {
		t.Fatal("local error touched transport")
	}
	if _, err := c.WriteProgramBlock(context.Background(), writePayload(0, 528, true)); err != nil {
		t.Fatal("bounded request rejected", err)
	}
	if _, err := c.Identity(context.Background()); err != nil {
		t.Fatal("healthy session damaged", err)
	}
	if _, err := (*Client)(nil).WriteProgramBlock(context.Background(), nil); err == nil {
		t.Fatal("nil write client accepted")
	}
}

func TestRawProgramWriteEndErrorAlsoPoisonsSession(t *testing.T) {
	transport := &writeTransport{exchange: func(req []byte) ([]byte, error) {
		frame := reply(req, nil)
		frame[12], frame[13] = 0x21, 0x02
		return frame, nil
	}}
	c := New(transport, Route{})
	if _, err := c.Call(context.Background(), 0x0307, writePayload(0, 4, true)); err == nil {
		t.Fatal("raw write error accepted")
	}
	if _, err := c.Identity(context.Background()); err == nil {
		t.Fatal("raw 0307 bypassed terminal failure")
	}
	if transport.calls != 1 || transport.closes != 1 {
		t.Fatal("unexpected retry", transport.calls, transport.closes)
	}
	capped := New(&writeTransport{capacity: -int(^uint(0)>>1) - 1}, Route{})
	if capped.ProgramWriteBlockLimit() != 0 {
		t.Fatal("capacity subtraction overflow")
	}
}
