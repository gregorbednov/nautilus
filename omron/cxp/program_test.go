package cxp

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

func programTestField(t *testing.T, doc *Document, path string) *Field {
	t.Helper()
	fields := doc.Fields
	var found *Field
	for _, part := range strings.Split(path, ".") {
		found = nil
		for i := range fields {
			if fields[i].Name == part {
				if found != nil {
					t.Fatal("duplicate field", path)
				}
				found = &fields[i]
			}
		}
		if found == nil {
			t.Fatal("missing field", path)
		}
		fields = found.Fields
	}
	return found
}

func programFixture(t *testing.T) *Document {
	t.Helper()
	data, err := os.ReadFile("testdata/cj1m.cxp")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Read(data)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestProgramSourceRetainsEmptyRungsAndGraphics(t *testing.T) {
	doc := programFixture(t)
	before, err := doc.MarshalCXT()
	if err != nil {
		t.Fatal(err)
	}
	for index, want := range []int{21, 28} {
		path := []string{"Resource[0].Programs.Program[0]", "Resource[0].Programs.Program[1]"}[index]
		source, err := doc.SourceForProgram(path)
		if err != nil {
			t.Fatal(err)
		}
		count, empty, graphic := 0, 0, 0
		for _, section := range source.Sections {
			for _, r := range section.Rungs {
				count++
				if len(r.Graphic) > 0 {
					graphic++
				}
				if len(r.Graphic) == 0 && len(r.Instructions) == 0 {
					empty++
				}
			}
		}
		if count != want || (index == 0 && (empty != 4 || graphic != 1)) || (index == 1 && (empty != 3 || graphic != 0)) {
			t.Fatal("source inventory changed", count, empty, graphic)
		}
	}
	after, err := doc.MarshalCXT()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("source traversal mutated document", err)
	}
}

func TestProgramSourceRejectsOmittedOrAmbiguousCode(t *testing.T) {
	const path = "Resource[0].Programs.Program[1]"
	for _, test := range []struct {
		name, target string
		mutate       func(*Field)
	}{
		{"section-count", path + ".Sections.SC", func(f *Field) { f.Value = "2" }},
		{"missing-rung", path + ".Sections.Sec[1].ProgramData", func(f *Field) { f.Fields = f.Fields[:len(f.Fields)-1] }},
		{"rung-order", path + ".Sections.Sec[1].ProgramData", func(f *Field) { f.Fields[1], f.Fields[2] = f.Fields[2], f.Fields[1] }},
		{"duplicate-sl", path + ".Sections.Sec[1].ProgramData.R[0]", func(f *Field) { f.Fields = append(f.Fields, Field{Name: "SL", Kind: "scalar", Value: `"END"`}) }},
		{"hidden-st", path + ".Sections.Sec[0]", func(f *Field) { f.Fields = append(f.Fields, Field{Name: "ST", Kind: "raw", Raw: "x:=1;"}) }},
		{"language", path + ".Sections.Sec[0].IEC1131Type", func(f *Field) { f.Value = "1" }},
		{"inline", path + ".CompileInline", func(f *Field) { f.Value = "1" }},
		{"flags", path + ".Sections.Sec[1].ProgramData.R[0].Flags", func(f *Field) { f.Value = `"0,0"` }},
		{"no-source", path + ".Sections.Sec[1].ProgramData.R[0].SL", func(f *Field) { f.Name = "Unknown" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			doc := programFixture(t)
			test.mutate(programTestField(t, doc, test.target))
			source, err := doc.SourceForProgram(path)
			if !errors.Is(err, ErrProgramSource) || source != nil {
				t.Fatal("accepted incomplete source", source, err)
			}
		})
	}
	var doc *Document
	if _, err := doc.SourceForProgram(path); !errors.Is(err, ErrProgramSource) {
		t.Fatal(err)
	}
}

func TestProgramSourceRetainsGraphicsAlongsideSL(t *testing.T) {
	doc := programFixture(t)
	// A summary treats SL as the preferred representation; compilation must
	// still see graphics if a source contains both representations.
	f := programTestField(t, doc, "Resource[0].Programs.Program[1].Sections.Sec[0].ProgramData.R[0]")
	f.Fields = append(f.Fields, Field{Name: "[99]", Kind: "object"})
	source, err := doc.SourceForProgram("Resource[0].Programs.Program[1]")
	if err != nil {
		t.Fatal(err)
	}
	if len(source.Sections[1].Rungs[0].Graphic) != 0 || len(source.Sections[0].Rungs[0].Graphic) != 1 {
		t.Fatal("mixed representation lost graphics")
	}
	for _, text := range []string{`""`, `"END"`} {
		programTestField(t, doc, "Resource[0].Programs.Program[1].Sections.Sec[0].ProgramData.R[0].SL").Value = text
		found := false
		for _, r := range doc.Summary().Rungs {
			if r.Path == source.Sections[0].Rungs[0].Path {
				found = true
				if len(r.Graphic) != 1 {
					t.Fatal("inspection dropped graphics")
				}
			}
		}
		if !found {
			t.Fatal("inspection omitted mixed source rung")
		}
	}
}
