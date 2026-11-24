package main

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/lang/ld"
	"github.com/joyautomation/nautilus/omron/cxp"
)

func TestOmronLadderNew4(t *testing.T) {
	data, err := os.ReadFile("../../omron/commission/testdata/cj1m-new4/source.cxp")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := cxp.Read(data)
	if err != nil {
		t.Fatal(err)
	}
	model, err := omronLadderModel(doc, "Resource[0]")
	if err != nil {
		t.Fatal(err)
	}
	var sections []string
	for _, comment := range model.Comments {
		sections = append(sections, comment.Text)
	}
	want := []string{
		"TK0 · MotorDrive_LIFT_1 · POWER", "TK0 · MotorDrive_LIFT_1 · UP",
		"TK0 · MotorDrive_LIFT_1 · DOWN", "TK0 · MotorDrive_LIFT_1 · END",
		"TK1 · NewProgram1 · HM", "TK1 · NewProgram1 · Doors", "TK1 · NewProgram1 · END",
	}
	if !slices.Equal(sections, want) || len(model.Rungs) != 51 {
		t.Fatalf("got %d rows and sections %q", len(model.Rungs), sections)
	}
	rung := func(name string) ld.Rung {
		t.Helper()
		for _, row := range model.Rungs {
			if row.Name == name && row.Line > 0 {
				return row
			}
		}
		t.Fatalf("missing %s", name)
		return ld.Rung{}
	}
	power := rung("TK0 · POWER · R2")
	if len(power.Elements) != 4 || !power.Elements[0].Neg ||
		power.Elements[1].Kind != "branch" || len(power.Elements[1].Legs) != 2 ||
		power.Elements[1].Legs[0][0].Ref != "L1_DOWN" || power.Elements[1].Legs[1][0].Ref != "L1_UP" ||
		power.Elements[3].Text != "UP(521)" || len(power.Coils) != 1 || power.Coils[0].Mode != "S" {
		t.Fatalf("unexpected power rung: %+v", power)
	}
	fb := rung("TK0 · POWER · R4")
	if len(fb.Elements) != 2 || fb.Elements[0].Ref != "ON" || fb.Elements[1].Inst != "BLINK_L1" ||
		fb.Elements[1].Args != "PV := #100, PN := #100, IN := L1_RBR, OUT => W1.01" {
		t.Fatalf("unexpected FB call: %+v", fb)
	}
	saved, restored := rung("TK0 · UP · R3.1"), rung("TK0 · UP · R3.2")
	if saved.EndLine >= restored.Line || len(saved.Elements) != 6 ||
		saved.Elements[1].Ref != "TR0" || saved.Elements[1].Mode != "TR" ||
		saved.Elements[3].Ref != "TR1" || saved.Elements[3].Mode != "TR" ||
		len(restored.Elements) != 1 || len(restored.Elements[0].Legs) != 2 ||
		restored.Elements[0].Legs[0][0].Ref != "TR1" || restored.Elements[0].Legs[1][0].Ref != "TR0" {
		t.Fatalf("unexpected temporary relay sequence: %+v, %+v", saved, restored)
	}
	if empty := rung("TK1 · HM · R0"); len(empty.Elements) != 0 || len(empty.Coils) != 0 {
		t.Fatalf("expected an empty rung: %+v", empty)
	}

	modified, err := cxp.Read([]byte(strings.Replace(string(doc.Source), "UP(521)", "UNKNOWN(999)", 1)))
	if err != nil {
		t.Fatal(err)
	}
	model, err = omronLadderModel(modified, "Resource[0]")
	if err != nil {
		t.Fatal(err)
	}
	if len(model.Rungs) != 50 || len(model.Comments) != 8 ||
		!strings.Contains(model.Comments[1].Text, "diagram unavailable") ||
		!strings.Contains(model.Comments[1].Text, "SET ON") {
		t.Fatalf("unsupported rung should show its source in place: %+v", model.Comments)
	}
}
