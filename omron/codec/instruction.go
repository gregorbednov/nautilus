package codec

import (
	"errors"
	"fmt"
	"strings"

	"github.com/joyautomation/nautilus/omron/il"
)

var ErrUnqualifiedProfile = errors.New("unqualified native codec profile")
var ErrUnqualifiedInstruction = errors.New("unqualified native instruction form")

// Profile describes the observed library configuration, not device permission.
// Only the two listed models with unknown unit version are currently covered.
// Specific unit versions require separate qualification before deployment.
type Profile struct {
	Model       string
	UnitVersion string
}

const (
	CJ1MCPU21 = "CJ1M CPU21"
	CJ1HCPU65 = "CJ1H-H CPU65"
)

func (profile Profile) validate() error {
	if profile.UnitVersion != "" || (profile.Model != CJ1MCPU21 && profile.Model != CJ1HCPU65) {
		return ErrUnqualifiedProfile
	}
	return nil
}

type bitInstruction struct {
	name   string
	opcode uint32
	write  bool
}

var bitInstructions = []bitInstruction{
	{"LD", 0xe2000000, false}, {"LDNOT", 0xe2200000, false},
	{"AND", 0xe2400000, false}, {"ANDNOT", 0xe2600000, false},
	{"OR", 0xe2800000, false}, {"ORNOT", 0xe2a00000, false},
	{"OUT", 0xfa000000, true}, {"OUTNOT", 0xfa200000, true},
	{"SET", 0xf0400000, true}, {"RSET", 0xf0600000, true},
}

func qualifierAllowed(mnemonic, label string) bool {
	if label == "" {
		return true
	}
	switch mnemonic {
	case "LD":
		return label == "000"
	case "END":
		return label == "001" || label == "1"
	case "NOP":
		return label == "000"
	case "MOV":
		return label == "021" || label == "21"
	}
	return false
}

// EncodeInstruction produces numeric 32-bit instruction containers, not an N1
// image or a wire payload. Modifiers, symbols, other instruction families and
// unqualified labels fail explicitly. It performs no implicit address allocation.
func EncodeInstruction(profile Profile, instruction il.Instruction) ([]uint32, error) {
	if err := profile.validate(); err != nil {
		return nil, err
	}
	mnemonic := strings.ToUpper(instruction.Mnemonic)
	if instruction.Modifier != "" || !qualifierAllowed(mnemonic, instruction.Function) {
		return nil, ErrUnqualifiedInstruction
	}
	if mnemonic == "MOV" {
		return encodeMOV(instruction.Operands)
	}
	if mnemonic == "END" || mnemonic == "NOP" {
		if len(instruction.Operands) != 0 {
			return nil, ErrUnqualifiedInstruction
		}
		if mnemonic == "END" {
			return []uint32{0xbb800001}, nil
		}
		return []uint32{0xa0000000}, nil
	}
	for _, definition := range bitInstructions {
		if mnemonic != definition.name {
			continue
		}
		if len(instruction.Operands) != 1 {
			return nil, ErrUnqualifiedInstruction
		}
		address, err := ParseBitAddress(instruction.Operands[0])
		if err != nil {
			return nil, err
		}
		bits, err := address.operandBits(definition.write)
		if err != nil {
			return nil, err
		}
		return []uint32{definition.opcode | bits}, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrUnqualifiedInstruction, mnemonic)
}

// DecodeInstruction decodes one qualified form. On failure it consumes zero
// containers so callers can preserve the original opaque sequence unchanged.
func DecodeInstruction(profile Profile, words []uint32) (il.Instruction, int, error) {
	fail := func(err error) (il.Instruction, int, error) { return il.Instruction{}, 0, err }
	if err := profile.validate(); err != nil {
		return fail(err)
	}
	if len(words) == 0 {
		return fail(ErrUnqualifiedInstruction)
	}
	var source string
	consumed := 1
	switch words[0] {
	case 0xb2400140:
		var err error
		source, err = decodeMOV(words)
		if err != nil {
			return fail(err)
		}
		consumed = 3
	case 0xbb800001:
		source = "END"
	case 0xa0000000:
		source = "NOP"
	default:
		for _, definition := range bitInstructions {
			if words[0]&0xffe00000 != definition.opcode {
				continue
			}
			address, err := addressFromBits(words[0] & 0x1fffff)
			if err != nil {
				return fail(err)
			}
			if err := address.validate(definition.write); err != nil {
				return fail(err)
			}
			source = definition.name + " " + address.String()
			break
		}
	}
	if source == "" {
		return fail(ErrUnqualifiedInstruction)
	}
	instruction, err := il.Parse(source)
	if err != nil {
		return fail(err)
	}
	return instruction, consumed, nil
}
