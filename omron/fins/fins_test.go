package fins

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"
)

func unhex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}
func reply(req, data []byte) []byte {
	b := append([]byte(nil), req[:12]...)
	b[0] = 0xC0
	copy(b[3:6], req[6:9])
	copy(b[6:9], req[3:6])
	return append(append(b, 0, 0), data...)
}
func TestFrameGoldenAndValidation(t *testing.T) {
	req := request(Route{Source: Address{Node: 1}, Destination: Address{Node: 2}}, 1, 0x0101, unhex("82000a00000a"))
	if hex.EncodeToString(req) != "80000200020000010001010182000a00000a" {
		t.Fatalf("request %x", req)
	}
	good := unhex("c0000200010000020001010100001234")
	b, err := response(req, good)
	if err != nil || !bytes.Equal(b, []byte{0x12, 0x34}) {
		t.Fatalf("%x %v", b, err)
	}
	for _, i := range []int{0, 1, 3, 4, 5, 6, 7, 8, 9, 10, 11} {
		bad := bytes.Clone(good)
		bad[i] ^= 0x41
		if _, err := response(req, bad); err == nil {
			t.Fatalf("accepted corrupt byte %d", i)
		}
	}
	bad := bytes.Clone(good)
	bad[12], bad[13] = 0x21, 0x02
	_, err = response(req, bad)
	var end *EndError
	if !errors.As(err, &end) || end.Code != 0x2102 {
		t.Fatalf("end error %v", err)
	}
	for i := 0; i < 14; i++ {
		if _, err := response(req, good[:i]); err == nil {
			t.Fatalf("accepted short frame %d", i)
		}
	}
}
func TestTCPEnvelope(t *testing.T) {
	want := unhex("46494e530000000c000000000000000000000001")
	if got := tcpFrame(0, []byte{0, 0, 0, 1}); !bytes.Equal(got, want) {
		t.Fatalf("TCP request %x", got)
	}
	b, err := readTCP(bytes.NewReader(tcpFrame(1, unhex("0000000100000002"))), 1)
	if err != nil || len(b) != 8 {
		t.Fatalf("%x %v", b, err)
	}
	for _, bad := range [][]byte{unhex("00000000000000080000000100000000"), unhex("46494e53000000070000000100000000"), unhex("46494e53000100010000000100000000"), unhex("46494e53000000080000000100000003"), tcpFrame(2, nil)} {
		if _, err := readTCP(bytes.NewReader(bad), 1); err == nil {
			t.Fatalf("accepted %x", bad)
		}
	}
}

type exchangeFunc func([]byte) ([]byte, error)

func (f exchangeFunc) Exchange(ctx context.Context, b []byte) ([]byte, error) { return f(b) }
func (f exchangeFunc) Close() error                                           { return nil }
func TestMemoryAddresses(t *testing.T) {
	for _, tt := range []struct {
		s    string
		area byte
		word uint16
		bit  byte
		bits bool
	}{
		{"100.01", 0x30, 100, 1, true}, {"CIO6143", 0xB0, 6143, 0, false}, {"D10", 0x82, 10, 0, false}, {"W511.15", 0x31, 511, 15, true}, {"H1408", 0xB2, 1408, 0, false}, {"A448", 0xB3, 448, 0, false}, {"E2_32767.0", 0x22, 32767, 0, true}, {"T1", 0x89, 1, 0, false}, {"C2.Q", 0x09, 0x8002, 0, true},
	} {
		m, err := ParseMemory(tt.s)
		if err != nil || m != (Memory{tt.area, tt.word, tt.bit, tt.bits}) {
			t.Fatalf("%s: %+v %v", tt.s, m, err)
		}
	}
	for _, s := range []string{"", "D-1", "D32768", "W512", "CIO6144", "D1.16", "D1.-1", "D1.", "D1.1.2", "E10_0", "T4096", "C1.0", "x1", "+1"} {
		if _, err := ParseMemory(s); err == nil {
			t.Fatalf("accepted %q", s)
		}
	}
	m, _ := ParseMemory("W511.15")
	if err := m.validate(2); err == nil {
		t.Fatal("accepted overflowing bit read")
	}
}
func TestReadMemory(t *testing.T) {
	c := New(exchangeFunc(func(req []byte) ([]byte, error) {
		if !bytes.Equal(req[10:], unhex("010182000a000002")) {
			t.Fatalf("read request %x", req)
		}
		return reply(req, unhex("1234abcd")), nil
	}), Route{})
	m, _ := ParseMemory("D10")
	b, err := c.ReadMemory(context.Background(), m, 2)
	if err != nil || !bytes.Equal(b, unhex("1234abcd")) {
		t.Fatalf("%x %v", b, err)
	}
	c = New(exchangeFunc(func(req []byte) ([]byte, error) { return reply(req, []byte{2}), nil }), Route{})
	m, _ = ParseMemory("W0.0")
	if _, err := c.ReadMemory(context.Background(), m, 1); err == nil {
		t.Fatal("accepted nonboolean bit")
	}
}
func TestProgramReadBlocks(t *testing.T) {
	data := bytes.Repeat([]byte{1, 2, 3, 4}, 300)
	calls := 0
	c := New(exchangeFunc(func(req []byte) ([]byte, error) {
		calls++
		if binary.BigEndian.Uint16(req[10:12]) != 0x0306 {
			t.Fatal("wrong command")
		}
		p := req[12:]
		start := int(binary.BigEndian.Uint32(p[2:6]))
		n := int(binary.BigEndian.Uint16(p[6:8]))
		if start%4 != 0 || n > 992 || n%4 != 0 {
			t.Fatalf("invalid range %d %d", start, n)
		}
		n = min(n, len(data)-start)
		out := bytes.Clone(p)
		flags := uint16(n)
		if start+n == len(data) {
			flags |= 0x8000
		}
		binary.BigEndian.PutUint16(out[6:8], flags)
		out = append(out, data[start:start+n]...)
		return reply(req, out), nil
	}), Route{})
	b, err := c.ReadProgram(context.Background(), len(data))
	if err != nil || !bytes.Equal(b, data) || calls != 2 {
		t.Fatalf("len=%d calls=%d %v", len(b), calls, err)
	}
	if _, err := c.ReadProgram(context.Background(), 1000); err == nil {
		t.Fatal("accepted incomplete backup")
	}
}
func TestProgramRejectsMalformedResponse(t *testing.T) {
	for _, body := range []string{"", "ffff000000000000", "ffff00000004000401020304", "ffff000000008003010203", "ffff00000000800801020304"} {
		c := New(exchangeFunc(func(req []byte) ([]byte, error) { return reply(req, unhex(body)), nil }), Route{})
		if _, err := c.ReadProgram(context.Background(), 1024); err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
}
func TestCancelledRequestNeverSent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := New(exchangeFunc(func([]byte) ([]byte, error) { t.Fatal("sent cancelled request"); return nil, nil }), Route{})
	if _, err := c.Call(ctx, 0x0501, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("%v", err)
	}
}
func TestContextInterruptsBlockedNetwork(t *testing.T) {
	left, right := net.Pipe()
	defer right.Close()
	c := New(&networkTransport{conn: left, tcp: true, timeout: time.Minute}, Route{})
	defer c.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := c.Call(ctx, 0x0501, nil); done <- err }()
	_ = right.SetDeadline(time.Now().Add(time.Second))
	if _, err := readTCP(right, 2); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation did not interrupt request")
	}
}

