package monitor

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	nio "github.com/joyautomation/nautilus/io"
	"github.com/joyautomation/nautilus/omron/fins"
)

type fixture struct {
	requests [][]byte
	fail     bool
	badDM    bool
	closed   bool
}

func (f *fixture) Close() error { f.closed = true; return nil }
func (f *fixture) Exchange(ctx context.Context, req []byte) ([]byte, error) {
	f.requests = append(f.requests, bytes.Clone(req))
	if f.fail {
		return nil, errors.New("link down")
	}
	header := bytes.Clone(req[:12])
	header[0] = 0xc0
	copy(header[3:6], req[6:9])
	copy(header[6:9], req[3:6])
	header = append(header, 0, 0)
	command := binary.BigEndian.Uint16(req[10:12])
	if command == 0x0601 {
		status := make([]byte, 26)
		status[1] = 4
		return append(header, status...), nil
	}
	if command != 0x0101 {
		return nil, errors.New("monitor attempted non-read command")
	}
	area := req[12]
	if area == 0x82 && f.badDM {
		header[12], header[13] = 0x11, 0x03
		return header, nil
	}
	switch area {
	case 0x06:
		return append(header, 1, 0), nil
	case 0x09:
		return append(header, 1), nil
	case 0xB1:
		return append(header, 0, 3), nil
	case 0x82:
		return append(header, 0, 0, 0x3f, 0x80), nil
	}
	return nil, errors.New("unexpected address")
}

func bindings() []Binding {
	return []Binding{{"Start", "W0.00", "BOOL"}, {"Stop", "W0.01", "BOOL"}, {"Speed", "D10", "REAL"}, {"Done", "C0.Q", "BOOL"}}
}
func newFixture(t *testing.T) (*Driver, *fixture) {
	t.Helper()
	wire := &fixture{}
	d, err := New(context.Background(), func(context.Context) (*fins.Client, error) { return fins.New(wire, fins.Route{}), nil }, bindings(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return d, wire
}
func TestPollGroupsAndWordOrder(t *testing.T) {
	d, wire := newFixture(t)
	defer d.Close()
	if len(d.blocks) != 3 {
		t.Fatalf("expected 3 contiguous blocks, got %d", len(d.blocks))
	}
	if d.Quality()["Start"] != nio.NotConnected {
		t.Fatal("unread tag starts good")
	}
	values, err := d.ReadInputs()
	if err != nil {
		t.Fatal(err)
	}
	if values["Start"] != true || values["Stop"] != true || values["Done"] != true || values["Speed"] != float64(1) {
		t.Fatalf("values: %v", values)
	}
	if len(wire.requests) != 4 {
		t.Fatalf("%d requests, wanted CPU status + three blocks", len(wire.requests))
	}
	if len(d.Quality()) != 0 || !d.Health().Connected || d.Health().Mode != 4 {
		t.Fatalf("health %+v quality %v", d.Health(), d.Quality())
	}
	values["Start"] = false
	if d.snapshot()["Start"] != true {
		t.Fatal("caller mutated driver cache")
	}
}

func TestPollAdjacentTaskFlags(t *testing.T) {
	wire := &fixture{}
	tags := []Binding{{"First", "TK0", "BOOL"}, {"Second", "TK1", "BOOL"}}
	d, err := New(context.Background(), func(context.Context) (*fins.Client, error) {
		return fins.New(wire, fins.Route{}), nil
	}, tags, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	values, err := d.ReadInputs()
	if err != nil || values["First"] != true || values["Second"] != false {
		t.Fatalf("task values: %v, %v", values, err)
	}
	if len(wire.requests) != 2 || !bytes.Equal(wire.requests[1][12:], []byte{6, 0, 0, 0, 0, 2}) {
		t.Fatalf("expected one grouped task read after status: %x", wire.requests)
	}
}
func TestPartialBadDoesNotPoisonSiblings(t *testing.T) {
	d, wire := newFixture(t)
	defer d.Close()
	wire.badDM = true
	values, _ := d.ReadInputs()
	if values["Start"] != true || d.Quality()["Speed"] != nio.Bad {
		t.Fatalf("values %v quality %v", values, d.Quality())
	}
	if _, bad := d.Quality()["Start"]; bad {
		t.Fatal("healthy sibling marked bad")
	}
	wire.badDM = false
	d.ReadInputs()
	if len(d.Quality()) != 0 {
		t.Fatal("quality did not recover")
	}
}
func TestDisconnectHoldsValuesAndBacksOff(t *testing.T) {
	d, wire := newFixture(t)
	defer d.Close()
	d.ReadInputs()
	wire.fail = true
	values, _ := d.ReadInputs()
	if values["Start"] != true || d.Quality()["Start"] != nio.Stale || d.Health().Connected {
		t.Fatalf("values %v health %+v quality %v", values, d.Health(), d.Quality())
	}
	count := len(wire.requests)
	d.ReadInputs()
	if len(wire.requests) != count || !wire.closed {
		t.Fatal("retried failed session without backoff")
	}
}
func TestFreshnessAndBadREAL(t *testing.T) {
	d, _ := newFixture(t)
	defer d.Close()
	d.ReadInputs()
	d.updated["Start"] = time.Now().Add(-2 * time.Second)
	if d.Quality()["Start"] != nio.Stale {
		t.Fatal("old reading still good")
	}
	_, err := decode(item{binding: Binding{Type: "REAL"}}, []byte{0, 0, 0x7f, 0x80})
	if err == nil {
		t.Fatal("nonfinite REAL accepted as JSON value")
	}
	i := item{binding: Binding{Type: "DINT"}}
	v, err := decode(i, []byte{0xff, 0xff, 0xff, 0xff})
	if err != nil || v != int64(-1) {
		t.Fatalf("signed double word %v %v", v, err)
	}
}
func TestBindingValidation(t *testing.T) {
	for _, b := range [][]Binding{
		{}, {{"A", "D0", "BOOL"}}, {{"A", "W0.00", "REAL"}}, {{"A", "D32767", "REAL"}}, {{"A", "H1535", "DWORD"}}, {{"A", "D0", "STRING"}}, {{"A", "D0", "INT"}, {"a", "D1", "INT"}},
	} {
		if _, err := New(context.Background(), func(context.Context) (*fins.Client, error) { panic("unexpected dial") }, b, time.Second); err == nil {
			t.Fatalf("accepted %v", b)
		}
	}
}
func TestFacadeReadOnlyAndState(t *testing.T) {
	d, _ := newFixture(t)
	defer d.Close()
	f, err := NewFacade(d, 250*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	f.rt.Scan()
	handler := f.Handler()
	for _, path := range []string{"/api/state", "/api/meta", "/api/omron/status"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("GET", "http://localhost"+path, nil))
		if w.Code != 200 || !json.Valid(w.Body.Bytes()) {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
		if path == "/api/meta" && strings.Contains(w.Body.String(), `"forces":true`) {
			t.Fatal("facade advertised physical forcing")
		}
	}
	for _, path := range []string{"/api/tags", "/api/program", "/api/forces", "/api/sfc/step"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest("POST", "http://localhost"+path, strings.NewReader(`{}`)))
		if w.Code != 403 {
			t.Fatalf("write allowed: %s %d", path, w.Code)
		}
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest("GET", "http://localhost/api/program", nil))
	if w.Code != 404 {
		t.Fatal("reported empty monitor program as physical PLC source")
	}
}
