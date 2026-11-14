package codec

import (
	"errors"
	"fmt"
	"strings"
)

var ErrTemporaryRelay = errors.New("invalid or unqualified temporary relay")

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
