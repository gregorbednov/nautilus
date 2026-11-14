package codec

import (
	"errors"
	"fmt"
	"strings"
)

var ErrTemporaryRelay = errors.New("invalid or unqualified temporary relay")

// These exact sequences implement zero-operand condition transformers. Their
// internal 0x120000 field is not a qualified address in the direct-bit codec.
var conditionInstructions = []struct {
	name  string
	words []uint32
}{
	{"UP", []uint32{0xe2d20000, 0xcb800000, 0xc2525002}},
	{"DOWN", []uint32{0xe2d20000, 0xcba00000, 0xc3000000, 0xc2525002}},
}

// TemporaryRelay retains an execution condition at a mnemonic branch point.
// It is an instruction operand, not an address in PLC memory or a word operand.
type TemporaryRelay uint8

func ParseTemporaryRelay(source string) (TemporaryRelay, error) {
	if len(source) > 64 || !strings.HasPrefix(strings.ToUpper(source), "TR") {
		return 0, ErrTemporaryRelay
	}
	index, err := decimal(source[2:], 4)
	if err != nil {
		return 0, ErrTemporaryRelay
	}
	return TemporaryRelay(index), nil
}

func (relay TemporaryRelay) String() string {
	if relay > 15 {
		return "<invalid-temporary-relay>"
	}
	return fmt.Sprintf("TR%d", relay)
}

var branchInstructions = []struct {
	name   string
	opcode uint32
}{
	{"NOT", 0xe3000000},
	{"ANDLD", 0xe3200000},
	{"ORLD", 0xe3400000},
	{"IL", 0xb1000002},
	{"ILC", 0xbb000003},
}
