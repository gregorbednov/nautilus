package fins

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
)

// ParameterArea selects a separate W342 parameter area, not I/O memory.
// These are registered settings, not an inventory of physically mounted units.
type ParameterArea uint16

const (
	ParameterPLCSetup     ParameterArea = 0x8010
	ParameterIOTable      ParameterArea = 0x8012
	ParameterRoutingTable ParameterArea = 0x8013
	ParameterCPUBusSetup  ParameterArea = 0x8002
)

// ByteLength returns the documented size of the complete area, or zero for an unknown area.
func (a ParameterArea) ByteLength() int { return 2 * a.words() }

func (a ParameterArea) words() int {
	switch a {
	case ParameterPLCSetup, ParameterRoutingTable:
		return 512
	case ParameterIOTable:
		return 1280
	case ParameterCPUBusSetup:
		return 5184
	default:
		return 0
	}
}

// ReadParameterArea reads one entire documented area using only 0201.
// It requires exact echoes, lengths and the last-word flag at the documented
// boundary. Errors return no partial backup and are never retried. Returned
// bytes retain wire word order. Multiple exchanges are not an atomic snapshot:
// callers must exclude concurrent configuration changes and compare copies.
// A parameter-area copy is not a full controller backup or restore guarantee.
func (c *Client) ReadParameterArea(ctx context.Context, area ParameterArea) ([]byte, error) {
	words := area.words()
	if words == 0 {
		return nil, fmt.Errorf("unsupported parameter area %04x", uint16(area))
	}
	chunk := min(498, (c.maxReadData()-6)/2)
	if chunk < 1 {
		return nil, errors.New("transport cannot carry a parameter word")
	}
	out := make([]byte, 0, words*2)
	for start := 0; start < words; {
		count := min(chunk, words-start)
		request := make([]byte, 6)
		binary.BigEndian.PutUint16(request, uint16(area))
		binary.BigEndian.PutUint16(request[2:], uint16(start))
		binary.BigEndian.PutUint16(request[4:], uint16(count))
		data, err := c.callValidated(ctx, 0x0201, request, func(b []byte) error {
			if len(b) != 6+count*2 || binary.BigEndian.Uint16(b) != uint16(area) || binary.BigEndian.Uint16(b[2:]) != uint16(start) {
				return errors.New("PARAMETER AREA READ length or echo mismatch")
			}
			flags := binary.BigEndian.Uint16(b[4:])
			if int(flags&0x7fff) != count || (flags&0x8000 != 0) != (start+count == words) {
				return errors.New("PARAMETER AREA READ count or last-word flag mismatch")
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		out = append(out, data[6:]...)
		start += count
	}
	return out, nil
}
