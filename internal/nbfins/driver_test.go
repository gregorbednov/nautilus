package nbfins

import (
	"context"
	"encoding/binary"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	nio "github.com/joyautomation/nautilus/io"
)

func TestFINSOperatorWrites(t *testing.T) {
	peer, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	var writes atomic.Int32
	var drop atomic.Bool
	var dropWriteAck atomic.Bool
	go func() {
		var data [64]byte
		var bits [16]byte
		for {
			n, from, err := peer.ReadFromUDP(data[:])
			if err != nil {
				return
			}
			if drop.Load() || n < 18 || data[12] != cioBit || data[15] >= byte(len(bits)) || data[17] != 1 {
				continue
			}
			reply := []byte{0xc0, 0, 2, data[6], data[7], data[8], data[3], data[4], data[5], data[9], data[10], data[11], 0, 0}
			switch binary.BigEndian.Uint16(data[10:12]) {
			case readMemory:
				reply = append(reply, bits[data[15]])
			case writeMemory:
				if n != 19 || data[15] != 3 {
					continue
				}
				bits[data[15]] = data[18]
				writes.Add(1)
				if dropWriteAck.Load() {
					continue
				}
			default:
				continue
			}
			_, _ = peer.WriteToUDP(reply, from)
		}
	}()

	d, err := New(Config{
		Endpoint: peer.LocalAddr().String(), Source: []byte{0, 1, 0},
		Destination: []byte{0, 2, 0}, WriteAllow: []string{"CIO2.03"},
	}, []Binding{{"NB_CIO_0002_03", "CIO2.03"}, {"NB_CIO_0002_04", "CIO2.04"}})
	if err != nil {
		t.Fatal(err)
	}
	const writeTag, readTag = "NB_CIO_0002_03", "NB_CIO_0002_04"
	ctx := context.Background()
	if owned, err := d.WriteTag(ctx, writeTag, true); !owned || err == nil {
		t.Fatalf("write before first read: owned=%v error=%v", owned, err)
	}
	if err := d.WriteOutputs(nio.Values{writeTag: true}); err != nil {
		t.Fatal(err)
	}
	d.poll(ctx)
	if got := d.Quality(); len(got) != 0 || writes.Load() != 0 {
		t.Fatalf("poll quality=%v, writes=%d", got, writes.Load())
	}
	if owned, err := d.WriteTag(ctx, readTag, true); !owned || err == nil {
		t.Fatalf("allowlist: owned=%v error=%v", owned, err)
	}
	if owned, err := d.WriteTag(ctx, writeTag, 1); !owned || err == nil {
		t.Fatalf("type validation: owned=%v error=%v", owned, err)
	}
	d.SetWriteGate(func() bool { return false })
	if _, err := d.WriteTag(ctx, writeTag, true); err == nil || writes.Load() != 0 {
		t.Fatal("gate allowed a write")
	}
	d.SetWriteGate(func() bool { return true })
	if owned, err := d.WriteTag(ctx, writeTag, true); !owned || err != nil {
		t.Fatalf("write: owned=%v error=%v", owned, err)
	}
	values, _ := d.ReadInputs()
	if values[writeTag] != true || writes.Load() != 1 {
		t.Fatalf("read-back: %v, writes=%d", values, writes.Load())
	}
	drop.Store(true)
	timeout, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	d.poll(timeout)
	cancel()
	if d.Quality()[writeTag] != nio.Stale {
		t.Fatal("lost link must mark existing data stale")
	}
	if _, err := d.WriteTag(ctx, writeTag, false); err == nil {
		t.Fatal("write succeeded on stale data")
	}
	drop.Store(false)
	d.poll(ctx)
	if len(d.Quality()) != 0 || writes.Load() != 1 {
		t.Fatal("reconnect must refresh reads without repeating a write")
	}
	if _, err := d.WriteTag(ctx, writeTag, false); err != nil || writes.Load() != 2 {
		t.Fatalf("restore: writes=%d error=%v", writes.Load(), err)
	}

	// A write can reach the PLC even if the acknowledgment never arrives.
	// Keep the last confirmed value until polling learns what happened.
	dropWriteAck.Store(true)
	deadline, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	_, err = d.WriteTag(deadline, writeTag, true)
	cancel()
	if err == nil || !strings.Contains(err.Error(), "write outcome unconfirmed") {
		t.Fatalf("lost write acknowledgment: %v", err)
	}
	values, _ = d.ReadInputs()
	if d.Quality()[writeTag] != nio.Stale || values[writeTag] != false || writes.Load() != 3 {
		t.Fatalf("unconfirmed write changed published data or was retried: %v, writes=%d", values, writes.Load())
	}
	d.poll(ctx)
	values, _ = d.ReadInputs()
	if len(d.Quality()) != 0 || values[writeTag] != true || writes.Load() != 3 {
		t.Fatalf("poll did not reconcile the actual value without replay: %v, writes=%d", values, writes.Load())
	}
	d.disconnect()
}
