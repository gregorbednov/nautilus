package compiler_test

import (
	"encoding/binary"
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
	"github.com/joyautomation/nautilus/omron/il"
	"github.com/joyautomation/nautilus/omron/um"
)

func rung(path, text string) compiler.Rung {
	result := compiler.Rung{Path: path}
	for i, line := range strings.Split(text, "\n") {
		result.Lines = append(result.Lines, compiler.Line{Number: 100 + i, Text: line})
	}
	return result
}

func profile() codec.Profile {
	return codec.Profile{Model: codec.CJ1MCPU21, TimerCounterMode: codec.TimerModeBCD}
}

func TestCompilesStructuredRungsAndSourceMap(t *testing.T) {
	rungs := []compiler.Rung{
		rung("parallel", "LD W0.00\nANDNOT W0.01\nLD W0.02\nAND W0.03\nORLD\nOUT W1.00"),
		rung("branches", "LD 0.00\nOUT TR0\nAND 0.01\nOUT TR1\nAND 0.02\nOUT 5.00\nLD TR1\nAND 0.03\nOUT 5.01\nLD TR0\nAND 0.04\nOUT 5.02\nLD TR0\nANDNOT 5.00\nOUT 5.03"),
		rung("effects", "@LD W0.00\n%ANDNOT W0.01\n@SET W0.02\n%RSET W0.03\nMOV(021) #1234 D0\nTIMH(015) 0 #0010"),
		rung("counter", "LD W0.00\nLD W0.01\nCNT 0 #9999"),
		rung("end", "END(001)"),
	}
	body, err := compiler.CompileRungs(profile(), rungs)
	if err != nil {
		t.Fatal(err)
	}
	if !body.HasEnd || body.MaxLogicDepth != 2 || len(body.Warnings) != 0 {
		t.Fatalf("unexpected body metadata %+v", body)
	}
	spanIndex, offset := 0, 0
	for _, network := range rungs {
		for _, line := range network.Lines {
			instruction, err := il.Parse(line.Text)
			if err != nil {
				t.Fatal(err)
			}
			want, err := codec.EncodeInstruction(profile(), instruction)
			if err != nil {
				t.Fatal(err)
			}
			span := body.Spans[spanIndex]
			if span.Rung != network.Path || span.Line != line.Number || span.OffsetWords != offset || span.LengthWords != len(want) || !reflect.DeepEqual(body.Words[offset:offset+len(want)], want) {
				t.Fatalf("source span/order differs: %+v", span)
			}
			spanIndex++
			offset += len(want)
		}
	}
	if spanIndex != len(body.Spans) || offset != len(body.Words) {
		t.Fatal("source map has gaps")
	}
	data := body.Bytes()
	if len(data) != len(body.Words)*4 {
		t.Fatal("code serialization length")
	}
	for i, word := range body.Words {
		if binary.BigEndian.Uint32(data[4*i:]) != word {
			t.Fatal("incorrect code endian")
		}
	}
	data[0] ^= 255
	if binary.BigEndian.Uint32(body.Bytes()) != body.Words[0] {
		t.Fatal("serialization aliases body")
	}
}

func TestMnemonicCaseDoesNotChangeFlow(t *testing.T) {
	upper := []compiler.Rung{rung("open", "LD W0.00\nIL(002)"), rung("body", "LD W0.01\nLD W0.02\nORLD\nOUT W0.03"), rung("close", "ILC(003)\nEND")}
	lower := []compiler.Rung{rung("open", "ld W0.00\nil(002)"), rung("body", "ld W0.01\nld W0.02\norld\nout W0.03"), rung("close", "ilc(003)\nend")}
	want, err := compiler.CompileRungs(profile(), upper)
	if err != nil {
		t.Fatal(err)
	}
	got, err := compiler.CompileRungs(profile(), lower)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("mnemonic spelling changed body or source map")
	}
}

