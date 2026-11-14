package compiler_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/um"
)

const symbolProgram = "Resource[0].Programs.Program[0]"

func symbolScope(name, raw string, count int) cxp.Field {
	return cxp.Field{Name: name, Kind: "object", Fields: []cxp.Field{{Name: "VariableList", Kind: "list", Raw: raw, Line: 10}, {Name: "VariableCount", Kind: "scalar", Value: fmt.Sprint(count), Line: 50}}}
}
func bindingDocument(global, local string) *cxp.Document {
	count := func(raw string) int {
		if raw == "" {
			return 0
		}
		return len(strings.Split(raw, "\n"))
	}
	return &cxp.Document{Fields: []cxp.Field{{Name: "Resource[0]", Kind: "object", Fields: []cxp.Field{
		symbolScope("GlobalVariables", global, count(global)),
		{Name: "Programs", Kind: "object", Fields: []cxp.Field{
			{Name: "Program[0]", Kind: "object", Fields: []cxp.Field{symbolScope("LocalVariables", local, count(local))}},
			{Name: "Program[1]", Kind: "object", Fields: []cxp.Field{symbolScope("LocalVariables", "Private,W0.01,BOOL,,,;", 1)}},
		}},
	}}}}
}
func symbolicRung(text string) []cxp.Rung {
	network := cxp.Rung{Path: symbolProgram + ".Sections.Sec[0].ProgramData.R[0]"}
	for index, text := range strings.Split(text, "\n") {
		network.Instructions = append(network.Instructions, cxp.Instruction{Line: 100 + index, Text: text})
	}
	return []cxp.Rung{network}
}

func TestTypedBindingsAndLocalPrecedence(t *testing.T) {
	doc := bindingDocument("Start,W1.00,BOOL,,,;\nSource,D10,CHANNEL,,,;\nDestination,D11,WORD,,,;\nAlias,W0.00,BOOL,,,;", "Start,W0.00,BOOL,,,;")
	networks := symbolicRung("ld Start\nMOV Source Destination\n@SET Alias")
	body, err := compiler.CompileCXPProgramRungs(profile(), doc, symbolProgram, networks)
	if err != nil {
		t.Fatal(err)
	}
	want, err := compiler.CompileRungs(profile(), []compiler.Rung{rung(networks[0].Path, "LD W0.00\nMOV D10 D11\n@SET W0.00")})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(body.Words, want.Words) || !reflect.DeepEqual(body.Spans, want.Spans) || len(body.Bindings) != 4 {
		t.Fatal("binding changed code/source map", body)
	}
	if body.Bindings[0].Address != "W0.00" || body.Bindings[0].Scope != symbolProgram || body.Bindings[0].DeclarationLine != 12 || body.Bindings[1].Scope != "Resource[0]" || body.Bindings[1].Operand != 0 || body.Bindings[2].Operand != 1 {
		t.Fatal("missing explicit binding provenance", body.Bindings)
	}
	if networks[0].Instructions[0].Text != "ld Start" {
		t.Fatal("resolution rewrote caller source")
	}
	quoted := bindingDocument("\"Пуск, \"\"А\"\"\",W0.00,BOOL,,,;", "")
	if _, err := compiler.CompileCXPProgramRungs(profile(), quoted, symbolProgram, symbolicRung("LD \"Пуск, \"\"А\"\"\"\nOUT W0.01")); err != nil {
		t.Fatal("quoted exact-name binding failed", err)
	}
	prefixed := bindingDocument("D0_buffer,D0,WORD,,,;", "")
	if body, err := compiler.CompileCXPProgramRungs(profile(), prefixed, symbolProgram, symbolicRung("LD W0.00\nMOV D0_buffer D1")); err != nil || len(body.Bindings) != 1 || body.Bindings[0].Name != "D0_buffer" {
		t.Fatal("identifier prefix was mistaken for a native address", body, err)
	}
}

