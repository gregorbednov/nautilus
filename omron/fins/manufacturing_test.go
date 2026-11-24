package fins

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
)

func manufacturingSample(date string, version [3]byte) []byte {
	b := make([]byte, 26)
	copy(b[6:10], date)
	copy(b[21:24], version[:])
	return b
}

func TestCPUManufacturingObservedReplies(t *testing.T) {
	var observations []struct {
		Target                    string
		DataHex                   string
		DataSHA256                string
		ExpectedCXVersionDisplay  string
		ExpectedCXRevisionDisplay string
	}
	b, err := os.ReadFile("testdata/cj1-manufacturing-observed.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &observations); err != nil {
		t.Fatal(err)
	}
	if len(observations) != 2 {
		t.Fatal("missing observed CPU")
	}
	for _, observation := range observations {
		t.Run(observation.Target, func(t *testing.T) {
			data, err := hex.DecodeString(observation.DataHex)
			if err != nil {
				t.Fatal(err)
			}
			if fmt.Sprintf("%x", sha256.Sum256(data)) != observation.DataSHA256 {
				t.Fatal("fixture hash")
			}
			calls := 0
			c := New(exchangeFunc(func(req []byte) ([]byte, error) {
				calls++
				if !bytes.Equal(req[10:], []byte{2, 0x26, 0, 0, 0, 0}) {
					t.Fatalf("unexpected request %x", req)
				}
				return reply(req, data), nil
			}), Route{})
			got, err := c.ReadCPUManufacturingData(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if calls != 1 || !bytes.Equal(got.Raw, data) || got.CXVersionDisplay != observation.ExpectedCXVersionDisplay || got.CXRevisionDisplay != observation.ExpectedCXRevisionDisplay || !got.NativeVersionObjectGuardPassed {
				t.Fatalf("observed decode: %+v", got)
			}
			data[21] = 0xff
			if got.Raw[21] != 3 || got.VersionBytes[0] != 3 {
				t.Fatal("borrowed input")
			}
		})
	}
}

func TestCPUManufacturingDisplayAndNativeGuard(t *testing.T) {
	for _, test := range []struct {
		date   string
		accept bool
	}{
		{"0311", false}, {"0312", true}, {"0401", true}, {"8999", true},
		{"9001", false}, {"0201", false}, {"03x2", false}, {"\xff312", false},
	} {
		got, err := DecodeCPUManufacturingData(manufacturingSample(test.date, [3]byte{0x12, 0xa, 0x10}))
		if err != nil || got.NativeVersionObjectGuardPassed != test.accept || got.CXVersionDisplay != "12.A" || got.CXRevisionDisplay != "10" {
			t.Fatalf("date%q: %+v %v", test.date, got, err)
		}
	}
	blank, _ := DecodeCPUManufacturingData(manufacturingSample("0401", [3]byte{' ', ' ', ' '}))
	if blank.NativeVersionObjectGuardPassed || blank.CXVersionDisplay != "-" || blank.CXRevisionDisplay != "-" {
		t.Fatal(blank)
	}
	zero, _ := DecodeCPUManufacturingData(manufacturingSample("0401", [3]byte{}))
	if zero.CXVersionDisplay != "0.0" || zero.CXRevisionDisplay != "0" {
		t.Fatal(zero)
	}
	input := append(manufacturingSample("0401", [3]byte{3, 0, 9}), bytes.Repeat([]byte{0xaa}, 974)...)
	got, err := DecodeCPUManufacturingData(input)
	if err != nil || len(got.Raw) != 1000 {
		t.Fatal(err)
	}
	input[0] = 0xff
	if got.Raw[0] == 0xff {
		t.Fatal("borrowed raw DATA")
	}
	if _, err = DecodeCPUManufacturingData(append(input, 0)); err == nil {
		t.Fatal("accepted oversized DATA")
	}
}

func TestCPUManufacturingMalformedPayloadPoisonsClient(t *testing.T) {
	for _, limit := range []int{26, 538, 1000} {
		for size := 0; size <= 26; size++ {
			length := size
			if size == 26 {
				length = limit + 1
			}
			tr := &payloadTransport{}
			c := New(parameterTransportWithClose{payloadTransport: tr, exchangeFunc: func(req []byte) ([]byte, error) {
				tr.calls++
				return reply(req, make([]byte, length)), nil
			}}, Route{})
			// parameterTransportWithClose has the default1000-byte limit; use a
			// wrapper to exercise the transport-specific response bound too.
			c.transport = manufacturingLimitedTransport{Transport: c.transport, limit: limit}
			if _, err := c.ReadCPUManufacturingData(context.Background()); err == nil {
				t.Fatalf("accepted length%d limit%d", length, limit)
			}
			if _, err := c.ReadCPUManufacturingData(context.Background()); err == nil || tr.calls != 1 || tr.closes != 1 {
				t.Fatal("reused malformed session")
			}
		}
	}
}

type manufacturingLimitedTransport struct {
	Transport
	limit int
}

func (t manufacturingLimitedTransport) MaxReadData() int { return t.limit }

func TestCPUManufacturingSmallTransportAndEndErrors(t *testing.T) {
	tr := &payloadTransport{}
	c := New(manufacturingLimitedTransport{Transport: tr, limit: 25}, Route{})
	if _, err := c.ReadCPUManufacturingData(context.Background()); err == nil || tr.calls != 0 || tr.closes != 0 {
		t.Fatal("undersized transport sent a request")
	}
	for _, code := range []uint16{0x0040, 0x0080, 0x8000, 0x1104} {
		calls := 0
		c = New(exchangeFunc(func(req []byte) ([]byte, error) {
			calls++
			r := reply(req, nil)
			r[12], r[13] = byte(code>>8), byte(code)
			return r, nil
		}), Route{})
		got, err := c.ReadCPUManufacturingData(context.Background())
		var end *EndError
		if !errors.As(err, &end) || end.Code != code || end.Command != 0x0226 || calls != 1 || got.Raw != nil {
			t.Fatalf("end code: %+v %v", got, err)
		}
	}
}
