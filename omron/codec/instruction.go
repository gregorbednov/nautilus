package codec

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/joyautomation/nautilus/omron/il"
)

var ErrUnqualifiedProfile = errors.New("unqualified native codec profile")
var ErrUnqualifiedInstruction = errors.New("unqualified native instruction form")

// Profile describes the observed library configuration, not device permission.
// Only the two listed models with unknown unit version are currently covered.
// Specific unit versions require separate qualification before deployment.
type Profile struct {
	Model            string
	UnitVersion      string
	TimerCounterMode TimerCounterMode
	// InOutBOOLVersion selects the measured FB extension independently of
	// general version-sensitive instruction support.
	InOutBOOLVersion string `json:",omitempty"`
}

// SupportsInOutBOOL applies only to the measured single-port Ladder inversion.
func (profile Profile) SupportsInOutBOOL() bool {
	return profile.Model == CJ1HCPU65 && profile.TimerCounterMode == TimerModeBCD &&
		((profile.UnitVersion == "4.0" && profile.InOutBOOLVersion == "") ||
			(profile.UnitVersion == "" && profile.InOutBOOLVersion == "4.0"))
}

// TimerCounterMode is required for timer/counter instructions. The zero value
// leaves their interpretation unknown, even when library defaults were binary.
type TimerCounterMode string

const (
	TimerModeBCD    TimerCounterMode = "bcd"
	TimerModeBinary TimerCounterMode = "binary"
)

const (
	CJ1MCPU21 = "CJ1M CPU21"
	CJ1HCPU65 = "CJ1H-H CPU65"
)

func (profile Profile) validate() error {
	if profile.InOutBOOLVersion != "" && (profile.InOutBOOLVersion != "4.0" || profile.Model != CJ1HCPU65 || profile.TimerCounterMode != TimerModeBCD) {
		return ErrUnqualifiedProfile
	}
	if profile.UnitVersion != "" || (profile.Model != CJ1MCPU21 && profile.Model != CJ1HCPU65) {
		return ErrUnqualifiedProfile
	}
	if profile.TimerCounterMode != "" && profile.TimerCounterMode != TimerModeBCD && profile.TimerCounterMode != TimerModeBinary {
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
	case "CMP":
		return label == "020" || label == "20"
	case "TIMH":
		return label == "015"
	case "TIMX":
		return label == "550"
	case "TIMHX":
		return label == "551"
	case "CNTX":
		return label == "546"
	case "IL":
		return label == "002"
	case "ILC":
		return label == "003"
	case "NOT":
		return label == "520"
	case "UP":
		return label == "521"
	case "DOWN":
		return label == "522"
	}
	return false
}

// MinimumCJ1UnitVersion reports known firmware requirements for CJ1M CPU21 and
// CJ1H-H CPU65 source mnemonics. An empty result means no requirement has been
// recorded. It does not validate operand addresses or imply encoder support.
func MinimumCJ1UnitVersion(instruction il.Instruction) string {
	if len(instruction.Operands) != 2 || (instruction.Modifier != "" && instruction.Modifier != "@") {
		return ""
	}
	code := conversionFunctionCodes[strings.ToUpper(instruction.Mnemonic)]
	if code != "" && (instruction.Function == "" || instruction.Function == code) {
		return "4.0"
	}
	return ""
}

var conversionFunctionCodes = map[string]string{
	"STR4": "601", "STR8": "602", "STR16": "603",
	"NUM4": "604", "NUM8": "605", "NUM16": "606",
}

