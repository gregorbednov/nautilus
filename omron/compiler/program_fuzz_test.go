package compiler_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/cxp"
)

func FuzzCXPProgramBody(f *testing.F) {
	// Keep the mutation seed small enough for useful bounded fuzz throughput.
	// Both complete original programs are covered by ordinary fixture tests.
	text := []byte(`CXProgVer:="1.8";
Resource[0]:=
BEGIN
Programs:=
BEGIN
Program[1]:=
BEGIN
CompileInline:=0;
FileType:=2;
Sections:=
BEGIN
SC:=1;
Sec[0]:=
BEGIN
IEC1131Type:=11;
CompileInline:=0;
SecType:=0;
SecName:="Main";
ProgramData:=
BEGIN
RC:=1;
R[0]:=
BEGIN
Flags:="1,0";
SL:=
$?St$Bk?_#[1]
LD W0.00
UP(521)
OUT W1.00
END(001)
$?St$Bk?_#[1]
` + strings.Repeat("END;\n", 7))
	doc, err := cxp.ReadEncoding(text, "utf-8")
	if err != nil {
		f.Fatal(err)
	}
	if _, err := compiler.CompileCXPProgramBody(profile(), doc, "Resource[0].Programs.Program[1]"); err != nil {
		f.Fatal("invalid mutation seed", err)
	}
	f.Add(text)
	sectionStart := strings.Index(string(text), "Sec[0]:=")
	parents := strings.Repeat("END;\n", 4)
	section := strings.TrimSuffix(string(text)[sectionStart:], parents)
	first := strings.Replace(section, "END(001)\n", "", 1)
	second := strings.Replace(section, "Sec[0]", "Sec[1]", 1)
	second = strings.Replace(second, "LD W0.00\nUP(521)\nOUT W1.00\n", "", 1)
	twoSections := []byte(strings.Replace(string(text)[:sectionStart], "SC:=1;", "SC:=2;", 1) + first + second + parents)
	seed, err := cxp.ReadEncoding(twoSections, "utf-8")
	if err != nil {
		f.Fatal(err)
	}
	if body, err := compiler.CompileCXPSectionedProgramBody(profile(), seed, "Resource[0].Programs.Program[1]"); err != nil || len(body.Generated) != 1 {
		f.Fatal("invalid sectioned seed", err)
	}
	f.Add(twoSections)
	f.Add([]byte("CXProgVer:=\"1.8\";\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 128<<10 {
			return
		}
		doc, err := cxp.ReadEncoding(data, "utf-8")
		if err != nil {
			return
		}
		for _, compile := range []func(codec.Profile, *cxp.Document, string) (*compiler.Body, error){compiler.CompileCXPProgramBody, compiler.CompileCXPSectionedProgramBody} {
			body, err := compile(profile(), doc, "Resource[0].Programs.Program[1]")
			if err != nil {
				if body != nil {
					t.Fatal("partial program body")
				}
				continue
			}
			if !body.HasEnd || len(body.Spans) == 0 || len(body.Spans) > compiler.MaxInstructions {
				t.Fatal("incomplete program accepted")
			}
			covered := make([]bool, len(body.Words))
			mark := func(offset, length int) {
				if offset < 0 || length <= 0 || offset > len(covered) || length > len(covered)-offset {
					t.Fatal("invalid span bounds")
				}
				for i := offset; i < offset+length; i++ {
					if covered[i] {
						t.Fatal("overlapping source/generated spans")
					}
					covered[i] = true
				}
			}
			for _, span := range body.Generated {
				if span.Kind != "section-separator" || span.LengthWords != 3 {
					t.Fatal("unknown generated form")
				}
				mark(span.OffsetWords, span.LengthWords)
			}
			for _, span := range body.Spans {
				mark(span.OffsetWords, span.LengthWords)
				instruction, n, err := codec.DecodeInstruction(profile(), body.Words[span.OffsetWords:])
				if err != nil || n != span.LengthWords {
					t.Fatal("program instruction cannot decode", err)
				}
				again, err := codec.EncodeInstruction(profile(), instruction)
				if err != nil || !reflect.DeepEqual(again, body.Words[span.OffsetWords:span.OffsetWords+n]) {
					t.Fatal("program instruction changed")
				}
			}
			for _, yes := range covered {
				if !yes {
					t.Fatal("program source map has gaps")
				}
			}
		}
	})
}
