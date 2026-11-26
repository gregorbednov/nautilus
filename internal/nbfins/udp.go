package nbfins

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
)

// Address is a FINS network/node/unit tuple. UDP framing and response
// correlation are adapted from omron/fins at b76c9000.
type Address [3]byte

type route struct{ source, destination Address }

type udpClient struct {
	conn  net.Conn
	route route
	sid   byte
}

const (
	readMemory  = 0x0101
	writeMemory = 0x0102
	cioBit      = 0x30
	workBit     = 0x31
)

type bitAddress struct {
	area byte
	word uint16
	bit  byte
}

func (a bitAddress) String() string {
	area := "CIO"
	if a.area == workBit {
		area = "W"
	}
	return fmt.Sprintf("%s%d.%02d", area, a.word, a.bit)
}

func dialUDP(ctx context.Context, endpoint string, r route) (*udpClient, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "udp4", endpoint)
	if err != nil {
		return nil, err
	}
	return &udpClient{conn: conn, route: r}, nil
}

func (c *udpClient) exchange(ctx context.Context, command uint16, address bitAddress, write *bool) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	deadline, ok := ctx.Deadline()
	if !ok {
		return nil, errors.New("FINS exchange requires a deadline")
	}
	if err := c.conn.SetDeadline(deadline); err != nil {
		return nil, err
	}
	cancelled := make(chan struct{})
	cancel := context.AfterFunc(ctx, func() { _ = c.conn.SetDeadline(deadlinePassed); close(cancelled) })
	defer func() {
		if !cancel() {
			<-cancelled
		}
	}()
	c.sid++
	dst, src := c.route.destination, c.route.source
	req := []byte{
		0x80, 0, 2, dst[0], dst[1], dst[2], src[0], src[1], src[2], c.sid,
		byte(command >> 8), byte(command), address.area, byte(address.word >> 8), byte(address.word), address.bit, 0, 1,
	}
	if write != nil {
		var value byte
		if *write {
			value = 1
		}
		req = append(req, value)
	}
	n, err := c.conn.Write(req)
	if err != nil {
		return nil, err
	}
	if n != len(req) {
		return nil, errors.New("short FINS UDP write")
	}
	var reply [64]byte
	n, err = c.conn.Read(reply[:])
	if err != nil {
		return nil, err
	}
	return response(req, reply[:n])
}

func response(req, b []byte) ([]byte, error) {
	if len(req) < 12 || len(b) < 14 {
		return nil, errors.New("short FINS response")
	}
	if b[0]&0x40 == 0 || b[0]&1 != 0 || b[1] != 0 {
		return nil, errors.New("invalid FINS response header")
	}
	if b[9] != req[9] || !bytes.Equal(b[10:12], req[10:12]) {
		return nil, errors.New("FINS response SID/command mismatch")
	}
	if !bytes.Equal(b[3:6], req[6:9]) || !bytes.Equal(b[6:9], req[3:6]) {
		return nil, errors.New("FINS response route mismatch")
	}
	if code := binary.BigEndian.Uint16(b[12:14]); code != 0 {
		return nil, fmt.Errorf("FINS %04X end code %04X", binary.BigEndian.Uint16(req[10:12]), code)
	}
	return b[14:], nil
}

func (c *udpClient) read(ctx context.Context, address bitAddress) (bool, error) {
	data, err := c.exchange(ctx, readMemory, address, nil)
	if err != nil {
		return false, err
	}
	if len(data) != 1 || data[0] > 1 {
		return false, errors.New("invalid FINS bit read payload")
	}
	return data[0] == 1, nil
}

func (c *udpClient) write(ctx context.Context, address bitAddress, value bool) error {
	data, err := c.exchange(ctx, writeMemory, address, &value)
	if err == nil && len(data) != 0 {
		return errors.New("invalid FINS bit write payload")
	}
	return err
}
