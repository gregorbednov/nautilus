package fins

import (
	"bytes"
	"context"
	"errors"
	"strings"
)

// ConnectedUnit is a 0502 address/model record. Address is a FINS unit address,
// not a physical slot. Basic I/O modules are not enumerated by this command.
type ConnectedUnit struct {
	Address  byte   `json:"address"`
	Model    string `json:"model"`
	RawModel []byte `json:"rawModel"`
}

func validConnectionAddress(a byte) bool {
	return a == 0 || (a >= 0x10 && a <= 0x7f) || a == 0xe1
}

// ReadConnections enumerates CPU/CPU Bus/Special I/O/Inner Board model records
// using W342 0502, starting with the CPU. Requests always specify a bounded
// count. Ordered, forward-only pagination must reach the last-unit flag.
// Errors return no partial inventory and are not retried. Multiple pages are
// not an atomic snapshot, and this is not a complete physical rack inventory.
func (c *Client) ReadConnections(ctx context.Context) ([]ConnectedUnit, error) {
	limit := c.maxReadData()
	if limit < 22 {
		return nil, errors.New("transport cannot carry a connection record")
	}
	count := min(25, (limit-1)/21)
	start := byte(0)
	var units []ConnectedUnit
	for {
		data, err := c.callValidated(ctx, 0x0502, []byte{start, byte(count)}, func(b []byte) error {
			if len(b) < 1 {
				return errors.New("CONNECTION DATA READ missing count")
			}
			n := int(b[0] & 0x7f)
			if n == 0 || n > count || len(b) != 1+n*21 {
				return errors.New("CONNECTION DATA READ invalid count or length")
			}
			last := int(start) - 1
			for i := 0; i < n; i++ {
				record := b[1+i*21 : 1+(i+1)*21]
				a := record[0]
				if !validConnectionAddress(a) || int(a) <= last {
					return errors.New("CONNECTION DATA READ invalid or unordered address")
				}
				if len(units) == 0 && i == 0 && a != 0 {
					return errors.New("CONNECTION DATA READ missing CPU record")
				}
				model := strings.Trim(string(record[1:]), " \x00")
				if model == "" {
					return errors.New("CONNECTION DATA READ empty model")
				}
				for _, v := range []byte(model) {
					if v < 0x20 || v > 0x7e {
						return errors.New("CONNECTION DATA READ model is not printable ASCII")
					}
				}
				last = int(a)
			}
			if last == 0xe1 && b[0]&0x80 == 0 {
				return errors.New("CONNECTION DATA READ no last flag at final address")
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		for i := 0; i < int(data[0]&0x7f); i++ {
			record := data[1+i*21 : 1+(i+1)*21]
			units = append(units, ConnectedUnit{Address: record[0], Model: strings.Trim(string(record[1:]), " \x00"), RawModel: bytes.Clone(record[1:])})
		}
		if data[0]&0x80 != 0 {
			return units, nil
		}
		last := units[len(units)-1].Address
		switch last {
		case 0:
			start = 0x10
		case 0x7f:
			start = 0xe1
		default:
			start = last + 1
		}
	}
}