func TestFailedSessionIsNotReused(t *testing.T) {
	calls := 0
	c := New(exchangeFunc(func([]byte) ([]byte, error) { calls++; return nil, io.ErrUnexpectedEOF }), Route{})
	for i := 0; i < 2; i++ {
		if _, err := c.Call(context.Background(), 0x0501, nil); err == nil {
			t.Fatal("failed exchange accepted")
		}
	}
	if calls != 1 {
		t.Fatal("retried uncertain session")
	}
}

func TestPLCEndCodeDoesNotPoisonSession(t *testing.T) {
	calls := 0
	c := New(exchangeFunc(func(req []byte) ([]byte, error) {
		calls++
		b := reply(req, nil)
		if calls == 1 {
			b[12] = 0x21
			b[13] = 2
		}
		return b, nil
	}), Route{})
	if _, err := c.Call(context.Background(), 0x0501, nil); err == nil {
		t.Fatal("PLC error ignored")
	}
	if _, err := c.Call(context.Background(), 0x0501, nil); err != nil {
		t.Fatal(err)
	}
}
func TestHostLinkDirect(t *testing.T) {
	left, right := net.Pipe()
	defer right.Close()
	c, err := NewHostLink(left, 0, 0, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	done := make(chan error, 1)
	go func() {
		var got []byte
		one := []byte{0}
		for {
			if _, e := io.ReadFull(right, one); e != nil {
				done <- e
				return
			}
			got = append(got, one[0])
			if one[0] == '\r' {
				break
			}
		}
		// Captures the missing full Ethernet header and the single wait digit.
		body := "@00FA000000001010182000A000001"
		if string(got) != string(hostFrame(body)) {
			done <- errors.New("unexpected direct Host Link request: " + string(got))
			return
		}
		_, e := right.Write(hostFrame("@00FA40000001010100001234"))
		done <- e
	}()
	m, _ := ParseMemory("D10")
	b, err := c.ReadMemory(context.Background(), m, 1)
	if err != nil || !bytes.Equal(b, unhex("1234")) {
		t.Fatalf("%x %v", b, err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
func TestShortWrite(t *testing.T) {
	if err := writeAll(zeroWriter{}, []byte{1}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatal(err)
	}
}

type zeroWriter struct{}

func (zeroWriter) Write([]byte) (int, error) { return 0, nil }
func TestIdentityAndStatusValidation(t *testing.T) {
	for _, command := range []uint16{0x0501, 0x0601} {
		c := New(exchangeFunc(func(req []byte) ([]byte, error) { return reply(req, []byte{0, 0}), nil }), Route{})
		var err error
		if command == 0x0501 {
			_, err = c.Identity(context.Background())
		} else {
			_, err = c.Status(context.Background())
		}
		if err == nil {
			t.Fatalf("accepted truncated %04x response", command)
		}
	}
}
func FuzzFrame(f *testing.F) {
	f.Add(unhex("c0000200010000020001010100001234"))
	req := request(Route{Source: Address{Node: 1}, Destination: Address{Node: 2}}, 1, 0x0101, nil)
	f.Fuzz(func(t *testing.T, b []byte) { _, _ = response(req, b); _, _ = readTCP(bytes.NewReader(b), 2) })
}
func TestUnchangedInputs(t *testing.T) {
	b := unhex("c0000200010000020001010100001234")
	copyOf := bytes.Clone(b)
	_, _ = response(request(Route{Source: Address{Node: 1}, Destination: Address{Node: 2}}, 1, 0x0101, nil), b)
	if !reflect.DeepEqual(b, copyOf) {
		t.Fatal("response mutated")
	}
	if !strings.Contains((&EndError{0x0306, 0x2102}).Error(), "2102") {
		t.Fatal("missing end code")
	}
}