func TestStructuralErrorsAreAtomicAndLocated(t *testing.T) {
	for _, test := range []struct {
		source, code string
		line         int
	}{
		{"AND W0.00", "logic-underflow", 100},
		{"LD W0.00\nORLD", "logic-underflow", 101},
		{"OUT W0.00", "output-condition", 100},
		{"LD W0.00\nLD W0.01\nOUT W0.02", "output-condition", 102},
		{"LD W0.00\nLD W0.01", "logic-unjoined", 101},
		{"LD W0.00", "missing-output", 100},
		{"LD W0.00\nOUT W0.01\nNOT", "missing-output", 102},
		{"LD TR0\nOUT W0.00", "tr-undefined", 100},
		{"LD W0.00\nOUT TR0\nOUT TR0\nOUT W0.01", "tr-redefined", 102},
		{"LD W0.00\nOUT TR0\nLD W0.01\nLD TR0", "logic-unjoined", 103},
		{"OUT TR0", "tr-context", 100},
		{"LD W0.00\nOUT TR0", "missing-output", 101},
		{"LD W0.00\nIL", "il-unclosed", 101},
		{"ILC", "il-unmatched", 100},
		{"LD W0.00\nIL\nLD W0.01\nIL\nILC\nILC", "il-unmatched", 105},
		{"LD W0.00\nIL\nEND", "il-unclosed", 101},
		{"END\nNOP", "after-end", 101},
		{"LD W0.00\nCNT 0 #1", "output-condition", 101},
		{"LD W0.00\nLD W0.01\nCNT 0 #1\nOUT W0.02", "output-condition", 103},
		{"LD W0.00\nMOV(abc) #1 D0", "syntax", 101},
		{"LD W0.00\nOUT D0.00", "instruction", 101},
		{"LD W0.00\nMystery X", "instruction", 101},
	} {
		t.Run(test.code+"/"+test.source, func(t *testing.T) {
			body, err := compiler.CompileRungs(profile(), []compiler.Rung{rung("network", test.source)})
			var diagnostic *compiler.Error
			if body != nil || !errors.As(err, &diagnostic) || diagnostic.Code != test.code || diagnostic.Rung != "network" || diagnostic.Line != test.line {
				t.Fatalf("got body=%v err=%v; want %s at %d", body, err, test.code, test.line)
			}
		})
	}
}

func TestLogicDepthAndTRLifetimes(t *testing.T) {
	for _, depth := range []int{8, 9} {
		var lines []string
		for i := 0; i < depth; i++ {
			lines = append(lines, fmt.Sprintf("LD W%d.00", i))
		}
		for i := 1; i < depth; i++ {
			lines = append(lines, "ANDLD")
		}
		lines = append(lines, "OUT W100.00")
		body, err := compiler.CompileRungs(profile(), []compiler.Rung{rung("depth", strings.Join(lines, "\n"))})
		if depth == 8 {
			if err != nil || body.MaxLogicDepth != 8 {
				t.Fatal(body, err)
			}
		} else {
			var diagnostic *compiler.Error
			if body != nil || !errors.As(err, &diagnostic) || diagnostic.Code != "logic-overflow" || diagnostic.Line != 108 {
				t.Fatal(body, err)
			}
		}
	}
	// A new ordinary LD after an output begins a fresh block; AND/OR after
	// an output refine its existing condition. Saved TRs cannot leak between blocks.
	valid := "LD W0.00\nOUT TR0\nOUT W0.01\nLD TR0\nAND W0.02\nOUT W0.03\nAND W0.04\nLD W0.05\nORLD\nOUT W0.06\nLD W0.07\nOUT TR0\nOUT W0.08"
	if _, err := compiler.CompileRungs(profile(), []compiler.Rung{rung("reuse", valid)}); err != nil {
		t.Fatal(err)
	}
	for _, networks := range [][]compiler.Rung{
		{rung("a", "LD W0.00\nOUT TR0\nOUT W0.01"), rung("b", "LD TR0\nOUT W0.02")},
		{rung("a", "LD W0.00\nOUT TR0\nOUT W0.01\nLD W0.02\nOUT W0.03\nLD TR0\nOUT W0.04")},
	} {
		body, err := compiler.CompileRungs(profile(), networks)
		var diagnostic *compiler.Error
		if body != nil || !errors.As(err, &diagnostic) || diagnostic.Code != "tr-undefined" {
			t.Fatal(body, err)
		}
	}
}

