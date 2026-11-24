package fins

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Memory identifies an explicit word, bit, or timer/counter completion flag.
// Limits below are wire-family limits; CPU-specific limits must be checked
// against the target profile before compilation or deployment.
type Memory struct {
	Area      byte
	Word      uint16
	Bit       byte
	BitAccess bool
}

// ParseMemory accepts CIO100.01 (or 100.01), W, H, A, D, and E<hex>_<word>.
// T/C without a suffix designate their present value, with .Q their completion
// flag. T/C flags do not share the numbered word's bit-access semantics.
func ParseMemory(text string) (Memory, error) {
	s := strings.ToUpper(strings.TrimSpace(text))
	m := Memory{}
	wordMax := 32767
	wordCode, bitCode := byte(0xB0), byte(0x30)
	offset := 0
	if strings.HasPrefix(s, "TK") {
		wordCode, wordMax = 0x06, 31
		s = s[2:]
	} else if strings.HasPrefix(s, "CIO") {
		s = s[3:]
		wordMax = 6143
	} else if strings.HasPrefix(s, "E") {
		bank, rest, ok := strings.Cut(s[1:], "_")
		if !ok {
			return m, errors.New("EM address needs E<bank>_<word>")
		}
		n, err := strconv.ParseUint(bank, 16, 4)
		if err != nil {
			return m, errors.New("EM bank must be 0..F")
		}
		wordCode = 0xA0 + byte(n)
		bitCode = 0x20 + byte(n)
		s = rest
	} else if len(s) > 0 {
		switch s[0] {
		case 'W':
			wordCode, bitCode, wordMax = 0xB1, 0x31, 511
			s = s[1:]
		case 'H':
			wordCode, bitCode, wordMax = 0xB2, 0x32, 1535
			s = s[1:]
		case 'A':
			wordCode, bitCode, wordMax = 0xB3, 0x33, 959
			s = s[1:]
		case 'D':
			wordCode, bitCode = 0x82, 0x02
			s = s[1:]
		case 'T', 'C':
			wordCode, bitCode, wordMax = 0x89, 0x09, 4095
			if s[0] == 'C' {
				offset = 0x8000
			}
			s = s[1:]
		default:
			wordMax = 6143
		}
	}
	word, bit, hasBit := strings.Cut(s, ".")
	if word == "" || strings.IndexFunc(word, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return m, fmt.Errorf("invalid memory address %q", text)
	}
	n, err := strconv.ParseUint(word, 10, 16)
	if err != nil || int(n) > wordMax {
		return m, fmt.Errorf("memory address out of range: %q", text)
	}
	m.Word = uint16(int(n) + offset)
	m.Area = wordCode
	if wordCode == 0x06 {
		if hasBit {
			return m, errors.New("task flag requires a task number without a bit suffix")
		}
		m.BitAccess = true
	}
	if hasBit {
		m.BitAccess = true
		m.Area = bitCode
		if wordCode == 0x89 {
			if bit != "Q" {
				return m, errors.New("timer/counter suffix must be .Q")
			}
			return m, nil
		}
		if bit == "" || strings.IndexFunc(bit, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			return m, errors.New("bit must be 0..15")
		}
		v, err := strconv.ParseUint(bit, 10, 4)
		if err != nil {
			return m, errors.New("bit must be 0..15")
		}
		m.Bit = byte(v)
	}
	return m, nil
}

func (m Memory) elementBytes() int {
	if m.BitAccess {
		return 1
	}
	return 2
}
func (m Memory) validate(count uint16) error {
	if count == 0 || m.Bit > 15 || (!m.BitAccess && m.Bit != 0) {
		return errors.New("invalid memory range")
	}
	last := int(m.Word) + int(count) - 1
	if m.BitAccess && m.Area != 0x09 && m.Area != 0x06 {
		last = int(m.Word) + (int(m.Bit)+int(count)-1)/16
	}
	if last > 65535 {
		return errors.New("memory range overflows address")
	}
	switch m.Area {
	case 0x06:
		if m.Bit != 0 || last > 31 {
			return errors.New("task flag range exceeds TK0..TK31 or has a nonzero bit offset")
		}
	case 0xB0, 0x30:
		if last > 6143 {
			return errors.New("CIO range exceeds 6143")
		}
	case 0xB1, 0x31:
		if last > 511 {
			return errors.New("W range exceeds 511")
		}
	case 0xB2, 0x32:
		if last > 1535 {
			return errors.New("H range exceeds 1535")
		}
	case 0xB3, 0x33:
		if last > 959 {
			return errors.New("A range exceeds 959")
		}
	case 0x82, 0x02:
		if last > 32767 {
			return errors.New("D range exceeds 32767")
		}
	case 0x89, 0x09:
		if int(m.Word)&0x7fff > 4095 || last&0x7fff > 4095 {
			return errors.New("timer/counter range exceeds 4095")
		}
	default:
		if !((m.Area >= 0xA0 && m.Area <= 0xAF) || (m.Area >= 0x20 && m.Area <= 0x2F)) || last > 32767 {
			return errors.New("unsupported memory area/range")
		}
	}
	wordArea := (m.Area >= 0x80)
	if wordArea == m.BitAccess {
		return errors.New("memory area and access width disagree")
	}
	if m.Area == 0x09 && m.Bit != 0 {
		return errors.New("timer/counter flags require bit zero")
	}
	return nil
}