// EncodeInstruction produces numeric 32-bit instruction containers, not an N1
// image or a wire payload. Unqualified modifiers, symbols, instruction families and
// unqualified labels fail explicitly. It performs no implicit address allocation.
func EncodeInstruction(profile Profile, instruction il.Instruction) ([]uint32, error) {
	if err := profile.validate(); err != nil {
		return nil, err
	}
	mnemonic := strings.ToUpper(instruction.Mnemonic)
	if version := MinimumCJ1UnitVersion(instruction); version != "" {
		return nil, fmt.Errorf("%w: %s requires CJ1 unit version %s or later", ErrUnqualifiedInstruction, mnemonic, version)
	}
	if !qualifierAllowed(mnemonic, instruction.Function) {
		return nil, ErrUnqualifiedInstruction
	}
	if instruction.Modifier != "" {
		return encodeModified(profile, instruction)
	}
	if profile.TimerCounterMode == TimerModeBCD && mnemonic == "LD" && len(instruction.Operands) == 1 && strings.EqualFold(instruction.Operands[0], "CF113") {
		return []uint32{ldCF113}, nil
	}
	for _, definition := range timerInstructions {
		if mnemonic == definition.bcd || mnemonic == definition.binary {
			return encodeTimer(profile, definition, mnemonic, instruction.Operands)
		}
	}
	if mnemonic == "MOV" {
		return encodeMOV(instruction.Operands)
	}
	if mnemonic == "CMP" {
		return encodeCMP(instruction.Operands)
	}
	for _, definition := range conditionInstructions {
		if mnemonic == definition.name {
			if len(instruction.Operands) != 0 {
				return nil, ErrUnqualifiedInstruction
			}
			return slices.Clone(definition.words), nil
		}
	}
	for _, definition := range branchInstructions {
		if mnemonic != definition.name {
			continue
		}
		if len(instruction.Operands) != 0 {
			return nil, ErrUnqualifiedInstruction
		}
		return []uint32{definition.opcode}, nil
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
		if strings.HasPrefix(strings.ToUpper(instruction.Operands[0]), "TR") {
			if mnemonic != "LD" && mnemonic != "OUT" {
				return nil, ErrUnqualifiedInstruction
			}
			relay, err := ParseTemporaryRelay(instruction.Operands[0])
			if err != nil {
				return nil, err
			}
			return []uint32{definition.opcode | 0x122000 | uint32(relay)}, nil
		}
		address, err := ParseBitAddress(instruction.Operands[0])
		if err != nil {
			return nil, err
		}
		// W474 restricts direct DM bit operands in these forms to CJ2. The
		// instruction-unit library still emits them for CJ1 without ProgramCheck.
		if address.Area == Data {
			return nil, ErrUnqualifiedInstruction
		}
		bits, err := address.operandBits(definition.write)
		if err != nil {
			return nil, err
		}
		return []uint32{definition.opcode | bits}, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrUnqualifiedInstruction, mnemonic)
}

// Native CPU21 task 256 uses this contact at word offsets 0, 22 and 44.
const ldCF113 uint32 = 0xe212400f

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
	if profile.TimerCounterMode == TimerModeBCD && words[0] == ldCF113 {
		ins, err := il.Parse("LD CF113")
		return ins, 1, err
	}
	// Recognize only complete measured macros before the generic differentiated
	// contact decoder. Do not expose their internal field as a memory operand.
	for _, definition := range conditionInstructions {
		if len(words) >= len(definition.words) && slices.Equal(words[:len(definition.words)], definition.words) {
			instruction, err := il.Parse(definition.name)
			if err != nil {
				return fail(err)
			}
			return instruction, len(definition.words), nil
		}
	}
	if words[0]&0xffe00000 == 0xe2c00000 || words[0] == 0xfac00000 || words[0] == 0xfae00000 {
		return decodeModified(profile, words)
	}
	var source string
	consumed := 1
	for _, definition := range branchInstructions {
		if words[0] == definition.opcode {
			instruction, err := il.Parse(definition.name)
			if err != nil {
				return fail(err)
			}
			return instruction, 1, nil
		}
	}
	for _, definition := range timerInstructions {
		if words[0] == definition.opcode {
			source, err := decodeTimer(profile, definition, words)
			if err != nil {
				return fail(err)
			}
			instruction, err := il.Parse(source)
			if err != nil {
				return fail(err)
			}
			return instruction, 3, nil
		}
	}
	switch words[0] {
	case 0xb2400100:
		var err error
		source, err = decodeCMP(words)
		if err != nil {
			return fail(err)
		}
		consumed = 3
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
			if field := words[0] & 0x1fffff; field&0x1ffff0 == 0x122000 {
				if definition.name != "LD" && definition.name != "OUT" {
					return fail(ErrUnqualifiedInstruction)
				}
				source = definition.name + " " + TemporaryRelay(field&15).String()
				break
			}
			address, err := addressFromBits(words[0] & 0x1fffff)
			if err != nil {
				return fail(err)
			}
			if err := address.validate(definition.write); err != nil {
				return fail(err)
			}
			if address.Area == Data {
				return fail(ErrUnqualifiedInstruction)
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
