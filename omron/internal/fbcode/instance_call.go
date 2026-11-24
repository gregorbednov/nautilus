package fbcode

const maxCallIdentifier = 2047

// InstanceCall emits the measured selector8 FBICALL &instance &definition form
// before invocation postamble processing. Both identifiers are limited to
// 0..2047 by the native operand validator. This is an encoding domain, not a
// qualified number of definitions/instances that a physical PLC can allocate.
// It does not prove either identifier has a corresponding generated record.
func InstanceCall(selector uint32, instance, definition int) ([]uint32, error) {
	if selector != 8 || instance < 0 || instance > maxCallIdentifier || definition < 0 || definition > maxCallIdentifier {
		return nil, ErrPostamble
	}
	return []uint32{0xb2410806, 0x61440000 | uint32(instance), 0x65440000 | uint32(definition), 0x80000000}, nil
}
