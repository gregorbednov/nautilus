package cxp

import (
	"os"
	"testing"
)

func TestFixtureInstructionModel(t *testing.T) {
	for _, name := range []string{"cj1m.cxp", "cj1h.cxp"} {
		data, err := os.ReadFile("testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		d, err := Read(data)
		if err != nil {
			t.Fatal(err)
		}
		text, graphic := 0, 0
		for _, rung := range d.Summary().Rungs {
			graphic += len(rung.Graphic)
			for _, instruction := range rung.Instructions {
				text++
				if instruction.Parsed == nil || instruction.SyntaxError != nil || instruction.Parsed.Source != instruction.Text {
					t.Fatalf("%s %s: %+v", name, rung.Path, instruction)
				}
			}
		}
		if text == 0 || graphic == 0 {
			t.Fatalf("%s lost source: %d text, %d graphic", name, text, graphic)
		}
	}
}
func TestInspectionPreservesUnparsedInstruction(t *testing.T) {
	d := &Document{Fields: []Field{{Name: "R[0]", Fields: []Field{{Name: "SL", Kind: "raw", Raw: "MOV(abc) D0 D1\nLD W0.00", Line: 10}}}}}
	s := d.Summary()
	got := s.Rungs[0].Instructions
	if len(got) != 2 || got[0].Text != "MOV(abc) D0 D1" || got[0].SyntaxError == nil || got[0].Parsed != nil || got[0].Line != 12 || got[1].Parsed == nil || len(s.Warnings) != 2 {
		t.Fatalf("missing source or diagnostic: %+v", s)
	}
}
