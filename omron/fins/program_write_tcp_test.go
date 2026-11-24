package fins

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/joyautomation/nautilus/omron/transfer"
)

// The server copies opaque bytes only; it does not emulate CPU finalization,
// mode changes, protection, retained data or physical outputs.
func TestTCPProgramCandidateReadbackAndExplicitOriginalReturn(t *testing.T) {
	candidate, err := os.ReadFile("../compiler/testdata/n1-no-fb-cj1h-h.n1")
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile("../um/testdata/cj1h-baseline.n1")
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan error, 1)
	go func() {
		done <- func() error {
			conn, err := listener.Accept()
			if err != nil {
				return err
			}
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
			if _, err = readTCP(conn, 0); err != nil {
				return err
			}
			if err = writeAll(conn, tcpFrame(1, []byte{0, 0, 0, 10, 0, 0, 0, 20})); err != nil {
				return err
			}
			stored := bytes.Clone(original)
			var pending []byte
			completed := 0
			lastSID := byte(0)
			for {
				req, err := readTCP(conn, 2)
				if errors.Is(err, io.EOF) {
					if completed != 2 || !bytes.Equal(stored, original) {
						return errors.New("explicit return not completed")
					}
					return nil
				}
				if err != nil {
					return err
				}
				if len(req) < 12 || req[4] != 20 || req[7] != 10 || req[9] != lastSID+1 {
					return errors.New("wrong negotiated route/SID")
				}
				lastSID = req[9]
				var data []byte
				switch binary.BigEndian.Uint16(req[10:12]) {
				case 0x0601:
					data = make([]byte, 26) // synthetic PROGRAM, never a mode command
				case 0x0307:
					p, err := transfer.ValidateProgramWriteRequest(req[12:])
					if err != nil {
						return err
					}
					if p.OffsetBytes == 0 {
						pending = nil
					}
					if int(p.OffsetBytes) != len(pending) {
						return errors.New("nonsequential write")
					}
					pending = append(pending, req[20:]...)
					if p.Final {
						stored = bytes.Clone(pending)
						completed++
						want := candidate
						if completed == 2 {
							want = original
						}
						if completed > 2 || !bytes.Equal(stored, want) {
							return errors.New("unexpected complete bytes")
						}
					}
					data = bytes.Clone(req[12:20])
					data[6] &= 0x7f
				case 0x0306:
					if len(req) != 20 {
						return errors.New("bad read request")
					}
					offset := int(binary.BigEndian.Uint32(req[14:18]))
					count := int(binary.BigEndian.Uint16(req[18:20]))
					if offset >= len(stored) {
						return errors.New("read outside stored bytes")
					}
					count = min(count, len(stored)-offset)
					data = make([]byte, 8+count)
					copy(data, req[12:20])
					flags := uint16(count)
					if offset+count == len(stored) {
						flags |= 0x8000
					}
					binary.BigEndian.PutUint16(data[6:], flags)
					copy(data[8:], stored[offset:offset+count])
				default:
					return fmt.Errorf("unexpected command %04x", binary.BigEndian.Uint16(req[10:12]))
				}
				if err = writeAll(conn, tcpFrame(2, reply(req, data))); err != nil {
					return err
				}
			}
		}()
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, err := DialTCP(ctx, listener.Addr().String(), Route{}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	status, err := c.Status(ctx)
	if err != nil || status.Mode != 0 {
		t.Fatal("mock not in PROGRAM", err)
	}
	for _, image := range [][]byte{candidate, original} {
		plan, err := transfer.PlanProgramWrite(image, c.ProgramWriteBlockLimit())
		if err != nil {
			t.Fatal(err)
		}
		for _, block := range plan.Blocks {
			p, err := hex.DecodeString(block.PayloadHex)
			if err != nil {
				t.Fatal(err)
			}
			ack, err := c.WriteProgramBlock(ctx, p)
			if err != nil || ack.OffsetBytes != block.OffsetBytes || ack.DataBytes != block.DataBytes || ack.FinalRequested != block.Final {
				t.Fatal("block ACK failed", ack, err)
			}
		}
		got, err := c.ReadProgram(ctx, 16<<20)
		if err != nil || !bytes.Equal(got, image) {
			t.Fatal("mock readback differs", len(got), err)
		}
	}
	_ = c.Close()
	if err = <-done; err != nil {
		t.Fatal(err)
	}
}

func TestTCPLostProgramACKDoesNotReconnectOrRetry(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		if _, err = readTCP(conn, 0); err == nil {
			err = writeAll(conn, tcpFrame(1, []byte{0, 0, 0, 10, 0, 0, 0, 20}))
		}
		if err != nil {
			done <- err
			return
		}
		req, err := readTCP(conn, 2)
		if err == nil && (len(req) != 24 || binary.BigEndian.Uint16(req[10:12]) != 0x0307) {
			err = errors.New("wrong attempted write")
		}
		// Bytes could have been applied, but no ACK is delivered.
		done <- err
	}()
	c, err := DialTCP(context.Background(), listener.Addr().String(), Route{}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ack, err := c.WriteProgramBlock(context.Background(), writePayload(0, 4, true))
	if err == nil || ack != (transfer.ProgramWriteACK{}) {
		t.Fatal("lost ACK accepted")
	}
	if _, err = c.WriteProgramBlock(context.Background(), writePayload(0, 4, true)); err == nil {
		t.Fatal("retried uncertain write")
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
}

func TestHostLinkProgramWriteAtRequestCapacity(t *testing.T) {
	left, right := net.Pipe()
	defer right.Close()
	c, err := NewHostLink(left, 0, 0, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	payload := writePayload(0, 528, true)
	done := make(chan error, 1)
	go func() {
		_ = right.SetDeadline(time.Now().Add(2 * time.Second))
		got, err := bufio.NewReader(right).ReadString('\r')
		if err != nil {
			done <- err
			return
		}
		// Parameters are independent constants: FFFF, offset0, final count8210.
		expected := hostFrame("@00FA0000000010307FFFF000000008210" + strings.ToUpper(hex.EncodeToString(payload[8:])))
		if got != string(expected) {
			done <- errors.New("wrong bounded Host Link write frame")
			return
		}
		// ACK count is0210 without the request's final flag.
		_, err = right.Write(hostFrame("@00FA4000000103070000FFFF000000000210"))
		done <- err
	}()
	ack, err := c.WriteProgramBlock(context.Background(), payload)
	if err != nil || ack.DataBytes != 528 || !ack.FinalRequested || ack.NextOffsetBytes != 528 {
		t.Fatal("Host Link ACK failed", ack, err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
}
