package fins

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"testing"
	"time"
)

func controlCPUFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/cpu-unit-data-prefix-observations.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct{ Name, RawPrefixHex string }
	}
	if err = json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	for _, item := range corpus.Cases {
		if item.Name == name {
			b, err := hex.DecodeString(item.RawPrefixHex)
			if err != nil {
				t.Fatal(err)
			}
			return b
		}
	}
	t.Fatal("CPU fixture missing")
	return nil
}

// A protocol peer, not a CPU emulator: it only tracks a synthetic mode byte.
// No real endpoint, program write, access force or protection command is used.
func TestTCPExplicitControlSequenceBothCPUProfiles(t *testing.T) {
	for _, model := range []string{"cj1m", "cj1h"} {
		t.Run(model, func(t *testing.T) {
			cpu := controlCPUFixture(t, model)
			l, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer l.Close()
			done := make(chan error, 1)
			go func() {
				done <- func() error {
					c, err := l.Accept()
					if err != nil {
						return err
					}
					defer c.Close()
					_ = c.SetDeadline(time.Now().Add(5 * time.Second))
					if _, err = readTCP(c, 0); err != nil {
						return err
					}
					if err = writeAll(c, tcpFrame(1, []byte{0, 0, 0, 10, 0, 0, 0, 20})); err != nil {
						return err
					}
					expected := []uint16{0x0501, 0x0c01, 0x0402, 0x0601, 0x0401, 0x0601, 0x0401, 0x0601, 0x0c03, 0x0501}
					mode := byte(4)
					for i, command := range expected {
						req, err := readTCP(c, 2)
						if err != nil {
							return err
						}
						if len(req) < 12 || binary.BigEndian.Uint16(req[10:12]) != command || req[9] != byte(i+1) || req[4] != 20 || req[7] != 10 {
							return errors.New("wrong command order/route/SID")
						}
						var data []byte
						switch command {
						case 0x0501:
							data = cpu
						case 0x0601:
							data = make([]byte, 26)
							data[1] = mode
						case 0x0c01, 0x0c03, 0x0402:
							if !bytes.Equal(req[12:], []byte{0xff, 0xff}) {
								return errors.New("implicit program number")
							}
							if command == 0x0402 {
								mode = 0
							}
						case 0x0401:
							if len(req) != 15 || !bytes.Equal(req[12:14], []byte{0xff, 0xff}) || (req[14] != 2 && req[14] != 4) {
								return errors.New("implicit/invalid execution mode")
							}
							mode = req[14]
						}
						if err = writeAll(c, tcpFrame(2, reply(req, data))); err != nil {
							return err
						}
					}
					if _, err = readTCP(c, 2); !errors.Is(err, io.EOF) {
						return fmt.Errorf("unexpected automatic command after release: %v", err)
					}
					return nil
				}()
			}()
			c, err := DialTCP(context.Background(), l.Addr().String(), Route{}, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			before, err := c.Identity(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if err = c.AcquireProgramAccess(context.Background()); err != nil {
				t.Fatal(err)
			}
			for _, mode := range []CPUMode{CPUModeProgram, CPUModeMonitor, CPUModeRun} {
				if err = c.RequestCPUMode(context.Background(), mode); err != nil {
					t.Fatal(err)
				}
				status, err := c.Status(context.Background())
				if err != nil || status.Mode != byte(mode) {
					t.Fatal("explicit mock readback failed", status, err)
				}
			}
			if err = c.ReleaseProgramAccess(context.Background()); err != nil {
				t.Fatal(err)
			}
			after, err := c.Identity(context.Background())
			if err != nil || !bytes.Equal(before.Raw, after.Raw) {
				t.Fatal("identity changed", err)
			}
			_ = c.Close()
			if err = <-done; err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Adversarial byte peer deliberately keeps the owner after TCP EOF. W342 does
// not establish that a new connection inherits access or that EOF releases it.
// This test enforces absence of implicit cleanup/reacquisition assumptions.
func TestTCPNewConnectionDoesNotInheritAccessOrForceCleanup(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	done := make(chan error, 1)
	go func() {
		done <- func() error {
			for session := 0; session < 2; session++ {
				c, err := l.Accept()
				if err != nil {
					return err
				}
				_ = c.SetDeadline(time.Now().Add(5 * time.Second))
				err = func() error {
					defer c.Close()
					if _, err := readTCP(c, 0); err != nil {
						return err
					}
					if err := writeAll(c, tcpFrame(1, []byte{0, 0, 0, byte(10 + session), 0, 0, 0, 20})); err != nil {
						return err
					}
					req, err := readTCP(c, 2)
					if err != nil {
						return err
					}
					if len(req) != 14 || binary.BigEndian.Uint16(req[10:12]) != 0x0c01 || !bytes.Equal(req[12:], []byte{0xff, 0xff}) {
						return errors.New("unexpected acquire request")
					}
					b := reply(req, nil)
					if session == 1 {
						b = reply(req, []byte{0, 10, 0})
						b[12] = 0x30
						b[13] = 1
					}
					if err = writeAll(c, tcpFrame(2, b)); err != nil {
						return err
					}
					if _, err = readTCP(c, 2); !errors.Is(err, io.EOF) {
						return errors.New("automatic release/force/retry after close or busy")
					}
					return nil
				}()
				if err != nil {
					return err
				}
			}
			return nil
		}()
	}()
	first, err := DialTCP(context.Background(), l.Addr().String(), Route{}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err = first.AcquireProgramAccess(context.Background()); err != nil {
		t.Fatal(err)
	}
	_ = first.Close()
	second, err := DialTCP(context.Background(), l.Addr().String(), Route{}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	err = second.AcquireProgramAccess(context.Background())
	var busy *AccessRightBusyError
	if !errors.As(err, &busy) || busy.Owner != (Address{Node: 10}) {
		t.Fatal("fresh session assumed owner", err)
	}
	if err = second.ReleaseProgramAccess(context.Background()); err == nil {
		t.Fatal("busy connection reused for release")
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
}
