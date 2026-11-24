package codec

import (
	"errors"
	"fmt"
)

var ErrFBRelativeBody = errors.New("unsupported FB relative instruction body")

// FBRelativeOperand retains the base register and signed displacement. Unit
// describes how this instruction uses the displacement, not a physical address.
// Reading it does not allocate an instance or qualify the operand for encoding.
type FBRelativeOperand struct {
	BaseRegister int    `json:"baseRegister"`
	Offset       int    `json:"offset"`
	Unit         string `json:"unit"`
}

type FBRelativeInstruction struct {
	Mnemonic    string              `json:"mnemonic"`
	Function    string              `json:"function,omitempty"`
	OffsetWords int                 `json:"offsetWords"`
	LengthWords int                 `json:"lengthWords"`
	Operands    []FBRelativeOperand `json:"operands"`
}

// EncodeFBRelativeInstruction emits only the short forms accepted by
// DecodeFBRelativeBody. Offsets must already be allocated and fit signed12.
// Span fields are inspection metadata and do not participate in encoding.
// Extended displacements, other banks/forms and instance allocation are absent.
func EncodeFBRelativeInstruction(profile Profile, instruction FBRelativeInstruction) ([]uint32, error) {
	if err := profile.validate(); err != nil {
		return nil, err
	}
	fail := func() ([]uint32, error) {
		return nil, fmt.Errorf("%w: unqualified relative %s operands", ErrFBRelativeBody, instruction.Mnemonic)
	}
	for _, operand := range instruction.Operands {
		if operand.Offset < -2048 || operand.Offset > 2047 {
			return fail()
		}
	}
	if instruction.Mnemonic == "TIMH" {
		if profile.TimerCounterMode != TimerModeBCD || (instruction.Function != "" && instruction.Function != "015") || len(instruction.Operands) != 2 {
			return fail()
		}
		timer, preset := instruction.Operands[0], instruction.Operands[1]
		if timer.BaseRegister != 0 || timer.Unit != "timer" || preset.BaseRegister != 3 || preset.Unit != "word" {
			return fail()
		}
		return []uint32{0xbb4000c1, 0x513c0000 | uint32(timer.Offset)&0xfff, 0x653c3000 | uint32(preset.Offset)&0xfff}, nil
	}
	if instruction.Function != "" || len(instruction.Operands) != 1 {
		return fail()
	}
	operand := instruction.Operands[0]
	if instruction.Mnemonic == "LD" && operand.BaseRegister == 0 && operand.Unit == "timer" {
		return []uint32{0xe21f0000 | uint32(operand.Offset)&0xfff}, nil
	}
	if operand.BaseRegister != 3 || operand.Unit != "bit" {
		return fail()
	}
	var opcode uint32
	switch instruction.Mnemonic {
	case "LD":
		opcode = 0xe21f3000
	case "LDNOT":
		opcode = 0xe23f3000
	case "AND":
		opcode = 0xe25f3000
	case "ANDNOT":
		opcode = 0xe27f3000
	case "OUT":
		opcode = 0xfa1f3000
	case "SET":
		opcode = 0xf05f3000
	case "RSET":
		opcode = 0xf07f3000
	default:
		return fail()
	}
	return []uint32{opcode | uint32(operand.Offset)&0xfff}, nil
}

// EncodeFBRelativeBody assembles preallocated typed relative instructions. It
// does not infer bindings or include FBBN/FBBRET, task records or an N1 image.
func EncodeFBRelativeBody(profile Profile, instructions []FBRelativeInstruction) ([]uint32, error) {
	if err := profile.validate(); err != nil {
		return nil, err
	}
	if len(instructions) == 0 || len(instructions) > 65536 {
		return nil, fmt.Errorf("%w: invalid instruction count", ErrFBRelativeBody)
	}
	var result []uint32
	for i, instruction := range instructions {
		words, err := EncodeFBRelativeInstruction(profile, instruction)
		if err != nil {
			return nil, fmt.Errorf("instruction %d: %w", i, err)
		}
		if len(result) > 65536-len(words) {
			return nil, fmt.Errorf("%w: body length limit", ErrFBRelativeBody)
		}
		result = append(result, words...)
	}
	return result, nil
}

// DecodeFBRelativeBody reads the observed BLINK instruction forms, excluding
// FBBN/FBBRET wrappers. It accepts BR3 bit logic, LD of BR0 timer completion and
// BCD TIMH with a BR0 timer/BR3 word preset. Other forms fail atomically.
// This is a bounded decoder, not a definition compiler or allocation algorithm.
func DecodeFBRelativeBody(profile Profile, words []uint32) ([]FBRelativeInstruction, error) {
	if err := profile.validate(); err != nil {
		return nil, err
	}
	if len(words) == 0 || len(words) > 65536 {
		return nil, fmt.Errorf("%w: invalid body length", ErrFBRelativeBody)
	}
	result := make([]FBRelativeInstruction, 0)
	reference := func(bank int, word uint32, unit string) FBRelativeOperand {
		offset := int(word & 0xfff)
		if offset >= 0x800 {
			offset -= 0x1000
		}
		return FBRelativeOperand{bank, offset, unit}
	}
	for offset := 0; offset < len(words); {
		word := words[offset]
		instruction := FBRelativeInstruction{OffsetWords: offset, LengthWords: 1}
		switch word & 0xfffff000 {
		case 0xe21f3000:
			instruction.Mnemonic = "LD"
		case 0xe23f3000:
			instruction.Mnemonic = "LDNOT"
		case 0xe25f3000:
			instruction.Mnemonic = "AND"
		case 0xe27f3000:
			instruction.Mnemonic = "ANDNOT"
		case 0xfa1f3000:
			instruction.Mnemonic = "OUT"
		case 0xf05f3000:
			instruction.Mnemonic = "SET"
		case 0xf07f3000:
			instruction.Mnemonic = "RSET"
		case 0xe21f0000:
			instruction.Mnemonic = "LD"
			instruction.Operands = []FBRelativeOperand{reference(0, word, "timer")}
		}
		if instruction.Mnemonic != "" && len(instruction.Operands) == 0 {
			instruction.Operands = []FBRelativeOperand{reference(3, word, "bit")}
		}
		if word == 0xbb4000c1 && profile.TimerCounterMode == TimerModeBCD && len(words)-offset >= 3 && words[offset+1]&0xfffff000 == 0x513c0000 && words[offset+2]&0xfffff000 == 0x653c3000 {
			instruction.Mnemonic = "TIMH"
			instruction.Function = "015"
			instruction.LengthWords = 3
			instruction.Operands = []FBRelativeOperand{reference(0, words[offset+1], "timer"), reference(3, words[offset+2], "word")}
		}
		if instruction.Mnemonic == "" {
			return nil, fmt.Errorf("%w: word %d (%08x)", ErrFBRelativeBody, offset, word)
		}
		result = append(result, instruction)
		offset += instruction.LengthWords
	}
	return result, nil
}
