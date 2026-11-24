package fins

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"testing"
)

type parameterTransport struct {
	exchangeFunc
	limit int
}

func (p parameterTransport) MaxReadData() int { return p.limit }

func TestReadParameterAreaWholeRanges(t *testing.T) {
	for _, area := range []ParameterArea{ParameterPLCSetup, ParameterIOTable, ParameterRoutingTable, ParameterCPUBusSetup} {
		for _, limit := range []int{1000, 538, 8} {
			t.Run(fmt.Sprintf("%04x/%d", area, limit), func(t *testing.T) {
				words := map[ParameterArea]int{0x8010: 512, 0x8012: 1280, 0x8013: 512, 0x8002: 5184}[area]
				want := make([]byte, words*2)
				for i := 0; i < words; i++ {
					binary.BigEndian.PutUint16(want[i*2:], uint16(i))
				}
				next := 0
				c := New(parameterTransport{limit: limit, exchangeFunc: func(req []byte) ([]byte, error) {
					if len(req) != 18 || binary.BigEndian.Uint16(req[10:]) != 0x0201 || binary.BigEndian.Uint16(req[12:]) != uint16(area) {
						t.Fatalf("request %x", req)
					}
					start, n := int(binary.BigEndian.Uint16(req[14:])), int(binary.BigEndian.Uint16(req[16:]))
					if start != next || n < 1 || n > 498 || n*2+6 > limit || start+n > words {
						t.Fatalf("range %d/%d", start, n)
					}
					next += n
					b := append([]byte(nil), req[12:]...)
					if next == words {
						b[4] |= 0x80
					}
					return reply(req, append(b, want[start*2:next*2]...)), nil
				}}, Route{})
				got, err := c.ReadParameterArea(context.Background(), area)
				if err != nil || next != words || !bytes.Equal(got, want) {
					t.Fatalf("next=%d err=%v", next, err)
				}
			})
		}
	}
}

func TestParameterMalformedResponseStopsClient(t *testing.T) {
	for _, fault := range []string{"short", "area", "address", "count", "early-final", "missing-final", "extra"} {
		t.Run(fault, func(t *testing.T) {
			calls := 0
			tr := &payloadTransport{}
			// Track closure through the existing payload transport helper.
			c := New(parameterTransportWithClose{payloadTransport: tr, exchangeFunc: func(req []byte) ([]byte, error) {
				calls++
				n := int(binary.BigEndian.Uint16(req[16:]))
				b := append(append([]byte(nil), req[12:]...), make([]byte, n*2)...)
				final := binary.BigEndian.Uint16(req[14:])+uint16(n) == 512
				if final {
					b[4] |= 0x80
				}
				switch fault {
				case "short":
					b = b[:3]
				case "area":
					b[1] ^= 1
				case "address":
					b[3] ^= 1
				case "count":
					b[5] ^= 1
				case "early-final":
					b[4] |= 0x80
				case "missing-final":
					if final {
						b[4] &= 0x7f
					}
				case "extra":
					b = append(b, 0)
				}
				return reply(req, b), nil
			}}, Route{})
			got, err := c.ReadParameterArea(context.Background(), ParameterPLCSetup)
			if err == nil || got != nil {
				t.Fatal("accepted malformed/partial area")
			}
			before := calls
			if _, err = c.Call(context.Background(), 0x0501, nil); err == nil || calls != before || tr.closes != 1 {
				t.Fatal("malformed session reused")
			}
		})
	}
}

type parameterTransportWithClose struct {
	*payloadTransport
	exchangeFunc
}

func (p parameterTransportWithClose) Exchange(ctx context.Context, b []byte) ([]byte, error) {
	return p.exchangeFunc.Exchange(ctx, b)
}
func (p parameterTransportWithClose) Close() error { return p.payloadTransport.Close() }

func TestParameterLocalRejectionAndEndError(t *testing.T) {
	calls := 0
	c := New(exchangeFunc(func(req []byte) ([]byte, error) {
		calls++
		b := reply(req, nil)
		b[12], b[13] = 0x11, 0x04
		return b, nil
	}), Route{})
	for _, area := range []ParameterArea{0, 0x8000, 0x8011, 0xffff} {
		if _, err := c.ReadParameterArea(context.Background(), area); err == nil {
			t.Fatal("accepted unknown/combined area")
		}
	}
	if calls != 0 {
		t.Fatal("invalid area exchanged")
	}
	got, err := c.ReadParameterArea(context.Background(), ParameterPLCSetup)
	var end *EndError
	if got != nil || !errors.As(err, &end) || end.Code != 0x1104 || calls != 1 {
		t.Fatalf("unexpected retry/error %v calls=%d", err, calls)
	}
	small := New(parameterTransport{limit: 7, exchangeFunc: func([]byte) ([]byte, error) { t.Fatal("undersized transport used"); return nil, nil }}, Route{})
	if _, err := small.ReadParameterArea(context.Background(), ParameterPLCSetup); err == nil {
		t.Fatal("undersized transport accepted")
	}
}

func TestParameterLaterEndErrorDiscardsPartialArea(t *testing.T) {
	calls := 0
	c := New(exchangeFunc(func(req []byte) ([]byte, error) {
		calls++
		if calls == 2 {
			b := reply(req, nil)
			b[12], b[13] = 0x11, 0x04
			return b, nil
		}
		n := int(binary.BigEndian.Uint16(req[16:]))
		return reply(req, append(append([]byte(nil), req[12:]...), make([]byte, n*2)...)), nil
	}), Route{})
	got, err := c.ReadParameterArea(context.Background(), ParameterPLCSetup)
	var end *EndError
	if got != nil || !errors.As(err, &end) || calls != 2 {
		t.Fatalf("partial area returned or retry occurred: len=%d calls=%d err=%v", len(got), calls, err)
	}
}
