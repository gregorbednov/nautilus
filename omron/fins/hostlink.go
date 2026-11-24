package fins

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// HostLink wraps FINS in the direct-connection FA format from W342 3-5.
// Network-relayed Host Link and the proprietary peripheral/Toolbus protocol
// are different transports and are not silently substituted here.
type HostLink struct {
	stream  deadlineStream
	unit    int
	wait    byte
	timeout time.Duration
}

// NewHostLink takes ownership of a configured serial stream. Linux callers can
// use OpenSerial. A net.Pipe is also usable for protocol tests.
func NewHostLink(stream deadlineStream, unit int, wait byte, timeout time.Duration) (*Client, error) {
	if unit < 0 || unit > 31 || wait > 15 {
		return nil, errors.New("Host Link unit must be 0..31 and wait 0..15")
	}
	return New(&HostLink{stream: stream, unit: unit, wait: wait, timeout: timeout}, Route{}), nil
}
func (h *HostLink) Close() error     { return h.stream.Close() }
func (h *HostLink) MaxReadData() int { return 538 }

// MaxRequestData excludes the 12-byte FINS header/command prefix.
func (h *HostLink) MaxRequestData() int { return 538 }
func fcs(body string) string {
	var sum byte
	for i := 0; i < len(body); i++ {
		sum ^= body[i]
	}
	return fmt.Sprintf("%02X", sum)
}
func hostFrame(body string) []byte { return []byte(body + fcs(body) + "*\r") }
func (h *HostLink) Exchange(ctx context.Context, req []byte) (out []byte, err error) {
	if len(req) < 12 || len(req) > 550 {
		return nil, errors.New("Host Link command exceeds payload limit")
	}
	if req[3] != 0 || req[4] != 0 || req[6] != 0 || req[7] != 0 {
		return nil, errors.New("direct Host Link requires local FINS route")
	}
	// ICF=00, DA2, SA2, SID: this is NOT the full Ethernet FINS header.
	body := fmt.Sprintf("@%02dFA%X00%02X%02X%02X%s", h.unit, h.wait, req[5], req[8], req[9], strings.ToUpper(hex.EncodeToString(req[10:])))
	err = withDeadline(ctx, h.stream, h.timeout, func() error {
		if e := writeAll(h.stream, hostFrame(body)); e != nil {
			return e
		}
		var raw []byte
		var one [1]byte
		for len(raw) < 1115 {
			if _, e := io.ReadFull(h.stream, one[:]); e != nil {
				return e
			}
			raw = append(raw, one[0])
			if one[0] == '\r' {
				break
			}
		}
		if len(raw) < 25 || !strings.HasSuffix(string(raw), "*\r") {
			return errors.New("invalid Host Link response terminator/length")
		}
		content := string(raw[:len(raw)-4])
		sum := string(raw[len(raw)-4 : len(raw)-2])
		if sum != fcs(content) {
			return errors.New("Host Link FCS mismatch")
		}
		if !strings.HasPrefix(content, fmt.Sprintf("@%02dFA", h.unit)) {
			return errors.New("Host Link unit/header mismatch")
		}
		payload, e := hex.DecodeString(content[5:])
		if e != nil {
			return fmt.Errorf("Host Link response hex: %w", e)
		}
		if len(payload) < 8 || payload[0] != 0x40 || payload[1] != req[8] || payload[2] != req[5] || payload[3] != req[9] {
			return errors.New("Host Link response route/SID mismatch")
		}
		// Expand the direct-connection header for the shared FINS validator.
		out = []byte{0xC0, 0, 2, 0, 0, payload[1], 0, 0, payload[2], payload[3]}
		out = append(out, payload[4:]...)
		return nil
	})
	return
}
