// Package monitor reads CJ1 memory into Nautilus's tag API. It does not run
// control logic, change CPU mode or implement a program-download path.
package monitor

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	nio "github.com/joyautomation/nautilus/io"
	"github.com/joyautomation/nautilus/omron/fins"
)

type Binding struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Type    string `json:"type"`
}
type Dial func(context.Context) (*fins.Client, error)
type item struct {
	binding Binding
	address fins.Memory
	memory  fins.Memory
	width   int
	bit     int
}
type block struct {
	memory fins.Memory
	count  uint16
	items  []item
}
type Health struct {
	Connected bool         `json:"connected"`
	Mode      byte         `json:"mode"`
	LastError string       `json:"lastError,omitempty"`
	LastPoll  int64        `json:"lastPoll"`
	CPU       *fins.Status `json:"cpu,omitempty"`
}

type Driver struct {
	ctx        context.Context
	dial       Dial
	blocks     []block
	bindings   []Binding
	staleAfter time.Duration
	pollMu     sync.Mutex
	client     *fins.Client
	nextDial   time.Time
	backoff    time.Duration
	mu         sync.RWMutex
	values     nio.Values
	quality    map[string]nio.Quality
	updated    map[string]time.Time
	health     Health
}

func New(ctx context.Context, dial Dial, bindings []Binding, staleAfter time.Duration) (*Driver, error) {
	if ctx == nil || dial == nil || len(bindings) == 0 || staleAfter <= 0 {
		return nil, errors.New("monitor requires context, dialer, bindings and positive stale-after")
	}
	d := &Driver{ctx: ctx, dial: dial, bindings: append([]Binding(nil), bindings...), staleAfter: staleAfter, values: nio.Values{}, quality: map[string]nio.Quality{}, updated: map[string]time.Time{}}
	var items []item
	seen := map[string]bool{}
	for index, b := range bindings {
		key := strings.ToLower(b.Name)
		if key == "" || seen[key] {
			return nil, fmt.Errorf("empty or duplicate tag name %q", b.Name)
		}
		seen[key] = true
		i, err := parseBinding(b)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", b.Name, err)
		}
		d.bindings[index] = i.binding
		items = append(items, i)
		d.quality[b.Name] = nio.NotConnected
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].memory.Area != items[j].memory.Area {
			return items[i].memory.Area < items[j].memory.Area
		}
		return items[i].memory.Word < items[j].memory.Word
	})
	for _, i := range items {
		end := int(i.memory.Word) + i.width
		if len(d.blocks) > 0 {
			b := &d.blocks[len(d.blocks)-1]
			if b.memory.Area == i.memory.Area && int(i.memory.Word) <= int(b.memory.Word)+int(b.count) && end-int(b.memory.Word) <= 256 {
				b.count = uint16(max(int(b.count), end-int(b.memory.Word)))
				b.items = append(b.items, i)
				continue
			}
		}
		d.blocks = append(d.blocks, block{memory: i.memory, count: uint16(i.width), items: []item{i}})
	}
	return d, nil
}

// BindingMemory validates a monitor binding and returns its original address.
func BindingMemory(b Binding) (fins.Memory, error) {
	i, err := parseBinding(b)
	return i.address, err
}

func parseBinding(b Binding) (item, error) {
	b.Type = strings.ToUpper(b.Type)
	m, err := fins.ParseMemory(b.Address)
	if err != nil {
		return item{}, err
	}
	i := item{binding: b, address: m, memory: m, width: 1, bit: -1}
	switch b.Type {
	case "BOOL":
		if !m.BitAccess {
			return item{}, errors.New("BOOL requires an explicit bit or .Q flag")
		}
		if m.Area != 0x09 && m.Area != 0x06 {
			i.bit = int(m.Bit)
			i.memory.Area += 0x80
			i.memory.Bit = 0
			i.memory.BitAccess = false
		}
	case "WORD", "UINT", "INT":
		if m.BitAccess {
			return item{}, errors.New("word type requires a word address")
		}
	case "DWORD", "UDINT", "DINT", "REAL":
		if m.BitAccess {
			return item{}, errors.New("double-word type requires a word address")
		}
		i.width = 2
		if _, err := fins.ParseMemory(nextWord(b.Address)); err != nil {
			return item{}, fmt.Errorf("double-word range: %w", err)
		}
	default:
		return item{}, fmt.Errorf("unsupported monitor type %q", b.Type)
	}
	return i, nil
}

// nextWord is used only after ParseMemory accepted a word address.
func nextWord(s string) string {
	s = strings.TrimSpace(s)
	start := len(s)
	for start > 0 && s[start-1] >= '0' && s[start-1] <= '9' {
		start--
	}
	var n int
	_, _ = fmt.Sscanf(s[start:], "%d", &n)
	return s[:start] + fmt.Sprint(n+1)
}

