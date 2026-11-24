package codec_test

import (
	"reflect"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/il"
)

// Program type 32 alone does not convert physical operands to FB-relative code.
// Preserve the failed method as evidence, without relaxing production H bounds.
func TestFBBodyPhysicalOperandsRemainUnqualified(t *testing.T) {
	accepted, rejected := 0, 0
	for _, row := range loadLibraryProbeResults(t, "fb-body-manifest.json", 24, "fb-body-operands-v1") {
		model := codec.CJ1MCPU21
		if row.Device == "CJ1H-H" {
			model = codec.CJ1HCPU65
		}
		words, err := codec.EncodeInstruction(codec.Profile{Model: model, TimerCounterMode: codec.TimerModeBCD}, il.Instruction{Mnemonic: row.Mnemonic, Operands: row.Operands})
		if row.Accepted {
			accepted++
			if err != nil || !reflect.DeepEqual(words, row.Words) {
				t.Fatal("ordinary physical instruction differs", row.ID, words, err)
			}
		} else {
			rejected++
			if err == nil || len(row.OperandResults) == 0 {
				t.Fatal("unsupported physical FB operand admitted", row.ID)
			}
			found := false
			for _, result := range row.OperandResults {
				found = found || result == 8955
			}
			if !found {
				t.Fatal("rejection reason changed", row.ID, row.OperandResults)
			}
		}
	}
	if accepted != 8 || rejected != 16 {
		t.Fatal("probe coverage changed", accepted, rejected)
	}
}
