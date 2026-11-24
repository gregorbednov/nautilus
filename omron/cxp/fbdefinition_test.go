package cxp

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestOriginalFBDefinitionSources(t *testing.T) {
	want := [][]string{
		{"LDNOT IN", "ANDNOT ON", "AND EN", "TIMH TIMER_1 PV"},
		{"LD TIMER_1", "TIMH TIMER_2 PN"},
		{"LD TIMER_1", "OUT OUT"}, {"LD TIMER_2", "OUT ON"},
		{"LD EN", "AND IN", "SET OUT"}, {"LDNOT EN", "RSET OUT"},
	}
	for _, name := range []string{"cj1m.cxp", "cj1h.cxp"} {
		doc := originalFBSource(t, name)
		before := bytes.Clone(doc.Source)
		source, err := doc.SourceForFBDefinition(fbDefinition)
		if err != nil {
			t.Fatal(name, err)
		}
		if source.Name != "BLINK" || source.TaskID != 1024 || source.Version != "22.02.2020 15:11:13" || len(source.Variables) != 7 || len(source.Rungs) != 6 {
			t.Fatal("definition metadata", source)
		}
		for i, rung := range source.Rungs {
			var got []string
			for j, instruction := range rung.Instructions {
				text := instruction.Mnemonic
				if instruction.Line < 1 || (j == len(rung.Instructions)-1 && instruction.Column != 6) {
					t.Fatal("missing source location", instruction)
				}
				for _, operand := range instruction.Operands {
					text += " " + operand.Name
					if operand.TypeName == "" || (operand.Implicit && operand.Name != "EN") || (!operand.Implicit && operand.DeclarationLine < 1) {
						t.Fatal("lost reference type/origin", operand)
					}
				}
				got = append(got, text)
			}
			if !reflect.DeepEqual(got, want[i]) {
				t.Fatalf("%s rung %d: %v", name, i, got)
			}
		}
		if !bytes.Equal(before, doc.Source) {
			t.Fatal("source modified")
		}
		source.Rungs[0].Instructions[0].Operands[0].Name = "changed"
		source.Variables[0].Name = "changed"
		again, err := doc.SourceForFBDefinition(fbDefinition)
		if err != nil || again.Rungs[0].Instructions[0].Operands[0].Name != "IN" || again.Variables[0].Name != "TIMER_1" {
			t.Fatal("aliased result", err)
		}
	}
}

func TestFBDefinitionCoordinatesAndRenaming(t *testing.T) {
	doc := originalFBSource(t, "cj1m.cxp")
	want, err := doc.SourceForFBDefinition(fbDefinition)
	if err != nil {
		t.Fatal(err)
	}
	rung := fbTestField(t, doc, fbDefinition+".POUBody.FBProgramData.FBR[0]")
	for i, j := 0, len(rung.Fields)-1; i < j; i, j = i+1, j-1 {
		rung.Fields[i], rung.Fields[j] = rung.Fields[j], rung.Fields[i]
	}
	got, err := doc.SourceForFBDefinition(fbDefinition)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("serialized order affected graph", err)
	}
	list := fbTestField(t, doc, fbDefinition+".POUHeader.PouVarList")
	list.Raw = strings.Replace(list.Raw, "1,PV,1,", "1,Period,1,", 1)
	cell := fbTestField(t, doc, fbDefinition+".POUBody.FBProgramData.FBR[0].[1]")
	cell.Value = strings.Replace(cell.Value, ",PV,", ",Period,", 1)
	got, err = doc.SourceForFBDefinition(fbDefinition)
	if err != nil || got.Rungs[0].Instructions[3].Operands[1].Name != "Period" {
		t.Fatal("definition name hardcoded", err)
	}
}

