// Package fins implements the documented FINS wire protocol independently of
// CX-Server. Command layouts are from Omron W342, sections 3 and 5. Network
// transports are deliberately separate from executable program encoding.
package fins

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Address is a FINS network, node and unit tuple, not an IP address.
type Address struct{ Network, Node, Unit byte }
type Route struct{ Source, Destination Address }

// EndError is returned even when only a status bit is present in the end code.
// Callers must not silently turn abnormal PLC responses into good quality.
type EndError struct{ Command, Code uint16 }

func (e *EndError) Error() string { return fmt.Sprintf("FINS %04X end code %04X", e.Command, e.Code) }

func request(route Route, sid byte, command uint16, data []byte) []byte {
	b := []byte{0x80, 0, 2, route.Destination.Network, route.Destination.Node, route.Destination.Unit, route.Source.Network, route.Source.Node, route.Source.Unit, sid, byte(command >> 8), byte(command)}
	return append(b, data...)
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
		return nil, &EndError{binary.BigEndian.Uint16(req[10:12]), code}
	}
	return bytes.Clone(b[14:]), nil
}

const maxTCPSize = 65536

func tcpFrame(command uint32, data []byte) []byte {
	b := make([]byte, 16+len(data))
	copy(b, "FINS")
	binary.BigEndian.PutUint32(b[4:8], uint32(8+len(data)))
	binary.BigEndian.PutUint32(b[8:12], command)
	copy(b[16:], data)
	return b
}
func readTCP(r io.Reader, command uint32) ([]byte, error) {
	var h [16]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return nil, err
	}
	if string(h[:4]) != "FINS" {
		return nil, errors.New("invalid FINS/TCP signature")
	}
	n := binary.BigEndian.Uint32(h[4:8])
	if n < 8 || n > maxTCPSize {
		return nil, fmt.Errorf("invalid FINS/TCP length %d", n)
	}
	if code := binary.BigEndian.Uint32(h[12:16]); code != 0 {
		return nil, fmt.Errorf("FINS/TCP error %08X", code)
	}
	if got := binary.BigEndian.Uint32(h[8:12]); got != command {
		return nil, fmt.Errorf("FINS/TCP command %d, expected %d", got, command)
	}
	b := make([]byte, n-8)
	_, err := io.ReadFull(r, b)
	return b, err
}
