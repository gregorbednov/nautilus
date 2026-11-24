package fbcode_test

import (
	"testing"

	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/internal/fbcode"
)

func TestSourcePinTypeAndBindingGuards(t *testing.T) {
	makePin := func() cxp.FBCallPin {
		return cxp.FBCallPin{
			Declaration: cxp.FBVariable{Name: "IN", Usage: 1, TypeCode: 0, RemainingColumns: [4]string{"", "0", "FALSE", "0"}},
			Value:       cxp.FBCallOperand{Source: "Signal", Operand: "W0.00", Symbol: &cxp.Symbol{Name: "Signal", Address: "W0.00", DataType: "BOOL"}},
		}
	}
	for _, test := range []struct {
		name string
		edit func(*cxp.FBCallPin)
	}{
		{"unknown type", func(p *cxp.FBCallPin) { p.Declaration.TypeCode = 99; p.Declaration.TypeName = "BOOL" }},
		{"unsupported type", func(p *cxp.FBCallPin) { p.Declaration.TypeCode = 13 }},
		{"unqualified usage", func(p *cxp.FBCallPin) { p.Declaration.Usage = 3 }},
		{"array metadata", func(p *cxp.FBCallPin) { p.Declaration.RemainingColumns[0] = "2" }},
		{"retained metadata", func(p *cxp.FBCallPin) { p.Declaration.RemainingColumns[3] = "1" }},
		{"symbol type mismatch", func(p *cxp.FBCallPin) { p.Value.Symbol.DataType = "WORD" }},
		{"symbol address mismatch", func(p *cxp.FBCallPin) { p.Value.Operand = "W1.00" }},
		{"symbol identity mismatch", func(p *cxp.FBCallPin) { p.Value.Source = "Other" }},
		{"symbol flags", func(p *cxp.FBCallPin) { p.Value.Symbol.Flags = "A" }},
		{"empty source", func(p *cxp.FBCallPin) { p.Value.Source = "" }},
		{"unresolved source", func(p *cxp.FBCallPin) { p.Value.Symbol = nil }},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := makePin()
			test.edit(&p)
			words, err := fbcode.ScalarTransferForPin(8, p)
			if err == nil || words != nil {
				t.Fatal("accepted unqualified pin", words)
			}
		})
	}
	p := makePin()
	p.Declaration.TypeName = "WORD" // Cached display identity is not compiler input.
	if words, err := fbcode.ScalarTransferForPin(8, p); err != nil || len(words) != 3 || words[0] != 0xb2410147 {
		t.Fatal("trusted cached type name", words, err)
	}
}
