package fbcode

import (
	"strings"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/il"
)

// ScalarType names the measured native StringToDataType forms, not CXP numeric
// PouVarList codes. Mapping CXP declarations into these types remains separate.
type ScalarType string

const (
	Bool ScalarType = "BOOL"
	Word ScalarType = "WORD"
	Uint ScalarType = "UINT"
	Int  ScalarType = "INT"
)

type ParameterUsage uint8

const (
	Input  ParameterUsage = 1
	Output ParameterUsage = 2
)

// InOutBOOLAddress pushes the address of a W-area bit for the four-cell call.
func InOutBOOLAddress(source string) ([]uint32, error) {
	address, err := codec.ParseBitAddress(source)
	if err != nil || address.Area != codec.Work || !strings.Contains(source, ".") {
		return nil, ErrPostamble
	}
	field := (0xde00+uint32(address.Word))<<4 | uint32(address.Bit)
	return []uint32{0xb2410144, 0x52000000 | field, 0x5d17f00a}, nil
}

// ScalarTransfer encodes one scalar copy before invocation postamble processing.
// It reproduces the measured AddOperandPreamble count1 path: inputs push through
// IR62/63; outputs pop. It does not allocate these registers, validate the wider
// caller state, or compile graphical pins. INOUT, arrays, implicit EN/ENO,
// constants for BOOL and private FB addresses remain outside this contract.
func ScalarTransfer(selector uint32, dataType ScalarType, usage ParameterUsage, source string) ([]uint32, error) {
	if selector != 8 || (usage != Input && usage != Output) || source == "" || len(source) > 64 || strings.ContainsAny(source, " \t\r\n") {
		return nil, ErrPostamble
	}
	if dataType == Bool {
		if !strings.Contains(source, ".") {
			return nil, ErrPostamble
		}
		address, err := codec.ParseBitAddress(source)
		if err != nil {
			return nil, ErrPostamble
		}
		// BMOV uses the physical word address followed by four bit-index bits,
		// unlike the compact public contact address encoding.
		base, ok := map[codec.Area]uint32{codec.CIO: 0xc000, codec.Work: 0xde00, codec.Holding: 0xd800}[address.Area]
		if !ok {
			return nil, ErrPostamble
		}
		field := (base+uint32(address.Word))<<4 | uint32(address.Bit)
		if usage == Input {
			return []uint32{0xb2410147, 0x62000000 | field, 0x5717e009}, nil
		}
		return []uint32{0xb2410147, 0x6317e001, 0x56000000 | field}, nil
	}
	if dataType != Word && dataType != Uint && dataType != Int {
		return nil, ErrPostamble
	}
	// Share the qualified public 16-bit operand grammar/encoding. Only the
	// measured CIO/W/H/D areas are enabled for these generated transfers.
	if source[0] != '#' && source[0] != '&' {
		address, err := codec.ParseWordAddress(source)
		if err != nil || (address.Area != codec.CIO && address.Area != codec.Work && address.Area != codec.Holding && address.Area != codec.Data) {
			return nil, ErrPostamble
		}
	}
	text := "MOV " + source + " D0"
	if usage == Output {
		text = "MOV #0 " + source
	}
	instruction, err := il.Parse(text)
	if err != nil {
		return nil, ErrPostamble
	}
	words, err := codec.EncodeInstruction(codec.Profile{Model: "CJ1M CPU21"}, instruction)
	if err != nil {
		return nil, ErrPostamble
	}
	if usage == Input {
		return []uint32{0xb2410140, words[1], 0x5517f009}, nil
	}
	return []uint32{0xb2410140, 0x6117f001, words[2]}, nil
}