func (d *Driver) Close() error {
	d.pollMu.Lock()
	defer d.pollMu.Unlock()
	if d.client != nil {
		err := d.client.Close()
		d.client = nil
		return err
	}
	return nil
}
func (d *Driver) Bindings() []Binding { return append([]Binding(nil), d.bindings...) }
func (d *Driver) Health() Health {
	d.mu.RLock()
	defer d.mu.RUnlock()
	h := d.health
	if h.CPU != nil {
		status := *h.CPU
		status.Raw = append([]byte(nil), status.Raw...)
		h.CPU = &status
	}
	return h
}
func (d *Driver) WriteOutputs(values nio.Values) error {
	if len(values) > 0 {
		return errors.New("Omron monitor is read-only")
	}
	return nil
}

func (d *Driver) fail(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.health.Connected = false
	d.health.LastError = err.Error()
	for _, b := range d.bindings {
		if _, ok := d.values[b.Name]; ok {
			d.quality[b.Name] = nio.Stale
		} else {
			d.quality[b.Name] = nio.NotConnected
		}
	}
}
func (d *Driver) disconnect(err error) {
	d.fail(err)
	if d.client != nil {
		_ = d.client.Close()
		d.client = nil
	}
	if d.backoff == 0 {
		d.backoff = time.Second
	} else {
		d.backoff = min(30*time.Second, d.backoff*2)
	}
	d.nextDial = time.Now().Add(d.backoff)
}
func (d *Driver) snapshot() nio.Values {
	d.mu.RLock()
	defer d.mu.RUnlock()
	v := make(nio.Values, len(d.values))
	for k, value := range d.values {
		v[k] = value
	}
	return v
}

// ReadInputs polls contiguous ranges, then returns cached delivered values.
// Per-tag quality carries partial failures; a bad register must not mark its
// successfully read siblings stale through the runtime's whole-driver fallback.
func (d *Driver) ReadInputs() (nio.Values, error) {
	d.pollMu.Lock()
	defer d.pollMu.Unlock()
	if err := d.ctx.Err(); err != nil {
		d.fail(err)
		return d.snapshot(), nil
	}
	if d.client == nil {
		if time.Now().Before(d.nextDial) {
			return d.snapshot(), nil
		}
		client, err := d.dial(d.ctx)
		if err != nil {
			d.disconnect(err)
			return d.snapshot(), nil
		}
		d.client = client
	}
	status, err := d.client.Status(d.ctx)
	if err != nil {
		d.disconnect(err)
		return d.snapshot(), nil
	}
	d.mu.Lock()
	d.health.Connected = true
	d.health.CPU = &status
	d.health.Mode = status.Mode
	d.health.LastError = ""
	d.mu.Unlock()
	for _, b := range d.blocks {
		raw, err := d.client.ReadMemory(d.ctx, b.memory, b.count)
		if err != nil {
			var end *fins.EndError
			if !errors.As(err, &end) {
				d.disconnect(err)
				return d.snapshot(), nil
			}
			d.mu.Lock()
			for _, i := range b.items {
				d.quality[i.binding.Name] = nio.Bad
			}
			d.health.LastError = err.Error()
			d.mu.Unlock()
			continue
		}
		now := time.Now()
		d.mu.Lock()
		for _, i := range b.items {
			size := 2
			if b.memory.BitAccess {
				size = 1
			}
			offset := (int(i.memory.Word) - int(b.memory.Word)) * size
			value, err := decode(i, raw[offset:offset+i.width*size])
			if err != nil {
				d.quality[i.binding.Name] = nio.Bad
				d.health.LastError = err.Error()
				continue
			}
			d.values[i.binding.Name] = value
			d.quality[i.binding.Name] = nio.Good
			d.updated[i.binding.Name] = now
		}
		d.mu.Unlock()
	}
	d.backoff = 0
	d.mu.Lock()
	d.health.LastPoll = time.Now().UnixMilli()
	d.mu.Unlock()
	return d.snapshot(), nil
}

func decode(i item, b []byte) (any, error) {
	if i.binding.Type == "BOOL" {
		if i.bit < 0 {
			return b[0] != 0, nil
		}
		word := binary.BigEndian.Uint16(b)
		return word&(1<<i.bit) != 0, nil
	}
	word := binary.BigEndian.Uint16(b)
	switch i.binding.Type {
	case "INT":
		return int64(int16(word)), nil
	case "WORD", "UINT":
		return int64(word), nil
	}
	// FINS transmits bytes within a word high first. CJ double words store
	// the LOW 16-bit word at the lower memory address, independently of host.
	value := uint32(word) | uint32(binary.BigEndian.Uint16(b[2:]))<<16
	switch i.binding.Type {
	case "DINT":
		return int64(int32(value)), nil
	case "DWORD", "UDINT":
		return int64(value), nil
	case "REAL":
		v := float64(math.Float32frombits(value))
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, errors.New("non-finite PLC REAL")
		}
		return v, nil
	}
	return nil, errors.New("unsupported value type")
}

func (d *Driver) Quality() map[string]nio.Quality {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make(map[string]nio.Quality)
	for _, b := range d.bindings {
		q := d.quality[b.Name]
		if q == nio.Good && time.Since(d.updated[b.Name]) > d.staleAfter {
			q = nio.Stale
		}
		if q != nio.Good {
			out[b.Name] = q
		}
	}
	return out
}
