package fbcode

import (
	"encoding/binary"
	"errors"
	"fmt"
	"regexp"
	"slices"

	"github.com/joyautomation/nautilus/omron/codec"
)

var ErrDefinition = errors.New("unqualified FB definition record")
var definitionName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,63}$`)

// InOutBOOLInversion is the measured two-instruction rung, including its
// indirect-address setup and restoration. Only the one-port frame is known.
func InOutBOOLInversion() []uint32 {
	return []uint32{
		0x9b7f0141, 0x6906c000, 0x5d17f00a,
		0x93410141, 0x693c3001, 0x5d06c000,
		0xa3410fe1, 0x6736c000, 0xbb410ff0, 0x5736c000,
		0x9b7f0141, 0x6917f002, 0x5d06c000,
	}
}

// DefinitionEnvelope emits the observed FBBN/FBBRET wrappers. The body number
// is explicit; this does not choose a definition number or allocate identifiers.
func DefinitionEnvelope(selector uint32, bodyNumber int) (entry, exit []uint32, err error) {
	if selector != 8 || bodyNumber < 0 || bodyNumber > 2047 {
		return nil, nil, fmt.Errorf("%w: selector8 and body number0..2047 required", ErrDefinition)
	}
	return []uint32{0xb0400809, 0x65440000 | uint32(bodyNumber)}, []uint32{0xbb800805, 0x80000000}, nil
}

// AssembleDefinitionRecord emits one observed zero-flag 23-word N1 definition
// header and already wrapped short relative code. Counts include implicit
// EN/ENO. Numbering is explicit; no section header, instances or task linking.
// Names are rejected rather than silently truncating the native 64-byte field.
func AssembleDefinitionRecord(profile codec.Profile, number int, name string, inputs, outputs int, code []uint32) ([]uint32, error) {
	if number < 0 || number > 1023 {
		return nil, fmt.Errorf("%w: definition number must be0..1023", ErrDefinition)
	}
	if !definitionName.MatchString(name) {
		return nil, fmt.Errorf("%w: name must be an ASCII identifier of1..64 bytes", ErrDefinition)
	}
	if inputs < 1 || inputs > 65 || outputs < 1 || outputs > 65 {
		return nil, fmt.Errorf("%w: port counts including EN/ENO must be1..65", ErrDefinition)
	}
	if len(code) < 5 || len(code) > 65536 {
		return nil, fmt.Errorf("%w: wrapped code length outside implementation bounds", ErrDefinition)
	}
	if code[0] != 0xb0400809 || code[1]&0xfffff800 != 0x65440000 || code[len(code)-2] != 0xbb800805 || code[len(code)-1] != 0x80000000 {
		return nil, ErrDefinition
	}
	body := code[2 : len(code)-2]
	if !(profile.SupportsInOutBOOL() &&
		inputs == 2 && outputs == 2 && slices.Equal(body, InOutBOOLInversion())) {
		if _, err := codec.DecodeFBRelativeBody(profile, body); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrDefinition, err)
		}
	}
	header := make([]uint32, 23)
	header[0] = uint32(number)
	header[2] = uint32(len(code) + len(header))
	header[22] = uint32(inputs<<8 | outputs)
	var padded [64]byte
	for i := range padded {
		padded[i] = ' '
	}
	copy(padded[:], name)
	for i := 0; i < 16; i++ {
		header[3+i] = binary.BigEndian.Uint32(padded[i*4:])
	}
	return append(header, code...), nil
}
