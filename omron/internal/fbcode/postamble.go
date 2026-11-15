// Package fbcode contains qualified, low-level N1 operations for generated FB
// code. These operations do not qualify instruction forms or construct an FB.
package fbcode

import (
	"errors"
	"slices"
)

var ErrPostamble = errors.New("unqualified invocation postamble input")

// InvocationPostamble reproduces the measured selector8 postamble operation on
// one already compiled instruction. It must only be called for a generated
// invocation postamble, never for every instruction or for an entire code body.
// It does not decode/qualify operands, choose postambles, or allocate variables.
// end identifies the separately verified FBEND form. CII/CC retain the vendor's
// names; this operation does not assign unverified execution semantics to them.
func InvocationPostamble(selector uint32, words []uint32, end bool) ([]uint32, error) {
	// The native compiled-size getter sign-extends its one-byte count. Avoid
	// the unqualified128..255 range where SetCIIFlag would skip its mutation.
	if selector != 8 || len(words) == 0 || len(words) > 127 {
		return nil, ErrPostamble
	}
	if end && (len(words) != 1 || words[0] != 0xb000080e) {
		return nil, ErrPostamble
	}
	result := slices.Clone(words)
	if end {
		result[0] = 0x9000080e
		return result, nil
	}
	result[0] &= 0xdfffffff
	if result[0]&0xc0000000 == 0x80000000 {
		result[0] |= 0x023e0000
	}
	return result, nil
}
