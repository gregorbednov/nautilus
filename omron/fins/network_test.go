package fins

import (
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestTCPNegotiationWithFragmentedReply(t *testing.T) {
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
		_ = conn.SetDeadline(time.Now().Add(2 * time.Second))
		b, err := readTCP(conn, 0)
		if err != nil {
			done <- err
			return
		}
		if len(b) != 4 || binary.BigEndian.Uint32(b) != 0 {
			t.Error("wrong node assignment request")
		}
		frame := tcpFrame(1, unhex("0000000a00000014"))
		for _, v := range frame {
			if _, err := conn.Write([]byte{v}); err != nil {
				done <- err
				return
			}
		}
		req, err := readTCP(conn, 2)
		if err != nil {
			done <- err
			return
		}
		if req[4] != 20 || req[7] != 10 {
			t.Errorf("negotiated nodes missing: %x", req)
		}
		status := make([]byte, 26)
		status[1] = 4
		done <- writeAll(conn, tcpFrame(2, reply(req, status)))
	}()
	c, err := DialTCP(context.Background(), listener.Addr().String(), Route{}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	s, err := c.Status(context.Background())
	if err != nil || s.Mode != 4 {
		t.Fatalf("status %+v %v", s, err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestUDPReadUsesConfiguredRoute(t *testing.T) {
	server, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	done := make(chan error, 1)
	go func() {
		_ = server.SetDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, 2048)
		n, peer, err := server.ReadFrom(buf)
		if err != nil {
			done <- err
			return
		}
		req := buf[:n]
		if req[4] != 7 || req[7] != 8 {
			t.Errorf("configured nodes missing: %x", req)
		}
		_, err = server.WriteTo(reply(req, unhex("1234")), peer)
		done <- err
	}()
	c, err := DialUDP(context.Background(), server.LocalAddr().String(), Route{Source: Address{Node: 8}, Destination: Address{Node: 7}}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	m, _ := ParseMemory("D10")
	b, err := c.ReadMemory(context.Background(), m, 1)
	if err != nil || len(b) != 2 || b[0] != 0x12 {
		t.Fatalf("%x %v", b, err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