func TestSymbolFailuresCannotFallBackOrCoerce(t *testing.T) {
	for _, test := range []struct {
		global, local, source string
		cause                 error
	}{
		{"Alias,W0.00,BOOL,,,;", "", "LD Missing\nOUT W0.01", compiler.ErrUnknownSymbol},
		{"Alias,W0.00,BOOL,,,;", "", "LD Private\nOUT W0.01", compiler.ErrUnknownSymbol},
		{"Alias,W0.00,BOOL,,,;", "", "LD alias\nOUT W0.01", compiler.ErrUnknownSymbol},
		{"Alias,W0.00,BOOL,,,;", "Alias,\" [Auto]\",BOOL,,,;", "LD Alias\nOUT W0.01", compiler.ErrUnqualifiedSymbol},
		{"Alias,D0,WORD,,,;", "", "LD Alias\nOUT W0.01", compiler.ErrSymbolType},
		{"Alias,D0,WORD,,,;", "Alias,W0.00,BOOL,,,;", "LD W0.01\nMOV Alias D1", compiler.ErrSymbolType},
		{"Alias,W0.00,BOOL,,,;", "", "LD W0.01\nMOV Alias D0", compiler.ErrSymbolType},
		{"Alias,#10,NUMBER,,,;", "", "LD W0.00\nTIM 0 Alias", compiler.ErrSymbolType},
		{"Alias,D0,WORD,,,;", "", "LD W0.00\nTIM Alias #0010", compiler.ErrSymbolType},
		{"Alias,D0,DWORD,,,;", "", "LD W0.00\nMOV Alias D1", compiler.ErrSymbolType},
		{"Alias,Other,BOOL,,,;\nOther,W0.00,BOOL,,,;", "", "LD Alias\nOUT W0.01", compiler.ErrUnqualifiedSymbol},
		{"Alias,W0.00,BOOL,,A,;", "", "LD Alias\nOUT W0.01", compiler.ErrUnqualifiedSymbol},
		{"Alias,\" [Auto]\",FUNCTION BLOCK,BLINK,A,;", "", "LD Alias\nOUT W0.01", compiler.ErrUnqualifiedSymbol},
		{"Alias,A0.00,BOOL,,,;", "", "LD W0.00\nSET Alias", codec.ErrBitAddress},
		{"Alias,D0.00,BOOL,,,;", "", "LD Alias\nOUT W0.00", codec.ErrUnqualifiedInstruction},
		{"W0.00,W1.00,BOOL,,,;", "", "LD W0.00\nOUT W0.01", compiler.ErrUnqualifiedSymbol},
		{"Alias,W0.00,BOOL,,,;", "alias,W1.00,BOOL,,,;", "LD Alias\nOUT W0.01", compiler.ErrUnqualifiedSymbol},
	} {
		body, err := compiler.CompileCXPProgramRungs(profile(), bindingDocument(test.global, test.local), symbolProgram, symbolicRung(test.source))
		if body != nil || !errors.Is(err, test.cause) {
			t.Fatalf("%q: body=%v err=%v, want %v", test.source, body, err, test.cause)
		}
	}
	doc := bindingDocument("Alias,W0.00,BOOL,,,;", "")
	networks := symbolicRung("LD Alias\nOUT W0.01")
	networks[0].Path = "Resource[0].Programs.Program[1].Sections.Sec[0].ProgramData.R[0]"
	if body, err := compiler.CompileCXPProgramRungs(profile(), doc, symbolProgram, networks); body != nil || !errors.Is(err, cxp.ErrSymbolTable) {
		t.Fatal("wrong-scope rung received bindings", err)
	}
}

