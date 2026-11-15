package fbcode_test

import (
	"reflect"
	"testing"

	"github.com/joyautomation/nautilus/omron/internal/fbcode"
)

func TestInvocationPostambleBoundsAndOwnership(t *testing.T) {
	for _, spec := range []struct {
		selector uint32
		words    []uint32
		end      bool
	}{{8, nil, false}, {8, make([]uint32, 128), false}, {8, make([]uint32, 129), false}, {4, []uint32{0xb2410806}, false}, {8, []uint32{0xb000080d}, true}, {8, []uint32{0xb000080e, 0}, true}} {
		if words, err := fbcode.InvocationPostamble(spec.selector, spec.words, spec.end); err == nil || words != nil {
			t.Fatal("unqualified postamble produced words")
		}
	}
	for _, spec := range []struct {
		before, after []uint32
		end           bool
	}{
		{[]uint32{0xb2410806, 0x61440000, 0x65440400, 0x80000000}, []uint32{0x927f0806, 0x61440000, 0x65440400, 0x80000000}, false},
		{[]uint32{0xe2925008}, []uint32{0xc2925008}, false},
		{[]uint32{0xb000080e}, []uint32{0x9000080e}, true},
	} {
		before := append([]uint32{}, spec.before...)
		words, err := fbcode.InvocationPostamble(8, spec.before, spec.end)
		if err != nil || !reflect.DeepEqual(words, spec.after) || !reflect.DeepEqual(spec.before, before) {
			t.Fatal("postamble changed inputs or operands", err)
		}
		words[0] = 0
		if !reflect.DeepEqual(spec.before, before) {
			t.Fatal("postamble aliases input")
		}
	}
}

func TestInvocationAssemblyRejectsUnqualifiedBodies(t *testing.T) {
	call := []uint32{0xb2410806, 0x61440000, 0x65440400, 0x80000000}
	for _, body := range [][][]uint32{nil, {nil}, {call, call}, {{0xb2410806, 0x61440000, 0x65440400}}, {{0xb2410806, 0x61400000, 0x65440400, 0x80000000}}, {{0xb000080e}, call}, {{0xb2410140}, call}, make([][]uint32, 129)} {
		if words, err := fbcode.AssembleInvocation(8, body, false); err == nil || words != nil {
			t.Fatal("unqualified invocation body produced code")
		}
	}
	for _, n := range []int{-1, 0, 129} {
		if words, err := fbcode.InvocationHeader(8, n); err == nil || words != nil {
			t.Fatal("unbounded invocation header")
		}
	}
	if pre, post, err := fbcode.InvocationSkeleton(4, false); err == nil || pre != nil || post != nil {
		t.Fatal("unqualified selector generated skeleton")
	}
	before := append([]uint32{}, call...)
	words, err := fbcode.AssembleInvocation(8, [][]uint32{call}, false)
	if err != nil {
		t.Fatal(err)
	}
	words[0] = 0
	words[6] = 0
	if !reflect.DeepEqual(call, before) {
		t.Fatal("assembled frame aliases input")
	}
}

func FuzzInvocationPostamble(f *testing.F) {
	f.Add(uint32(0xb2410806), uint32(0x61440000), false)
	f.Add(uint32(0xb000080e), uint32(0), true)
	f.Fuzz(func(t *testing.T, first, operand uint32, end bool) {
		words := []uint32{first, operand}
		if end {
			words = words[:1]
		}
		before := append([]uint32{}, words...)
		result, err := fbcode.InvocationPostamble(8, words, end)
		if err != nil {
			if result != nil {
				t.Fatal("partial control transform")
			}
			return
		}
		if len(result) != len(words) || !reflect.DeepEqual(before, words) {
			t.Fatal("control transform changed source")
		}
		if len(result) > 1 && result[1] != operand {
			t.Fatal("control transform changed operand")
		}
		result[0] = 0
		if !reflect.DeepEqual(before, words) {
			t.Fatal("control transform aliases source")
		}
	})
}
