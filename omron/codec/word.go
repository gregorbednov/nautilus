package codec

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var ErrWordOperand = errors.New("invalid or unqualified native word operand")

// WordAddress designates a present value for T/C, rather than a completion bit.
type WordAddress struct {
	Area Area
	Word uint16
}

var wordRanges = []struct {
	area Area
	base uint32
	max  uint16
}{
	{CIO, 0xc000, 6143}, {Work, 0xde00, 511}, {Holding, 0xd800, 511},
	{Auxiliary, 0xb840, 959}, {Data, 0x10000, 32767},
	{Timer, 0xe000, 4095}, {Counter, 0xf000, 4095},
}

func (address WordAddress) field(write bool) (uint32, error) {
	for _, region := range wordRanges {
		if address.Area == region.area {
			if address.Word > region.max || (write && address.Area == Auxiliary && address.Word < 448) {
				return 0, ErrWordOperand
			}
			return region.base + uint32(address.Word), nil
		}
	}
	return 0, ErrWordOperand
}

func ParseWordAddress(source string) (WordAddress, error) {
	// Reuse the direct address grammar and bounds, without accepting bit suffixes.
	if strings.Contains(source, ".") {
		return WordAddress{}, ErrWordOperand
	}
	bit, err := ParseBitAddress(source)
	if err != nil {
		return WordAddress{}, ErrWordOperand
	}
	return WordAddress{Area: bit.Area, Word: bit.Word}, nil
}

func (address WordAddress) String() string {
	if _, err := address.field(false); err != nil {
		return "<invalid-word-address>"
	}
	prefix := map[Area]string{CIO: "", Work: "W", Holding: "H", Auxiliary: "A", Data: "D", Timer: "T", Counter: "C"}[address.Area]
	return fmt.Sprintf("%s%d", prefix, address.Word)
}

func wordAddressFromField(field uint32, write bool) (WordAddress, error) {
	for _, region := range wordRanges {
		if field >= region.base && field-region.base <= uint32(region.max) {
			address := WordAddress{region.area, uint16(field - region.base)}
			_, err := address.field(write)
			return address, err
		}
	}
	return WordAddress{}, ErrWordOperand
}

// Literal16 preserves the source radix marker: # and & encode differently.
type Literal16 struct {
	Value   uint16
	Decimal bool
}

func ParseLiteral16(source string) (Literal16, error) {
	if len(source) < 2 || len(source) > 64 || (source[0] != '#' && source[0] != '&') {
		return Literal16{}, ErrWordOperand
	}
	base := 16
	if source[0] == '&' {
		base = 10
	}
	for _, ch := range source[1:] {
		if (ch < '0' || ch > '9') && (base != 16 || !strings.ContainsRune("abcdefABCDEF", ch)) {
			return Literal16{}, ErrWordOperand
		}
	}
	value, err := strconv.ParseUint(source[1:], base, 16)
	if err != nil {
		return Literal16{}, ErrWordOperand
	}
	return Literal16{uint16(value), base == 10}, nil
}

func (literal Literal16) String() string {
	if literal.Decimal {
		return fmt.Sprintf("&%d", literal.Value)
	}
	return fmt.Sprintf("#%04X", literal.Value)
}

func encodeMOV(operands []string) ([]uint32, error) {
	if len(operands) != 2 {
		return nil, ErrUnqualifiedInstruction
	}
	var source uint32
	if strings.HasPrefix(operands[0], "#") || strings.HasPrefix(operands[0], "&") {
		literal, err := ParseLiteral16(operands[0])
		if err != nil {
			return nil, err
		}
		source = 0x61400000 | uint32(literal.Value)
		if literal.Decimal {
			source |= 0x40000
		}
	} else {
		address, err := ParseWordAddress(operands[0])
		if err != nil {
			return nil, err
		}
		field, err := address.field(false)
		if err != nil {
			return nil, err
		}
		source = 0x60000000 | field
	}
	address, err := ParseWordAddress(operands[1])
	if err != nil {
		return nil, err
	}
	field, err := address.field(true)
	if err != nil {
		return nil, err
	}
	return []uint32{0xb2400140, source, 0x54000000 | field}, nil
}

func decodeMOV(words []uint32) (string, error) {
	if len(words) < 3 {
		return "", ErrUnqualifiedInstruction
	}
	var source string
	switch words[1] & 0xffff0000 {
	case 0x61400000, 0x61440000:
		source = (Literal16{uint16(words[1]), words[1]&0x40000 != 0}).String()
	default:
		if words[1]&0xfffe0000 != 0x60000000 {
			return "", ErrWordOperand
		}
		address, err := wordAddressFromField(words[1]&0x1ffff, false)
		if err != nil {
			return "", err
		}
		source = address.String()
	}
	if words[2]&0xfffe0000 != 0x54000000 {
		return "", ErrWordOperand
	}
	destination, err := wordAddressFromField(words[2]&0x1ffff, true)
	if err != nil {
		return "", err
	}
	return "MOV(021) " + source + " " + destination.String(), nil
}