func TestOriginalSymbolicRungsMatchCapturedImages(t *testing.T) {
	for _, test := range []struct{ project, image, model string }{{"cj1h.cxp", "cj1h-baseline.n1", codec.CJ1HCPU65}, {"cj1m.cxp", "cj1m-input-bit.n1", codec.CJ1MCPU21}} {
		data, err := os.ReadFile(filepath.Join("..", "cxp", "testdata", test.project))
		if err != nil {
			t.Fatal(err)
		}
		doc, err := cxp.Read(data)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(filepath.Join("..", "um", "testdata", test.image))
		if err != nil {
			t.Fatal(err)
		}
		image, err := um.ParseN1(raw)
		if err != nil {
			t.Fatal(err)
		}
		selectedProfile := profile()
		selectedProfile.Model = test.model
		found := 0
		for _, network := range doc.Summary().Rungs {
			otherProgram := "Resource[0].Programs.Program[1]"
			if network.Path != symbolProgram+".Sections.Sec[0].ProgramData.R[1]" && network.Path != symbolProgram+".Sections.Sec[0].ProgramData.R[2]" && network.Path != symbolProgram+".Sections.Sec[0].ProgramData.R[3]" && network.Path != symbolProgram+".Sections.Sec[0].ProgramData.R[5]" && network.Path != symbolProgram+".Sections.Sec[1].ProgramData.R[0]" && network.Path != symbolProgram+".Sections.Sec[1].ProgramData.R[3]" && network.Path != symbolProgram+".Sections.Sec[2].ProgramData.R[3]" && network.Path != otherProgram+".Sections.Sec[1].ProgramData.R[0]" && network.Path != otherProgram+".Sections.Sec[1].ProgramData.R[1]" {
				continue
			}
			owner, taskIndex := symbolProgram, 0
			if strings.HasPrefix(network.Path, otherProgram+".") {
				owner, taskIndex = otherProgram, 1
			}
			body, err := compiler.CompileCXPProgramRungs(selectedProfile, doc, owner, []cxp.Rung{network})
			if err != nil {
				t.Fatal(test.project, network.Path, err)
			}
			if len(body.Bindings) == 0 {
				t.Fatal("original symbolic rung did not resolve any bindings")
			}
			matches := 0
			for index := 0; index+len(body.Words) <= len(image.Tasks[taskIndex].Code); index++ {
				if reflect.DeepEqual(body.Words, image.Tasks[taskIndex].Code[index:index+len(body.Words)]) {
					matches++
				}
			}
			if matches != 1 {
				t.Fatalf("%s %s: complete symbolic rung %x occurs %d times in captured task", test.project, network.Path, body.Words, matches)
			}
			found++
		}
		if found != 9 {
			t.Fatal("missing original symbolic rung controls", found)
		}
	}
}

func FuzzScopedBindings(f *testing.F) {
	for _, source := range []string{"LD Start\nOUT Stop", "LD Start\nMOV Source Destination", "@LD Start\n%SET Stop", "LD Start\nTIM 0 Source", "LD Private\nOUT Stop", "LD Start\nLD TR0\nOUT Stop"} {
		f.Add(source)
	}
	doc := bindingDocument("Start,W1.00,BOOL,,,;\nStop,W0.01,BOOL,,,;\nSource,D0,CHANNEL,,,;\nDestination,D1,WORD,,,;", "Start,W0.00,BOOL,,,;")
	want := map[string]struct{ address, scope string }{"Start": {"W0.00", symbolProgram}, "Stop": {"W0.01", "Resource[0]"}, "Source": {"D0", "Resource[0]"}, "Destination": {"D1", "Resource[0]"}}
	f.Fuzz(func(t *testing.T, source string) {
		if len(source) > 4096 {
			return
		}
		body, err := compiler.CompileCXPProgramRungs(profile(), doc, symbolProgram, symbolicRung(source))
		if err != nil {
			if body != nil {
				t.Fatal("partial symbolic compilation")
			}
			return
		}
		for _, use := range body.Bindings {
			expected, found := want[use.Name]
			if !found || use.Address != expected.address || use.Scope != expected.scope || use.Rung != symbolicRung("")[0].Path || use.Line < 100 || use.Operand < 0 {
				t.Fatal("scope/address provenance changed", use)
			}
		}
		for _, span := range body.Spans {
			decoded, count, err := codec.DecodeInstruction(profile(), body.Words[span.OffsetWords:])
			if err != nil || count != span.LengthWords {
				t.Fatal("symbolic instruction did not decode", err)
			}
			words, err := codec.EncodeInstruction(profile(), decoded)
			if err != nil || !reflect.DeepEqual(words, body.Words[span.OffsetWords:span.OffsetWords+count]) {
				t.Fatal("symbolic containers changed on round trip")
			}
		}
	})
}
