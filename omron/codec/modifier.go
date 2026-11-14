package codec

import (
	"strings"

	"github.com/joyautomation/nautilus/omron/il"
)

// Only these documented forms have both operand and container qualification.
// Prefixes are not applied generically: the library emits unsupported variants.
var edgeInputs = []struct {
	name          string
	plain, suffix uint32
}{
	{"LD", 0xe2000000, 0xc2125002}, {"LDNOT", 0xe2200000, 0xc2325002},
	{"AND", 0xe2400000, 0xc2525002}, {"ANDNOT", 0xe2600000, 0xc2725002},
	{"OR", 0xe2800000, 0xc2925002}, {"ORNOT", 0xe2a00000, 0xc2b25002},
}

func encodeModified(profile Profile, instruction il.Instruction) ([]uint32, error) {
	if instruction.Modifier != "@" && instruction.Modifier != "%" {
		return nil, ErrUnqualifiedInstruction
	}
	name := strings.ToUpper(instruction.Mnemonic)
	input := false
	suffix := uint32(0)
	for _, definition := range edgeInputs {
		if definition.name == name {
			input = true
			suffix = definition.suffix
			break
		}
	}
	if !input && name != "SET" && name != "RSET" {
		return nil, ErrUnqualifiedInstruction
	}
	// TR is an execution-condition save/restore operand. Its differentiated
	// library outputs are retained as unqualified controls, not general contacts.
	if len(instruction.Operands) != 1 || strings.HasPrefix(strings.ToUpper(instruction.Operands[0]), "TR") {
		return nil, ErrUnqualifiedInstruction
	}
	plain := instruction
	plain.Modifier = ""
	words, err := EncodeInstruction(profile, plain)
	if err != nil {
		return nil, err
	}
	if len(words) != 1 {
		return nil, ErrUnqualifiedInstruction
	}
	if input {
		edge := uint32(0xcb800000)
		if instruction.Modifier == "%" {
			edge = 0xcba00000
		}
		return []uint32{0xe2c00000 | words[0]&0x1fffff, edge, suffix}, nil
	}
	prefix := uint32(0xfac00000)
	if instruction.Modifier == "%" {
		prefix = 0xfae00000
	}
	return []uint32{prefix, (words[0] | 0x04000000) & 0xdfffffff}, nil
}

func decodeModified(profile Profile, words []uint32) (il.Instruction, int, error) {
	fail := func(err error) (il.Instruction, int, error) { return il.Instruction{}, 0, err }
	modifier, consumed := "@", 2
	var plainWord uint32
	if words[0]&0xffe00000 == 0xe2c00000 {
		consumed = 3
		if len(words) < 3 || (words[1] != 0xcb800000 && words[1] != 0xcba00000) {
			return fail(ErrUnqualifiedInstruction)
		}
		if words[1] == 0xcba00000 {
			modifier = "%"
		}
		found := false
		for _, definition := range edgeInputs {
			if words[2] == definition.suffix {
				plainWord = definition.plain | words[0]&0x1fffff
				found = true
				break
			}
		}
		if !found {
			return fail(ErrUnqualifiedInstruction)
		}
	} else {
		if len(words) < 2 {
			return fail(ErrUnqualifiedInstruction)
		}
		if words[0] == 0xfae00000 {
			modifier = "%"
		} else if words[0] != 0xfac00000 {
			return fail(ErrUnqualifiedInstruction)
		}
		switch words[1] & 0xffe00000 {
		case 0xd4400000:
			plainWord = 0xf0400000 | words[1]&0x1fffff
		case 0xd4600000:
			plainWord = 0xf0600000 | words[1]&0x1fffff
		default:
			return fail(ErrUnqualifiedInstruction)
		}
	}
	plain, count, err := DecodeInstruction(profile, []uint32{plainWord})
	if err != nil {
		return fail(err)
	}
	if count != 1 || len(plain.Operands) != 1 || strings.HasPrefix(plain.Operands[0], "TR") {
		return fail(ErrUnqualifiedInstruction)
	}
	instruction, err := il.Parse(modifier + plain.Source)
	if err != nil {
		return fail(err)
	}
	return instruction, consumed, nil
}
