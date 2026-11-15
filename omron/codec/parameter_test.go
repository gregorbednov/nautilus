package codec_test

import (
	"github.com/joyautomation/nautilus/omron/cxp"
	"reflect"
	"testing"

	"github.com/joyautomation/nautilus/omron/internal/fbcode"
)

func TestScalarTransfersAgainstNativeGeneration(t *testing.T) {
	for _, row := range loadLibraryProbeResults(t, "parameter-manifest.json", 176, "parameter-transfer-v1") {
		p := row.Parameter
		if !row.Accepted || p == nil || len(p.Instructions) != 1 || p.SourceAfter != p.Input.Source {
			t.Fatal("incomplete scalar generation", row.ID)
		}
		if p.DataType != map[string]uint32{"BOOL": 1, "INT": 2, "UINT": 4, "WORD": 8}[p.Input.Type] {
			t.Fatal("native type mapping changed", row.ID)
		}
		words, err := fbcode.ScalarTransfer(8, fbcode.ScalarType(p.Input.Type), fbcode.ParameterUsage(p.Input.Usage), p.Input.Source)
		if err != nil {
			t.Fatal(row.ID, err)
		}
		pin := cxp.FBCallPin{Declaration: cxp.FBVariable{Name: "P", Usage: p.Input.Usage, TypeCode: map[string]int{"BOOL": 0, "INT": 1, "UINT": 2, "WORD": 7}[p.Input.Type], RemainingColumns: [4]string{"", "0", "0", "0"}}, Value: cxp.FBCallOperand{Source: p.Input.Source, Operand: p.Input.Source}}
		fromPin, e := fbcode.ScalarTransferForPin(8, pin)
		if e != nil || !reflect.DeepEqual(fromPin, words) {
			t.Fatal("PH_TYPE pin lowering differs from native scalar form", row.ID, e)
		}
		final, err := fbcode.InvocationPostamble(8, words, false)
		wantMnemonic := "MOV"
		if p.Input.Type == "BOOL" {
			wantMnemonic = "BMOV"
		}
		got := p.Instructions[0]
		if err != nil || !reflect.DeepEqual(final, got.Words) || got.Mnemonic != wantMnemonic || got.Flags&0x40000080 != 0x40000080 {
			t.Fatal("scalar transfer differs from normal native generation", row.ID, final, got, err)
		}
	}
}
