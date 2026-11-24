package fbcode

import (
	"fmt"
	"strconv"
)

type CallInstruction struct {
	Text  string
	Words []uint32
}

// WordResultCall builds the measured CPU21 CIO-word to DM-double-word FB frame.
func WordResultCall(input, output uint32, instance, body int) ([]CallInstruction, error) {
	if input > 6143 || output >= 32767 {
		return nil, ErrPostamble
	}
	inputText := strconv.FormatUint(uint64(input), 10)
	push, err := ScalarTransfer(8, Word, Input, inputText)
	if err != nil {
		return nil, ErrPostamble
	}
	call, err := InstanceCall(8, instance, body)
	if err != nil {
		return nil, ErrPostamble
	}
	pop := []uint32{0xb2410141, 0x6917f002, 0x5c010000 + output}
	pre, post, _ := InvocationSkeleton(8, false)
	header, _ := InvocationHeader(8, len(pre)+3+len(post))
	parts := []CallInstruction{{"FBBEGIN &8", header}, {"OR CF115", pre[0]}, {"BMOV(559) PF0 ,IR62+", pre[1]}}
	for _, p := range []CallInstruction{
		{"MOV(021) " + inputText + " ,IR63+", push},
		{fmt.Sprintf("FBICALL &%d &%d", instance, body), call},
		{fmt.Sprintf("MOVL(498) ,--IR63 D%d", output), pop},
	} {
		p.Words, err = InvocationPostamble(8, p.Words, false)
		if err != nil {
			return nil, ErrPostamble
		}
		parts = append(parts, p)
	}
	parts = append(parts, CallInstruction{"BMOV(559) ,-IR62 PF0", post[0]}, CallInstruction{"FBEND", post[1]})
	return parts, nil
}