func TestFBDefinitionRejectsUnqualifiedGraphs(t *testing.T) {
	const data = fbDefinition + ".POUBody.FBProgramData"
	const rung = data + ".FBR[0]"
	for _, test := range []struct {
		name, path string
		edit       func(*Field)
	}{
		{"flags", rung + ".Flags", func(f *Field) { f.Value = `"0,0"` }},
		{"count", rung + ".TotEl", func(f *Field) { f.Value = "8" }},
		{"rung count", data + ".FBRC", func(f *Field) { f.Value = "5" }},
		{"branch", rung + ".[2]", func(f *Field) { f.Value = `"2,1,1,1,1,1,EN,0"` }},
		{"width", rung + ".[2]", func(f *Field) { f.Value = `"2,0,2,1,1,1,EN,0"` }},
		{"duplicate coordinate", rung + ".[2]", func(f *Field) { f.Value = `"1,0,1,1,1,1,EN,0"` }},
		{"contact gap", rung + ".[3]", func(f *Field) { f.Value = `"1,0,1,1,0,3"` }},
		{"unknown reference", rung + ".[2]", func(f *Field) { f.Value = `"2,0,1,1,1,1,MISSING,0"` }},
		{"word contact", rung + ".[2]", func(f *Field) { f.Value = `"2,0,1,1,1,1,PV,0"` }},
		{"timer type", rung + ".[1]", func(f *Field) { f.Value = strings.Replace(f.Value, "TIMER_1", "ON", 1) }},
		{"preset type", rung + ".[1]", func(f *Field) { f.Value = strings.Replace(f.Value, ",PV,", ",IN,", 1) }},
		{"opcode", rung + ".[1]", func(f *Field) { f.Value = strings.Replace(f.Value, "TIMH(015)", "UNKNOWN", 1) }},
		{"truncated", rung + ".[1]", func(f *Field) { f.Value = `"6,0,1,0,3,5"` }},
		{"extra code", fbDefinition, func(f *Field) { f.Fields = append(f.Fields, Field{Name: "ExtraCode", Kind: "object"}) }},
		{"duplicate code", fbDefinition, func(f *Field) {
			for _, child := range f.Fields {
				if child.Name == "POUBody" {
					f.Fields = append(f.Fields, child)
					break
				}
			}
		}},
		{"extra body", fbDefinition + ".POUBody", func(f *Field) { f.Fields = append(f.Fields, Field{Name: "SL", Kind: "scalar"}) }},
		{"metadata", fbDefinition + ".POUHeader.PouVarList", func(f *Field) { f.Raw = strings.Replace(f.Raw, "1,PV,1,,0,", "1,PV,1,D0,0,", 1) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			doc := originalFBSource(t, "cj1m.cxp")
			test.edit(fbTestField(t, doc, test.path))
			got, err := doc.SourceForFBDefinition(fbDefinition)
			if got != nil || !errors.Is(err, ErrFBDefinition) {
				t.Fatal("accepted unsupported definition", got, err)
			}
		})
	}
	var nilDoc *Document
	if _, err := nilDoc.SourceForFBDefinition(fbDefinition); !errors.Is(err, ErrFBDefinition) {
		t.Fatal(err)
	}
}

func FuzzFBDefinitionCell(f *testing.F) {
	for _, cell := range []string{"0,0,1,1,1,1,IN,8", "6,0,1,0,3,5,TIMH(015),0,TIMER_1,0,PV,0", "", "2,1,1,1,1,1,EN,0"} {
		f.Add(cell)
	}
	refs := map[string]FBReference{"IN": {Name: "IN", TypeCode: 0}, "TIMER_1": {Name: "TIMER_1", TypeCode: 13}, "PV": {Name: "PV", TypeCode: 1}}
	f.Fuzz(func(t *testing.T, cell string) {
		if len(cell) > 4096 {
			t.Skip()
		}
		rung := Field{Kind: "object", Fields: []Field{{Name: "Flags", Kind: "scalar", Value: `"1,0"`}, {Name: "TotEl", Kind: "scalar", Value: "7"}, {Name: "Com", Kind: "scalar", Value: `""`}}}
		for i := 0; i < 7; i++ {
			value := fmt.Sprintf("%d,0,1,1,0,3", i)
			if i == 0 {
				value = "0,0,1,1,1,1,IN,8"
			}
			if i == 6 {
				value = cell
			}
			rung.Fields = append(rung.Fields, Field{Name: fmt.Sprintf("[%d]", i), Kind: "scalar", Value: strconv.Quote(value)})
		}
		got, err := readFBSeriesRung(rung, "fuzz", refs, false)
		if err != nil && got != nil {
			t.Fatal("partial result")
		}
		if err == nil && (len(got.Instructions) != 2 || got.Instructions[1].Column != 6) {
			t.Fatal("invalid accepted graph")
		}
	})
}
