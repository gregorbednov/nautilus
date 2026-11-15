package codec_test

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/joyautomation/nautilus/omron/internal/fbcode"
)

func TestInstanceCallAgainstNativeOperands(t *testing.T) {
	accepted, rejected, otherSyntax := 0, 0, 0
	for _, row := range loadLibraryResults(t, "fbicall-manifest.json", 122) {
		if row.Mnemonic != "FBICALL" || len(row.Operands) != 2 {
			t.Fatal("wrong call fixture")
		}
		// The API emits decimal internal identifiers. Native '#' parsing is
		// different (e.g. #0400 => decimal400), so do not normalize that syntax.
		if row.Operands[0][0] != '&' || row.Operands[1][0] != '&' {
			otherSyntax++
			continue
		}
		instance, e1 := strconv.Atoi(row.Operands[0][1:])
		definition, e2 := strconv.Atoi(row.Operands[1][1:])
		if e1 != nil || e2 != nil {
			t.Fatal("invalid fixture numbers")
		}
		words, err := fbcode.InstanceCall(8, instance, definition)
		if row.Accepted {
			if err != nil || !reflect.DeepEqual(words, row.Words) {
				t.Fatal("native FBICALL differs", row.ID, err, words, row.Words)
			}
			accepted++
		} else {
			if err == nil || words != nil {
				t.Fatal("accepted native-rejected identifiers", row.ID)
			}
			rejected++
		}
	}
	if accepted != 98 || rejected != 16 || otherSyntax != 8 {
		t.Fatal("call evidence coverage changed", accepted, rejected, otherSyntax)
	}
}
