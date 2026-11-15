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