// ReadMemory issues one 0101 request. Transfers are limited to 498 words or
// 996 bits (W342); a transport may impose a smaller MaxReadData size.
func (c *Client) ReadMemory(ctx context.Context, m Memory, count uint16) ([]byte, error) {
	if err := m.validate(count); err != nil {
		return nil, err
	}
	if int(count)*m.elementBytes() > min(996, c.maxReadData()) {
		return nil, errors.New("memory read exceeds transport payload limit")
	}
	data := []byte{m.Area, byte(m.Word >> 8), byte(m.Word), m.Bit, byte(count >> 8), byte(count)}
	return c.callValidated(ctx, 0x0101, data, func(b []byte) error {
		if len(b) != int(count)*m.elementBytes() {
			return errors.New("MEMORY AREA READ length mismatch")
		}
		if m.BitAccess {
			for _, v := range b {
				if v > 1 {
					return errors.New("invalid bit data")
				}
			}
		}
		return nil
	})
}

func (c *Client) maxReadData() int {
	if t, ok := c.transport.(interface{ MaxReadData() int }); ok {
		return min(1000, t.MaxReadData())
	}
	return 1000
}

// ForcedValue separates a single force mask from its observed value (W342 5-2-2).
type ForcedValue struct {
	Mask  uint16 `json:"mask"`
	Value uint16 `json:"value"`
}

func (c *Client) ReadForcedStatus(ctx context.Context, m Memory, count uint16) ([]ForcedValue, error) {
	if err := m.validate(count); err != nil {
		return nil, err
	}
	width := 4
	switch m.Area {
	case 0xB0, 0xB1:
	case 0xB2:
		if int(m.Word)+int(count) > 512 {
			return nil, errors.New("forced status only covers H0..511")
		}
	case 0x09:
		width = 1
	default:
		return nil, errors.New("forced status requires a CIO, W, H word or T/C completion flag")
	}
	if count > 167 || int(count)*max(4, width+1) > c.maxReadData() {
		return nil, errors.New("forced status read exceeds payload limit")
	}
	code := m.Area + 0x40
	req := make([]byte, 0, 4*int(count))
	for i := uint16(0); i < count; i++ {
		addr := m.Word + i
		req = append(req, code, byte(addr>>8), byte(addr), 0)
	}
	raw, err := c.callValidated(ctx, 0x0104, req, func(b []byte) error {
		if len(b) != int(count)*(width+1) {
			return errors.New("forced status response length mismatch")
		}
		for i := 0; i < len(b); i += width + 1 {
			if b[i] != code {
				return errors.New("forced status area echo mismatch")
			}
			if width == 1 && b[i+1] > 3 {
				return errors.New("invalid forced completion flag")
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	values := make([]ForcedValue, count)
	for i := range values {
		data := raw[i*(width+1)+1:]
		if width == 1 {
			values[i] = ForcedValue{uint16(data[0] >> 1), uint16(data[0] & 1)}
		} else {
			values[i] = ForcedValue{binary.BigEndian.Uint16(data), binary.BigEndian.Uint16(data[2:])}
		}
	}
	return values, nil
}

// ReadProgram backs up opaque user memory using 0306. It does not interpret
// memory as source or machine instructions. Every response must echo its
// program number/address and terminate with the documented last-block flag.
// limit bounds memory use and prevents an unending or corrupt transfer.
func (c *Client) ReadProgram(ctx context.Context, limit int) ([]byte, error) {
	if limit < 4 || limit > 32<<20 {
		return nil, errors.New("program limit must be 4..33554432 bytes")
	}
	chunk := min(992, c.maxReadData()-8)
	chunk -= chunk % 4
	if chunk < 4 {
		return nil, errors.New("transport cannot carry a program block")
	}
	var out []byte
	for len(out) < limit {
		n := min(chunk, limit-len(out))
		n -= n % 4
		if n == 0 {
			return nil, errors.New("program exceeds configured byte limit")
		}
		for {
			data := make([]byte, 8)
			data[0], data[1] = 255, 255
			binary.BigEndian.PutUint32(data[2:6], uint32(len(out)))
			binary.BigEndian.PutUint16(data[6:8], uint16(n))
			b, err := c.callValidated(ctx, 0x0306, data, func(b []byte) error {
				if len(b) < 8 || b[0] != 255 || b[1] != 255 || binary.BigEndian.Uint32(b[2:6]) != uint32(len(out)) {
					return errors.New("PROGRAM AREA READ echo mismatch")
				}
				got := int(binary.BigEndian.Uint16(b[6:8]) & 0x7fff)
				if got == 0 || got%4 != 0 || got > n || len(b) != 8+got {
					return errors.New("PROGRAM AREA READ invalid block length")
				}
				return nil
			})
			if err != nil {
				// Some CPUs reject an otherwise valid read that crosses the UM
				// boundary instead of returning a short final block. Retry this
				// read only, at the same address with a smaller aligned count.
				// A range error is never EOF: the final response flag is required.
				var end *EndError
				if n > 4 && errors.As(err, &end) && end.Command == 0x0306 && end.Code == 0x1104 {
					n = (n / 8) * 4
					chunk = n
					continue
				}
				return nil, err
			}
			flags := binary.BigEndian.Uint16(b[6:8])
			out = append(out, b[8:]...)
			if flags&0x8000 != 0 {
				return out, nil
			}
			break
		}
	}
	return nil, errors.New("program has no final block within configured byte limit")
}
