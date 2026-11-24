package um

import (
	"slices"

	"github.com/joyautomation/nautilus/omron/il"
	"github.com/joyautomation/nautilus/omron/internal/fbcode"
)

// CPU21 task 256 contains two calls with a CIO word input and a DM
// double-word result. Decode the operands, then rebuild the entire measured
// frame before exposing any instruction. Generated CII/CC bits depend on
// their invocation context.
func decodeWordCall(words []uint32, offset, imageStart uint32) ([]N1DecodedInstruction, bool) {
	const size = 20
	if len(words) < size {
		return nil, false
	}
	input := words[7] - 0x6000c000
	output := words[15] - 0x5c010000
	instance := int(words[10] & 0x7ff)
	body := int(words[11] & 0x7ff)
	parts, err := fbcode.WordResultCall(input, output, instance, body)
	if err != nil {
		return nil, false
	}
	result := make([]N1DecodedInstruction, 0, len(parts))
	pos := 0
	for _, p := range parts {
		end := pos + len(p.Words)
		if end > size || !slices.Equal(words[pos:end], p.Words) {
			return nil, false
		}
		ins, err := il.Parse(p.Text)
		if err != nil {
			return nil, false
		}
		at := offset + uint32(pos)
		result = append(result, N1DecodedInstruction{at, imageStart + at, p.Words, ins})
		pos = end
	}
	return result, pos == size
}
