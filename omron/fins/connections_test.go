package fins

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

func connectionRecord(address byte, model string) []byte {
	b := bytes.Repeat([]byte{' '}, 21)
	b[0] = address
	copy(b[1:], model)
	return b
}

func TestReadConnectionsPagination(t *testing.T) {
	full := []byte{0}
	for i := 0x10; i <= 0x7f; i++ {
		full = append(full, byte(i))
	}
	full = append(full, 0xe1)
	for _, addresses := range [][]byte{{0}, {0, 0x11, 0x12, 0x23}, {0, 0x7f, 0xe1}, full} {
		for _, limit := range []int{1000, 43, 22} {
			calls := 0
			c := New(parameterTransport{limit: limit, exchangeFunc: func(req []byte) ([]byte, error) {
				calls++
				if len(req) != 14 || binary.BigEndian.Uint16(req[10:]) != 0x0502 {
					t.Fatalf("request %x", req)
				}
				start, n := req[12], int(req[13])
				if n < 1 || n > 25 || 1+21*n > limit {
					t.Fatal("unbounded count")
				}
				var b []byte
				b = append(b, 0)
				last := 0
				for _, a := range addresses {
					if a < start {
						continue
					}
					if int(b[0]) == n {
						break
					}
					b = append(b, connectionRecord(a, "TEST-UNIT")...)
					b[0]++
					last = int(a)
				}
				if byte(last) == addresses[len(addresses)-1] {
					b[0] |= 0x80
				}
				return reply(req, b), nil
			}}, Route{})
			units, err := c.ReadConnections(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			var got []byte
			for _, u := range units {
				got = append(got, u.Address)
				if u.Model != "TEST-UNIT" || len(u.RawModel) != 20 {
					t.Fatalf("unit %+v", u)
				}
			}
			if !bytes.Equal(got, addresses) || calls > len(addresses) {
				t.Fatalf("addresses=%x calls=%d", got, calls)
			}
		}
	}
}

func TestMalformedConnectionDataPoisonsClient(t *testing.T) {
	good := append([]byte{0x82}, connectionRecord(0, "CPU")...)
	good = append(good, connectionRecord(0x11, "BUS")...)
	for _, fault := range []string{"empty", "zero-count", "too-many", "short", "extra", "missing-cpu", "duplicate", "bad-address", "empty-model", "non-ascii", "final-address-without-last", "backwards-second-page"} {
		t.Run(fault, func(t *testing.T) {
			calls := 0
			tr := &payloadTransport{}
			c := New(parameterTransportWithClose{payloadTransport: tr, exchangeFunc: func(req []byte) ([]byte, error) {
				calls++
				b := bytes.Clone(good)
				switch fault {
				case "empty":
					b = nil
				case "zero-count":
					b = []byte{0x80}
				case "too-many":
					b[0] = 0x9a
				case "short":
					b = b[:len(b)-1]
				case "extra":
					b = append(b, 0)
				case "missing-cpu":
					b[1] = 0x10
				case "duplicate":
					b[22] = 0
				case "bad-address":
					b[22] = 0x80
				case "empty-model":
					copy(b[2:22], bytes.Repeat([]byte{' '}, 20))
				case "non-ascii":
					b[2] = 0xff
				case "final-address-without-last":
					b[0] = 2
					b[22] = 0xe1
				case "backwards-second-page":
					if calls == 1 {
						b[0] = 2
					} else {
						b[1] = 0
					}
				}
				return reply(req, b), nil
			}}, Route{})
			got, err := c.ReadConnections(context.Background())
			if err == nil || got != nil {
				t.Fatal("accepted malformed/partial enumeration")
			}
			before := calls
			if _, err = c.Call(context.Background(), 0x0501, nil); err == nil || calls != before || tr.closes != 1 {
				t.Fatal("malformed client reused")
			}
		})
	}
}

func TestConnectionsEndErrorAndSmallTransport(t *testing.T) {
	calls := 0
	c := New(exchangeFunc(func(req []byte) ([]byte, error) {
		calls++
		if calls == 1 {
			return reply(req, append([]byte{1}, connectionRecord(0, "CPU")...)), nil
		}
		b := reply(req, nil)
		b[12], b[13] = 0x11, 0x03
		return b, nil
	}), Route{})
	got, err := c.ReadConnections(context.Background())
	var end *EndError
	if got != nil || !errors.As(err, &end) || end.Code != 0x1103 || calls != 2 {
		t.Fatalf("partial/retry/error %v %d", err, calls)
	}
	small := New(parameterTransport{limit: 21, exchangeFunc: func([]byte) ([]byte, error) { t.Fatal("undersized transport used"); return nil, nil }}, Route{})
	if _, err = small.ReadConnections(context.Background()); err == nil {
		t.Fatal("small transport accepted")
	}
}

func TestConnectionModelOwnedCopy(t *testing.T) {
	wire := append([]byte{0x81}, connectionRecord(0, "CPU")...)
	c := New(exchangeFunc(func(req []byte) ([]byte, error) { return reply(req, wire), nil }), Route{})
	got, err := c.ReadConnections(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	saved := bytes.Clone(got[0].RawModel)
	wire[2] = 'X'
	if !reflect.DeepEqual(saved, got[0].RawModel) {
		t.Fatal("raw model aliased transport")
	}
}

func TestConnectionsRetainedHardwareObservations(t *testing.T) {
	raw, err := os.ReadFile("testdata/connection-data-observations.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Target  string
		DataHex string
		Units   []ConnectedUnit
	}
	if err = json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 2 {
		t.Fatal("missing hardware observation")
	}
	for _, f := range fixtures {
		t.Run(f.Target, func(t *testing.T) {
			data, err := hex.DecodeString(f.DataHex)
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			c := New(exchangeFunc(func(req []byte) ([]byte, error) {
				calls++
				if !bytes.Equal(req[10:], []byte{5, 2, 0, 25}) {
					t.Fatalf("unexpected command %x", req)
				}
				return reply(req, data), nil
			}), Route{})
			got, err := c.ReadConnections(context.Background())
			if err != nil || calls != 1 || !reflect.DeepEqual(got, f.Units) {
				t.Fatalf("got=%+v err=%v calls=%d", got, err, calls)
			}
		})
	}
}
