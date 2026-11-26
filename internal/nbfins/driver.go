// Package nbfins supplies FINS/UDP data for NB operator screens. Only an
// explicit operator request can write. The scan and polling paths only read.
package nbfins

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	nio "github.com/joyautomation/nautilus/io"
)

type Config struct {
	Endpoint    string   `yaml:"endpoint"`
	Source      []byte   `yaml:"source"`
	Destination []byte   `yaml:"destination"`
	WriteAllow  []string `yaml:"write-allow"`
}

type Binding struct {
	Tag   string
	Alias string
}

const (
	pollInterval = 250 * time.Millisecond
	ioTimeout    = 900 * time.Millisecond
	staleAfter   = 2 * time.Second
)

var deadlinePassed = time.Unix(1, 0)

type Driver struct {
	endpoint string
	route    route
	names    []string
	bindings map[string]bitAddress
	allowed  map[bitAddress]bool

	// ioMu serializes the UDP client, reconnect and read-back sequence.
	ioMu sync.Mutex
	conn *udpClient

	mu      sync.RWMutex
	values  nio.Values
	updated time.Time
	healthy bool
	writeOK func() bool
}

func New(c Config, bindings []Binding) (*Driver, error) {
	host, port, err := net.SplitHostPort(c.Endpoint)
	ip := net.ParseIP(host)
	n, portErr := strconv.ParseUint(port, 10, 16)
	if err != nil || portErr != nil || n == 0 || ip == nil || ip.To4() == nil || ip.IsUnspecified() || ip.IsMulticast() ||
		ip.Equal(net.IPv4bcast) {
		return nil, errors.New("nb-fins.endpoint must be a unicast IPv4:port")
	}
	if len(c.Source) != 3 || len(c.Destination) != 3 ||
		c.Source[1] == 0 || c.Source[1] == 255 || c.Destination[1] == 0 || c.Destination[1] == 255 {
		return nil, errors.New("nb-fins source and destination need [network, node, unit] with unicast nodes 1–254")
	}
	if len(bindings) == 0 {
		return nil, errors.New("nb-fins needs at least one NB FINS tag")
	}
	d := &Driver{
		endpoint: c.Endpoint, route: route{Address(c.Source), Address(c.Destination)},
		bindings: make(map[string]bitAddress), allowed: make(map[bitAddress]bool), values: make(nio.Values),
	}
	configured := map[bitAddress]bool{}
	for _, binding := range bindings {
		address, err := parseBit(binding.Alias)
		if err != nil {
			return nil, fmt.Errorf("nb-fins tag %s: %w", binding.Tag, err)
		}
		if configured[address] {
			return nil, fmt.Errorf("nb-fins duplicate binding for %s", address)
		}
		configured[address] = true
		d.bindings[binding.Tag] = address
		d.names = append(d.names, binding.Tag)
	}
	slices.Sort(d.names)
	for _, alias := range c.WriteAllow {
		address, err := parseBit(alias)
		if err != nil {
			return nil, fmt.Errorf("nb-fins.write-allow: %w", err)
		}
		if !configured[address] {
			return nil, fmt.Errorf("nb-fins.write-allow: %s has no project tag", address)
		}
		d.allowed[address] = true
	}
	return d, nil
}

func parseBit(s string) (bitAddress, error) {
	var area byte
	var limit uint64
	text := s
	switch {
	case strings.HasPrefix(s, "CIO"):
		text, area, limit = s[3:], cioBit, 6143
	case strings.HasPrefix(s, "W"):
		text, area, limit = s[1:], workBit, 511
	}
	word, bit, ok := strings.Cut(text, ".")
	w, wErr := strconv.ParseUint(word, 10, 16)
	b, bErr := strconv.ParseUint(bit, 10, 4)
	if area == 0 || !ok || wErr != nil || bErr != nil || w > limit {
		return bitAddress{}, fmt.Errorf("invalid FINS bit address %q (expected CIO or W)", s)
	}
	return bitAddress{area, uint16(w), byte(b)}, nil
}

