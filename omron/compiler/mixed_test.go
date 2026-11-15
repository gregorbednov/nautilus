package compiler_test

import (
	"crypto/sha256"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/um"
)

func checkMixedCoverage(t *testing.T, body *compiler.Body) {
	t.Helper()
	covered := make([]int, len(body.Words))
	mark := func(offset, length int) {
		if offset < 0 || length < 1 || offset > len(covered)-length {
			t.Fatal("invalid mixed range")
		}
		for i := offset; i < offset+length; i++ {
			covered[i]++
		}
	}
	for _, span := range body.Spans {
		if span.Line < 1 {
			t.Fatal("lost source line")
		}
		mark(span.OffsetWords, span.LengthWords)
	}
	for _, span := range body.Generated {
		mark(span.OffsetWords, span.LengthWords)
	}
	for i, n := range covered {
		if n != 1 {
			t.Fatal("mixed code missing/overlapping provenance", i, n)
		}
	}
}

func TestMixedProgramMatchesEntireCapturedFirstTaskCode(t *testing.T) {
	// These first-rung recipes and CXT hashes come from the portable oracle
	// manifest. The original fixture's UP/SET rung is a different program.
	for _, spec := range []struct{ source, image, model, first, cxtHash string }{
		{"cj1m.cxp", "cj1m-input-bit.n1", codec.CJ1MCPU21, "LD W0.15\nOUT W0.01", "c1881eb043d9acc409a635410e91db03cc3c109c447e3addd65d632bf1d4234e"},
		{"cj1h.cxp", "cj1h-baseline.n1", codec.CJ1HCPU65, "LD W0.00\nOUT W0.01", "1df37113657da21d0a9b3f151c64cf9063b222f5b38f32b93b7ea860023b93b6"},
	} {
		doc := callDocument(t, spec.source)
		p := codec.Profile{Model: spec.model, TimerCounterMode: codec.TimerModeBCD}
		original, err := compiler.CompileCXPMixedProgramBody(p, doc, symbolProgram, true)
		if err != nil {
			t.Fatal(err)
		}
		if err := doc.SetRung(symbolProgram+".Sections.Sec[0].ProgramData.R[0]", spec.first); err != nil {
			t.Fatal(err)
		}
		cxt, err := doc.MarshalCXT()
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(cxt)) != spec.cxtHash {
			t.Fatal("reconstructed native source differs")
		}
		body, err := compiler.CompileCXPMixedProgramBody(p, doc, symbolProgram, true)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile("../um/testdata/" + spec.image)
		if err != nil {
			t.Fatal(err)
		}
		image, err := um.ParseN1(raw)
		if err != nil {
			t.Fatal(err)
		}
		captured := image.Tasks[0].Code
		if !body.HasEnd || len(body.Words) != 151 || !reflect.DeepEqual(body.Words, captured) {
			t.Fatalf("mixed first task differs: got %d words, want %d; got %08x want %08x", len(body.Words), len(captured), body.Words, captured)
		}
		if len(original.Words) != 154 || !reflect.DeepEqual(original.Words[5:], captured[2:]) {
			t.Fatal("original source differences extend beyond known first-rung replacement")
		}
		if len(body.Generated) != 6 {
			t.Fatal("FB scaffolding or section separators lost")
		}
		checkMixedCoverage(t, body)
		checkMixedCoverage(t, original)
		without, err := compiler.CompileCXPMixedProgramBody(p, doc, symbolProgram, false)
		if err != nil {
			t.Fatal(err)
		}
		// Independently fixed native ranges exclude the three known separators.
		wantSource := append([]uint32(nil), captured[:59]...)
		wantSource = append(wantSource, captured[62:103]...)
		wantSource = append(wantSource, captured[106:147]...)
		wantSource = append(wantSource, captured[150:]...)
		if len(without.Words) != 142 || len(without.Generated) != 3 || !reflect.DeepEqual(without.Words, wantSource) {
			t.Fatal("source-only mixed body differs")
		}
		checkMixedCoverage(t, without)
		// The textual-only entry point remains explicit about unsupported graphics.
		if partial, err := compiler.CompileCXPSectionedProgramBody(p, doc, symbolProgram); partial != nil || err == nil {
			t.Fatal("legacy path silently enabled FB lowering")
		}
	}
}

func TestMixedProgramWithoutGraphicsPreservesExistingProgram(t *testing.T) {
	doc := callDocument(t, "cj1m.cxp")
	const program = "Resource[0].Programs.Program[1]"
	for _, markers := range []bool{false, true} {
		got, err := compiler.CompileCXPMixedProgramBody(profile(), doc, program, markers)
		if err != nil {
			t.Fatal(err)
		}
		compile := compiler.CompileCXPProgramBody
		if markers {
			compile = compiler.CompileCXPSectionedProgramBody
		}
		want, err := compile(profile(), doc, program)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got.Words, want.Words) || !reflect.DeepEqual(got.Spans, want.Spans) || len(got.Generated) != len(want.Generated) {
			t.Fatal("text-only source changed in opt-in path")
		}
		checkMixedCoverage(t, got)
	}
}

func TestMixedProgramRejectsUnqualifiedBoundariesAndCalls(t *testing.T) {
	doc := callDocument(t, "cj1m.cxp")
	duplicate := *callField(t, doc, callRung)
	duplicate.Name = "R[3]"
	*callField(t, doc, symbolProgram+".Sections.Sec[0].ProgramData.R[3]") = duplicate
	if body, err := compiler.CompileCXPMixedProgramBody(profile(), doc, symbolProgram, true); body != nil || err == nil {
		t.Fatal("multiple FB calls admitted without qualification")
	}
	for _, edit := range []struct{ path, text string }{
		{symbolProgram + ".Sections.Sec[0].ProgramData.R[3]", "LD W0.00\nIL"},
		{symbolProgram + ".Sections.Sec[0].ProgramData.R[3]", "END"},
		{symbolProgram + ".Sections.Sec[3].ProgramData.R[0]", "NOP"},
		{symbolProgram + ".Sections.Sec[1].ProgramData.R[0]", "ILC"},
	} {
		doc := callDocument(t, "cj1m.cxp")
		if err := doc.SetRung(edit.path, edit.text); err != nil {
			t.Fatal(err)
		}
		if body, err := compiler.CompileCXPMixedProgramBody(profile(), doc, symbolProgram, true); body != nil || err == nil {
			t.Fatal("invalid mixed boundary admitted", edit)
		}
	}
	for _, edit := range []struct{ path, value string }{
		{callRung + ".FBversion", `"old"`}, {callRung + ".TotEl", "6"}, {instanceRoot + ".Instance[0].InstanceNum", "2048"},
	} {
		doc := callDocument(t, "cj1m.cxp")
		callField(t, doc, edit.path).Value = edit.value
		if body, err := compiler.CompileCXPMixedProgramBody(profile(), doc, symbolProgram, true); body != nil || err == nil {
			t.Fatal("unqualified FB call admitted", edit)
		}
	}
	for _, p := range []codec.Profile{{Model: codec.CJ1MCPU21}, {Model: codec.CJ1MCPU21, TimerCounterMode: codec.TimerModeBinary}, {Model: "unknown", TimerCounterMode: codec.TimerModeBCD}} {
		if body, err := compiler.CompileCXPMixedProgramBody(p, callDocument(t, "cj1m.cxp"), symbolProgram, true); body != nil || err == nil {
			t.Fatal("unqualified mixed profile admitted", p)
		}
	}
}
