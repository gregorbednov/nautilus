package fbcode

import (
	"reflect"
	"testing"
)

func TestScalarTransferRejectsUnqualifiedForms(t *testing.T) {
	for _, test := range []struct {
		selector uint32
		typeName ScalarType
		usage    ParameterUsage
		source   string
	}{
		{4, Word, Input, "D0"}, {8, "DWORD", Input, "D0"},
		{8, Word, 4, "D0"}, {8, Word, Input, ""},
		{8, Word, Output, "#1"}, {8, Word, Input, "&65536"},
		{8, Word, Input, "W512"}, {8, Word, Output, "H512"},
		{8, Word, Input, "D32768"}, {8, Word, Input, "T0"},
		{8, Word, Input, "A448"}, {8, Word, Input, "IR62"},
		{8, Word, Input, "0,IR63"}, {8, Word, Input, "D0.01"},
		{8, Word, Input, "D0 D1"}, {8, Word, Input, "D0\nEND"},
		{8, Bool, Input, "#1"}, {8, Bool, Output, "W0"},
		{8, Bool, Output, "W511.16"}, {8, Bool, Input, "H512.00"},
		{8, Bool, Input, "D0.00"}, {8, Bool, Input, "A448.00"},
	} {
		words, err := ScalarTransfer(test.selector, test.typeName, test.usage, test.source)
		if err == nil || words != nil {
			t.Fatal("accepted unqualified scalar transfer", test, words)
		}
	}
}

func FuzzScalarTransfer(f *testing.F) {
	f.Add(uint8(8), "WORD", uint8(1), "#0100")
	f.Add(uint8(8), "BOOL", uint8(2), "W1.01")
	f.Add(uint8(8), "INT", uint8(1), "D32768")
	f.Fuzz(func(t *testing.T, selector uint8, kind string, usage uint8, source string) {
		words, err := ScalarTransfer(uint32(selector), ScalarType(kind), ParameterUsage(usage), source)
		if err != nil {
			if words != nil {
				t.Fatal("partial transfer on failure")
			}
			return
		}
		if len(words) != 3 || (words[0] != 0xb2410140 && words[0] != 0xb2410147) {
			t.Fatal("invalid transfer shape")
		}
		final, err := InvocationPostamble(8, words, false)
		if err != nil || final[0] != words[0]&0xdfffffff|0x023e0000 || !reflect.DeepEqual(final[1:], words[1:]) {
			t.Fatal("invalid transfer postamble")
		}
	})
}