func TestInterlocksAcrossRungsAndSharedClose(t *testing.T) {
	for _, opens := range []int{1, 2} {
		rungs := []compiler.Rung{}
		for i := 0; i < opens; i++ {
			rungs = append(rungs, rung(fmt.Sprintf("open%d", i), "LD W0.00\nIL(002)"))
		}
		rungs = append(rungs, rung("body", "LD W0.01\nOUT W0.02"), rung("close", "ILC(003)"), rung("end", "END"))
		body, err := compiler.CompileRungs(profile(), rungs)
		if err != nil || len(body.Warnings) != opens-1 {
			t.Fatal(body, err)
		}
		if opens == 2 && (body.Warnings[0].Code != "shared-ilc" || body.Warnings[0].Rung != "close") {
			t.Fatal(body.Warnings)
		}
	}
}

func TestCompilerLimitsAndProfiles(t *testing.T) {
	if body, err := compiler.CompileRungs(codec.Profile{}, nil); body != nil || !errors.Is(err, codec.ErrUnqualifiedProfile) {
		t.Fatal(body, err)
	}
	for _, rungs := range [][]compiler.Rung{nil, {rung("empty", " \n\t")}, {{Path: "bad", Lines: []compiler.Line{{Number: -1, Text: "NOP"}}}}} {
		if body, err := compiler.CompileRungs(profile(), rungs); body != nil || !errors.Is(err, compiler.ErrStructure) {
			t.Fatal(body, err)
		}
	}
	for _, rungs := range [][]compiler.Rung{
		make([]compiler.Rung, compiler.MaxInstructions+1),
		{{Lines: make([]compiler.Line, compiler.MaxInstructions+1)}},
		{rung("large", strings.Repeat(" ", compiler.MaxSourceBytes+1))},
	} {
		if body, err := compiler.CompileRungs(profile(), rungs); body != nil || !errors.Is(err, compiler.ErrSourceLimit) {
			t.Fatal("source limit was not enforced", err)
		}
	}
	if body, err := compiler.CompileRungs(profile(), []compiler.Rung{rung("fragment", "LD W0.00\nOUT W0.01")}); err != nil || body.HasEnd || len(body.Words) != 2 {
		t.Fatal("inserted implicit END", body, err)
	}
}

func TestCXPAdapterAndCompiledProjectPrefix(t *testing.T) {
	for _, test := range []struct{ project, image, source, model string }{
		{"cj1h.cxp", "cj1h-baseline.n1", "LD W0.00\nOUT W0.01", codec.CJ1HCPU65},
		{"cj1m.cxp", "cj1m-input-bit.n1", "LD W0.15\nOUT W0.01", codec.CJ1MCPU21},
	} {
		selectedProfile := profile()
		selectedProfile.Model = test.model
		data, err := os.ReadFile(filepath.Join("..", "cxp", "testdata", test.project))
		if err != nil {
			t.Fatal(err)
		}
		doc, err := cxp.Read(data)
		if err != nil {
			t.Fatal(err)
		}
		path := "Resource[0].Programs.Program[0].Sections.Sec[0].ProgramData.R[0]"
		if err := doc.SetRung(path, test.source); err != nil {
			t.Fatal(err)
		}
		var selected []cxp.Rung
		var graphic cxp.Rung
		for _, network := range doc.Summary().Rungs {
			if network.Path == path {
				selected = append(selected, network)
			}
			if len(network.Graphic) > 0 {
				graphic = network
			}
		}
		body, err := compiler.CompileCXPRungs(selectedProfile, selected)
		if err != nil {
			t.Fatal(err)
		}
		imageBytes, err := os.ReadFile(filepath.Join("..", "um", "testdata", test.image))
		if err != nil {
			t.Fatal(err)
		}
		image, err := um.ParseN1(imageBytes)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(body.Words, image.Tasks[0].Code[:2]) || body.HasEnd || body.Spans[0].Rung != path || body.Spans[0].Line != selected[0].Instructions[0].Line {
			t.Fatal("controlled CXP prefix/source map differs from captured compiler image")
		}
		if graphic.Path == "" {
			t.Fatal("no graphical control fixture")
		}
		if result, err := compiler.CompileCXPRungs(selectedProfile, append(selected, graphic)); result != nil || !errors.Is(err, compiler.ErrUnsupportedRung) {
			t.Fatal("silently compiled around graphical FB", result, err)
		}
		selected[0].Instructions[0].Text = "LD UnknownSymbol"
		if result, err := compiler.CompileCXPRungs(selectedProfile, selected); result != nil || err == nil {
			t.Fatal("trusted stale Parsed instead of current source")
		}
	}
}
