package fbcode

// InvocationSkeleton reproduces the two normally constructed native invocation
// lists. option is the measured GenerateInvocationCode boolean; true omits the
// PF0 save. Its wider caller-state meaning is not qualified here.
func InvocationSkeleton(selector uint32, option bool) (preamble, epilogue [][]uint32, err error) {
	if selector != 8 {
		return nil, nil, ErrPostamble
	}
	makeInstruction := func(words []uint32, end bool) []uint32 {
		result, _ := InvocationPostamble(selector, words, end)
		return result
	}
	// OR CF115; BMOV PF0 ,IR62+; BMOV ,-IR62 PF0; FBEND.
	preamble = [][]uint32{makeInstruction([]uint32{0xe2925008}, false)}
	if !option {
		preamble = append(preamble, makeInstruction([]uint32{0xb2410147, 0x63080000, 0x5717e009}, false))
	}
	epilogue = [][]uint32{makeInstruction([]uint32{0xb2410147, 0x6317e001, 0x57080000}, false), makeInstruction([]uint32{0xb000080e}, true)}
	return preamble, epilogue, nil
}

// InvocationHeader uses the generated instruction count, not container count.
// The bound is an implementation limit, not a qualified PLC resource capacity.
func InvocationHeader(selector uint32, instructions int) ([]uint32, error) {
	if selector != 8 || instructions < 1 || instructions > 128 {
		return nil, ErrPostamble
	}
	return []uint32{0xb040080d, 0x65440000 | uint32(instructions+1)}, nil
}

// AssembleInvocation surrounds already encoded/allocated body instructions with
// the qualified skeleton. It supports one FBICALL and measured transfer forms.
// It does NOT encode private operands, allocate ports or lower CXP/LD graphs.
// No source compiler calls this operation until those stages are qualified.
func AssembleInvocation(selector uint32, body [][]uint32, option bool) ([]uint32, error) {
	pre, post, err := InvocationSkeleton(selector, option)
	if err != nil {
		return nil, err
	}
	if len(body) > 128-len(pre)-len(post) {
		return nil, ErrPostamble
	}
	callCount := 0
	middle := make([][]uint32, 0, len(body))
	for _, words := range body {
		if len(words) == 0 {
			return nil, ErrPostamble
		}
		switch words[0] {
		case 0xb2410140, 0xb2410147:
			if len(words) != 3 {
				return nil, ErrPostamble
			}
		case 0xb2410806:
			if len(words) != 4 || words[1]&0xffff0000 != 0x61440000 || words[2]&0xffff0000 != 0x65440000 || words[3] != 0x80000000 {
				return nil, ErrPostamble
			}
			callCount++
		default:
			return nil, ErrPostamble
		}
		processed, err := InvocationPostamble(selector, words, false)
		if err != nil {
			return nil, err
		}
		middle = append(middle, processed)
	}
	if callCount != 1 {
		return nil, ErrPostamble
	}
	result, err := InvocationHeader(selector, len(pre)+len(middle)+len(post))
	if err != nil {
		return nil, err
	}
	for _, list := range [][][]uint32{pre, middle, post} {
		for _, words := range list {
			result = append(result, words...)
		}
	}
	return result, nil
}
