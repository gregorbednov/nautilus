// Package codec encodes and decodes the currently qualified native instruction
// forms. It does not assemble a project, allocate FBs or produce a PLC transfer.
package codec

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var ErrBitAddress = errors.New("invalid or unqualified native bit address")

type Area uint8

const (
	UnknownArea Area = iota
	CIO
	Work
	Holding
	Auxiliary
	Data
	Timer
	Counter
)

// BitAddress has instruction semantics. Bare T/C designate completion flags,
// unlike fins.Memory's bare T/C present values. Numeric CIO uses native IL
// spelling (e.g. 100.01); the explicit CIO prefix is a different source dialect.
type BitAddress struct {
	Area Area
	Word uint16
	Bit  uint8
}

func (address BitAddress) validate(write bool) error {
	if address.Bit > 15 {
		return ErrBitAddress
	}
	max := uint16(0)
	switch address.Area {
	case CIO:
		max = 6143
	case Work, Holding:
		max = 511
	case Auxiliary:
		max = 959
	case Data:
		max = 32767
	case Timer, Counter:
		max = 4095
		if write || address.Bit != 0 {
			return fmt.Errorf("%w: completion flag access", ErrBitAddress)
		}
	default:
		return ErrBitAddress
	}
	if address.Word > max {
		return fmt.Errorf("%w: index outside qualified range", ErrBitAddress)
	}
	if write && address.Area == Auxiliary && address.Word < 448 {
		return fmt.Errorf("%w: read-only auxiliary range", ErrBitAddress)
	}
	return nil
}

func decimal(source string, bits int) (uint64, error) {
	if source == "" {
		return 0, ErrBitAddress
	}
	for _, ch := range source {
		if ch < '0' || ch > '9' {
			return 0, ErrBitAddress
		}
	}
	value, err := strconv.ParseUint(source, 10, bits)
	if err != nil {
		return 0, ErrBitAddress
	}
	return value, nil
}

func ParseBitAddress(source string) (BitAddress, error) {
	address := BitAddress{Area: CIO}
	if len(source) == 0 || len(source) > 64 {
		return BitAddress{}, ErrBitAddress
	}
	text := strings.ToUpper(source)
	switch text[0] {
	case 'W':
		address.Area = Work
		text = text[1:]
	case 'H':
		address.Area = Holding
		text = text[1:]
	case 'A':
		address.Area = Auxiliary
		text = text[1:]
	case 'D':
		address.Area = Data
		text = text[1:]
	case 'T':
		address.Area = Timer
		text = text[1:]
	case 'C':
		address.Area = Counter
		text = text[1:]
	}
	word, bit, hasBit := strings.Cut(text, ".")
	index, err := decimal(word, 16)
	if err != nil {
		return BitAddress{}, fmt.Errorf("%w: %q", ErrBitAddress, source)
	}
	address.Word = uint16(index)
	if hasBit {
		if address.Area == Timer || address.Area == Counter || len(bit) > 2 {
			return BitAddress{}, ErrBitAddress
		}
		value, err := decimal(bit, 4)
		if err != nil {
			return BitAddress{}, ErrBitAddress
		}
		address.Bit = uint8(value)
	}
	if err := address.validate(false); err != nil {
		return BitAddress{}, err
	}
	return address, nil
}

// String returns canonical native IL spelling for a validated address.
func (address BitAddress) String() string {
	if address.validate(false) != nil {
		return "<invalid-bit-address>"
	}
	prefix := map[Area]string{CIO: "", Work: "W", Holding: "H", Auxiliary: "A", Data: "D", Timer: "T", Counter: "C"}[address.Area]
	if address.Area == Timer || address.Area == Counter {
		return fmt.Sprintf("%s%d", prefix, address.Word)
	}
	return fmt.Sprintf("%s%d.%02d", prefix, address.Word, address.Bit)
}

func (address BitAddress) operandBits(write bool) (uint32, error) {
	if err := address.validate(write); err != nil {
		return 0, err
	}
	base := map[Area]uint32{CIO: 0x40000, Work: 0x5e000, Holding: 0x58000, Auxiliary: 0x38400, Data: 0x80000, Timer: 0x3e000, Counter: 0x3f000}[address.Area]
	if address.Area == Timer || address.Area == Counter {
		return base + uint32(address.Word), nil
	}
	return base + uint32(address.Word)*16 + uint32(address.Bit), nil
}

func addressFromBits(bits uint32) (BitAddress, error) {
	for _, region := range []struct {
		area        Area
		first, last uint32
	}{
		{Auxiliary, 0x38400, 0x3bfff}, {Timer, 0x3e000, 0x3efff}, {Counter, 0x3f000, 0x3ffff},
		{CIO, 0x40000, 0x57fff}, {Holding, 0x58000, 0x59fff}, {Work, 0x5e000, 0x5ffff}, {Data, 0x80000, 0xfffff},
	} {
		if bits < region.first || bits > region.last {
			continue
		}
		offset := bits - region.first
		if region.area == Timer || region.area == Counter {
			return BitAddress{Area: region.area, Word: uint16(offset)}, nil
		}
		return BitAddress{Area: region.area, Word: uint16(offset / 16), Bit: uint8(offset % 16)}, nil
	}
	return BitAddress{}, ErrBitAddress
}