func (d *Driver) Writable() bool                { return len(d.allowed) > 0 }
func (d *Driver) InputNames() []string          { return slices.Clone(d.names) }
func (d *Driver) WriteOutputs(nio.Values) error { return nil }

func (d *Driver) SetWriteGate(open func() bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.writeOK = open
}

func (d *Driver) Start(ctx context.Context) {
	go func() {
		tick := time.NewTicker(pollInterval)
		defer tick.Stop()
		defer func() {
			d.ioMu.Lock()
			defer d.ioMu.Unlock()
			d.disconnect()
		}()
		for {
			d.poll(ctx)
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
	}()
}

func (d *Driver) ReadInputs() (nio.Values, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return maps.Clone(d.values), nil
}

func (d *Driver) Quality() map[string]nio.Quality {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.healthy && time.Since(d.updated) <= staleAfter {
		return nil
	}
	result := make(map[string]nio.Quality, len(d.names))
	for _, tag := range d.names {
		if _, ok := d.values[tag]; ok {
			result[tag] = nio.Stale
		} else {
			result[tag] = nio.NotConnected
		}
	}
	return result
}

func (d *Driver) poll(parent context.Context) {
	d.ioMu.Lock()
	defer d.ioMu.Unlock()
	ctx, cancel := context.WithTimeout(parent, ioTimeout)
	defer cancel()
	if d.conn == nil {
		conn, err := dialUDP(ctx, d.endpoint, d.route)
		if err != nil {
			d.disconnect()
			return
		}
		d.conn = conn
	}
	values := make(nio.Values, len(d.names))
	for _, tag := range d.names {
		value, err := d.conn.read(ctx, d.bindings[tag])
		if err != nil {
			d.disconnect()
			return
		}
		values[tag] = value
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.values, d.updated, d.healthy = values, time.Now(), true
}

func (d *Driver) disconnect() {
	if d.conn != nil {
		_ = d.conn.conn.Close()
		d.conn = nil
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.healthy = false
}

type refusal struct {
	status int
	text   string
}

func (e refusal) Error() string   { return e.text }
func (e refusal) HTTPStatus() int { return e.status }

// WriteTag reports whether it owns the name. It refuses a write before the
// first good read and after link loss, and never retries a write.
func (d *Driver) WriteTag(parent context.Context, name string, value any) (bool, error) {
	name = strings.ToUpper(name)
	a, found := d.bindings[name]
	if !found {
		return false, nil
	}
	if !d.allowed[a] {
		return true, refusal{http.StatusForbidden, fmt.Sprintf("%s is outside nb-fins.write-allow", a)}
	}
	bit, ok := value.(bool)
	if !ok {
		return true, refusal{http.StatusUnprocessableEntity, "FINS bit writes require a boolean"}
	}
	d.ioMu.Lock()
	defer d.ioMu.Unlock()
	d.mu.RLock()
	healthy := d.healthy && time.Since(d.updated) <= staleAfter
	gate := d.writeOK
	d.mu.RUnlock()
	if gate != nil && !gate() {
		return true, refusal{http.StatusConflict, "Nautilus write gate is closed"}
	}
	if d.conn == nil || !healthy {
		return true, refusal{http.StatusConflict, "no recent PLC data; wait for a successful read"}
	}
	ctx, cancel := context.WithTimeout(parent, ioTimeout)
	defer cancel()
	if err := d.conn.write(ctx, a, bit); err != nil {
		d.disconnect()
		return true, fmt.Errorf("%s write outcome unconfirmed: %w", a, err)
	}
	got, err := d.conn.read(ctx, a)
	if err != nil {
		d.disconnect()
		return true, fmt.Errorf("%s read-back failed: %w", a, err)
	}
	d.mu.Lock()
	d.values[name] = got
	d.mu.Unlock()
	if got != bit {
		return true, fmt.Errorf("%s read-back was %t; requested %t", a, got, bit)
	}
	return true, nil
}
