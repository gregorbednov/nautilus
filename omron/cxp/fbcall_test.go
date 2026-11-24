package cxp

import (
	"bytes"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

const fbProgram = "Resource[0].Programs.Program[0]"
const fbRung = fbProgram + ".Sections.Sec[0].ProgramData.R[4]"
const fbDefinition = "Resource[0].FunctionBlocks.FunctionBlockWrapper[0].FunctionBlock"

func originalFBSource(t *testing.T, name string) *Document {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Read(data)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func fbTestField(t *testing.T, doc *Document, path string) *Field {
	t.Helper()
	fields := &doc.Fields
	var result *Field
	for _, part := range strings.Split(path, ".") {
		result = nil
		for i := range *fields {
			if (*fields)[i].Name == part {
				result = &(*fields)[i]
				break
			}
		}
		if result == nil {
			t.Fatal("missing test field", path)
		}
		fields = &result.Fields
	}
	return result
}

func TestOriginalFBCallPlans(t *testing.T) {
	for _, name := range []string{"cj1m.cxp", "cj1h.cxp"} {
		doc := originalFBSource(t, name)
		before := bytes.Clone(doc.Source)
		plan, err := doc.PlanFBCall(fbProgram, fbRung)
		if err != nil {
			t.Fatal(name, err)
		}
		if plan.Instance != "BLINK_L1" || plan.InstanceNumber != 0 || plan.Definition != "BLINK" || plan.DefinitionPath != fbDefinition || plan.TaskID != 1024 || plan.Version != "22.02.2020 15:11:13" || len(plan.Variables) != 7 {
			t.Fatal("definition/instance metadata", plan)
		}
		if plan.Enable.Source != "ON" || plan.Enable.Operand != "W1.00" || plan.Enable.Symbol == nil || plan.Enable.Symbol.Scope != "Resource[0]" {
			t.Fatal("enable", plan.Enable)
		}
		if len(plan.Inputs) != 3 || len(plan.Outputs) != 1 {
			t.Fatal("pins", plan)
		}
		for i, want := range []struct {
			name, source, address string
			typeCode              int
		}{
			{"PV", "#100", "#100", 1}, {"PN", "#100", "#100", 1}, {"IN", "L1_out_powerON", "4.14", 0},
		} {
			p := plan.Inputs[i]
			if p.Declaration.Name != want.name || p.Declaration.Usage != 1 || p.Declaration.TypeCode != want.typeCode || p.Value.Source != want.source || p.Value.Operand != want.address || p.Row != i+2 || p.Value.Line < 1 || p.Declaration.Line < 1 {
				t.Fatal("input ordering/binding", p)
			}
		}
		p := plan.Outputs[0]
		if p.Declaration.Name != "OUT" || p.Declaration.Usage != 2 || p.Declaration.TypeCode != 0 || p.Value.Source != "BLINK_1_OUT" || p.Value.Operand != "W1.01" || p.Value.Symbol == nil || p.Value.Symbol.Scope != fbProgram {
			t.Fatal("output", p)
		}
		if !bytes.Equal(doc.Source, before) {
			t.Fatal("source changed")
		}
		// A second call must not reuse mutable slices/pointers from this result.
		plan.Inputs[0].Declaration.Name = "changed"
		plan.Enable.Symbol.Address = "changed"
		again, err := doc.PlanFBCall(fbProgram, fbRung)
		if err != nil || again.Inputs[0].Declaration.Name != "PV" || again.Enable.Operand != "W1.00" {
			t.Fatal("aliased plan", err)
		}
	}
}

func TestFBPinOrderIndependentOfSerializedCells(t *testing.T) {
	doc := originalFBSource(t, "cj1m.cxp")
	want, err := doc.PlanFBCall(fbProgram, fbRung)
	if err != nil {
		t.Fatal(err)
	}
	f := fbTestField(t, doc, fbRung)
	for i, j := 0, len(f.Fields)-1; i < j; i, j = i+1, j-1 {
		f.Fields[i], f.Fields[j] = f.Fields[j], f.Fields[i]
	}
	got, err := doc.PlanFBCall(fbProgram, fbRung)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("serialized field order affected call", err)
	}
}

func TestFBCallPlanPreservesUnqualifiedTypeCodes(t *testing.T) {
	doc := originalFBSource(t, "cj1m.cxp")
	f := fbTestField(t, doc, fbDefinition+".POUHeader.PouVarList")
	f.Raw = strings.Replace(f.Raw, "1,PV,1,", "1,PV,99,", 1)
	plan, err := doc.PlanFBCall(fbProgram, fbRung)
	if err != nil || plan.Inputs[0].Declaration.TypeCode != 99 || plan.Inputs[0].Declaration.TypeName != "" {
		t.Fatal("invented type qualification", err)
	}
}

func TestFBCallPlanHonorsLocalBindingsAndSourceChanges(t *testing.T) {
	doc := originalFBSource(t, "cj1m.cxp")
	list := fbTestField(t, doc, fbProgram+".LocalVariables.VariableList")
	list.Raw += "\nON,W2.00,BOOL,,,;"
	fbTestField(t, doc, fbProgram+".LocalVariables.VariableCount").Value = "11"
	cell := fbTestField(t, doc, fbRung+".[5]")
	cell.Value = strings.Replace(cell.Value, "#100", "#200", 1)
	plan, err := doc.PlanFBCall(fbProgram, fbRung)
	if err != nil || plan.Enable.Operand != "W2.00" || plan.Enable.Symbol.Scope != fbProgram || plan.Inputs[0].Value.Operand != "#200" || plan.Inputs[1].Value.Operand != "#100" {
		t.Fatal("source change or scope ignored", err)
	}
}

func TestFBCallPlanRejectsAmbiguousOrUnsupportedSource(t *testing.T) {
	for _, test := range []struct {
		name, path string
		edit       func(*Field)
	}{
		{"stale call", fbRung + ".FBversion", func(f *Field) { f.Value = `"old"` }},
		{"wrong cell count", fbRung + ".TotEl", func(f *Field) { f.Value = "8" }},
		{"duplicate cell index", fbRung + ".[2]", func(f *Field) { f.Name = "[3]" }},
		{"wrong pin row", fbRung + ".[2]", func(f *Field) { f.Value = strings.Replace(f.Value, "1\x114\x11", "1\x113\x11", 1) }},
		{"unknown operation", fbRung + ".[0]", func(f *Field) { f.Value = strings.Replace(f.Value, "LD", "LDNOT", 1) }},
		{"truncated cell", fbRung + ".[1]", func(f *Field) { f.Value = `"2"` }},
		{"unknown operand", fbRung + ".[2]", func(f *Field) { f.Value = strings.Replace(f.Value, "L1_out_powerON", "Missing", 1) }},
		{"wrong variable count", fbDefinition + ".POUHeader.PouVarCount", func(f *Field) { f.Value = "8" }},
		{"duplicate declaration", fbDefinition + ".POUHeader.PouVarList", func(f *Field) { f.Raw = strings.Replace(f.Raw, ",PN,", ",PV,", 1) }},
		{"array parameter", fbDefinition + ".POUHeader.PouVarList", func(f *Field) { f.Raw = strings.Replace(f.Raw, "1,PV,1,,", "1,PV,1,2,", 1) }},
		{"inout parameter", fbDefinition + ".POUHeader.PouVarList", func(f *Field) { f.Raw = strings.Replace(f.Raw, "1,PV,1,", "4,PV,1,", 1) }},
		{"missing input declaration", fbDefinition + ".POUHeader.PouVarList", func(f *Field) { f.Raw = strings.Replace(f.Raw, "1,PV,1,", "0,PV,1,", 1) }},
		{"duplicate instance", "Resource[0].GlobalVariables.POUInstances.Instance[1].Name", func(f *Field) { f.Value = `"BLINK_L1"` }},
		{"duplicate instance number", "Resource[0].GlobalVariables.POUInstances.Instance[1].InstanceNum", func(f *Field) { f.Value = "0" }},
		{"instance type mismatch", "Resource[0].GlobalVariables.POUInstances.Instance[0].POUType", func(f *Field) { f.Value = `"Other"` }},
		{"case ambiguous name", fbProgram + ".LocalVariables.VariableList", func(f *Field) { f.Raw = strings.Replace(f.Raw, "BLINK_1_OUT,", "on,", 1) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			doc := originalFBSource(t, "cj1m.cxp")
			test.edit(fbTestField(t, doc, test.path))
			plan, err := doc.PlanFBCall(fbProgram, fbRung)
			if !errors.Is(err, ErrFBCall) || plan != nil {
				t.Fatal("accepted unsupported call", plan)
			}
		})
	}
	var doc *Document
	if plan, err := doc.PlanFBCall(fbProgram, fbRung); err == nil || plan != nil {
		t.Fatal("nil document")
	}
	doc = originalFBSource(t, "cj1m.cxp")
	for _, path := range []string{fbProgram, fbProgram + ".Sections.Sec[0].ProgramData.R[0]", strings.Replace(fbRung, "Program[0]", "Program[1]", 1)} {
		if plan, err := doc.PlanFBCall(fbProgram, path); err == nil || plan != nil {
			t.Fatal("accepted invalid scope", path)
		}
	}
}

func TestFBCallRejectsUnqualifiedLocalInstanceScope(t *testing.T) {
	doc := originalFBSource(t, "cj1m.cxp")
	list := fbTestField(t, doc, fbProgram+".LocalVariables.VariableList")
	list.Raw += "\nBLINK_L1,\" [Auto]\",FUNCTION BLOCK,BLINK,A,;"
	fbTestField(t, doc, fbProgram+".LocalVariables.VariableCount").Value = "11"
	if plan, err := doc.PlanFBCall(fbProgram, fbRung); !errors.Is(err, ErrFBCall) || plan != nil {
		t.Fatal("local instance reused global record", plan, err)
	}
}

func FuzzFBCallPlan(f *testing.F) {
	data, err := os.ReadFile("testdata/cj1m.cxp")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(uint8(0), `"1`+"\x114\x118\x111\x111\x11FB_INPUT\x11L1_out_powerON\x1110"+`"`)
	f.Add(uint8(1), "1,PV,99,,0,0,0,;")
	f.Add(uint8(2), `"old"`)
	f.Fuzz(func(t *testing.T, which uint8, value string) {
		if len(value) > 8192 {
			t.Skip()
		}
		doc, e := Read(data)
		if e != nil {
			t.Fatal(e)
		}
		paths := []string{fbRung + ".[2]", fbDefinition + ".POUHeader.PouVarList", fbRung + ".FBversion"}
		field := fbTestField(t, doc, paths[int(which)%len(paths)])
		if int(which)%len(paths) == 1 {
			field.Raw = value
		} else {
			field.Value = value
		}
		plan, e := doc.PlanFBCall(fbProgram, fbRung)
		if e != nil {
			if plan != nil || !errors.Is(e, ErrFBCall) {
				t.Fatal("partial plan/error contract")
			}
			return
		}
		if len(plan.Inputs) != 3 || len(plan.Outputs) != 1 || len(plan.Variables) != 7 || plan.Version != "22.02.2020 15:11:13" {
			t.Fatal("invalid successful plan")
		}
		for i, p := range plan.Inputs {
			if p.Row != i+2 || p.Declaration.Usage != 1 {
				t.Fatal("invalid input ordering")
			}
		}
	})
}
