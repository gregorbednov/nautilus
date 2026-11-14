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
	f.Add([]byte("CXProgVer:=\"1.8\";\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 128<<10 {
			return
		}
		doc, err := cxp.ReadEncoding(data, "utf-8")
		if err != nil {
			return
		}
		body, err := compiler.CompileCXPProgramBody(profile(), doc, "Resource[0].Programs.Program[1]")
		if err != nil {
			if body != nil {
				t.Fatal("partial program body")
			}
			return
		}
		if !body.HasEnd || len(body.Spans) == 0 || len(body.Spans) > compiler.MaxInstructions {
			t.Fatal("incomplete program accepted")
		}
		next := 0
		for _, span := range body.Spans {
			if span.OffsetWords != next || span.LengthWords <= 0 || span.LengthWords > len(body.Words)-next {
				t.Fatal("invalid program source span")
			}
			instruction, n, err := codec.DecodeInstruction(profile(), body.Words[next:])
			if err != nil || n != span.LengthWords {
				t.Fatal("program instruction cannot decode", err)
			}
			again, err := codec.EncodeInstruction(profile(), instruction)
			if err != nil || !reflect.DeepEqual(again, body.Words[next:next+n]) {
				t.Fatal("program instruction changed")
			}
			next += n
		}
		if next != len(body.Words) {
			t.Fatal("program source map has gaps")
		}
	})
}
