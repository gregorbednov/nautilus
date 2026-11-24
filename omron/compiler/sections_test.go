package compiler_test

import (
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/um"
)

func TestSectionedProgramMatchesEntireCapturedCode(t *testing.T) {
	for _, spec := range []struct{ project, image, model string }{{"cj1m.cxp", "cj1m-input-bit.n1", codec.CJ1MCPU21}, {"cj1h.cxp", "cj1h-baseline.n1", codec.CJ1HCPU65}} {
		data, err := os.ReadFile("../cxp/testdata/" + spec.project)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := cxp.Read(data)
		if err != nil {
			t.Fatal(err)
		}
		p := profile()
		p.Model = spec.model
		body, err := compiler.CompileCXPSectionedProgramBody(p, doc, "Resource[0].Programs.Program[1]")
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
		if !reflect.DeepEqual(body.Words, image.Tasks[1].Code) || len(body.Generated) != 2 || len(body.Spans) != 133 || !body.HasEnd {
			t.Fatal("sectioned program differs from complete captured code", spec.project)
		}
		covered := make([]bool, len(body.Words))
		for i, span := range body.Generated {
			if span.Kind != "section-separator" || span.LengthWords != 3 || span.OffsetWords != []int{0, 135}[i] || !strings.HasSuffix(span.Section, []string{".Sec[1]", ".Sec[2]"}[i]) {
				t.Fatal("wrong generated provenance", span)
			}
			decoded, n, err := codec.DecodeInstruction(p, body.Words[span.OffsetWords:])
			if err != nil || n != 3 || decoded.Mnemonic != "CMP" || !reflect.DeepEqual(decoded.Operands, []string{"#DEAD", "#DEAD"}) {
				t.Fatal("bad section separator", decoded, err)
			}
			for j := span.OffsetWords; j < span.OffsetWords+span.LengthWords; j++ {
				covered[j] = true
			}
		}
		for _, span := range body.Spans {
			for j := span.OffsetWords; j < span.OffsetWords+span.LengthWords; j++ {
				if j >= len(covered) || covered[j] {
					t.Fatal("source/generated span overlap")
				}
				covered[j] = true
			}
		}
		for _, yes := range covered {
			if !yes {
				t.Fatal("unattributed generated word")
			}
		}
		if partial, err := compiler.CompileCXPSectionedProgramBody(p, doc, "Resource[0].Programs.Program[0]"); !errors.Is(err, compiler.ErrUnsupportedRung) || partial != nil {
			t.Fatal("FB call omitted", err)
		}
		cross, err := cxp.Read(data)
		if err != nil {
			t.Fatal(err)
		}
		if err := cross.SetRung("Resource[0].Programs.Program[1].Sections.Sec[0].ProgramData.R[0]", "LD W0.00\nIL"); err != nil {
			t.Fatal(err)
		}
		if err := cross.SetRung("Resource[0].Programs.Program[1].Sections.Sec[2].ProgramData.R[0]", "ILC\nEND"); err != nil {
			t.Fatal(err)
		}
		if _, err := compiler.CompileCXPProgramBody(p, cross, "Resource[0].Programs.Program[1]"); err != nil {
			t.Fatal("invalid cross-section negative control", err)
		}
		if partial, err := compiler.CompileCXPSectionedProgramBody(p, cross, "Resource[0].Programs.Program[1]"); !errors.Is(err, compiler.ErrStructure) || partial != nil {
			t.Fatal("cross-section interlock silently changed", err)
		}
		// An empty noninitial section is explicitly outside the measured layout.
		source, err := doc.SourceForProgram("Resource[0].Programs.Program[1]")
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range source.Sections[1].Rungs {
			if len(r.Instructions) > 0 {
				if err := doc.SetRung(r.Path, ""); err != nil {
					t.Fatal(err)
				}
			}
		}
		if partial, err := compiler.CompileCXPSectionedProgramBody(p, doc, "Resource[0].Programs.Program[1]"); !errors.Is(err, compiler.ErrUnsupportedRung) || partial != nil {
			t.Fatal("empty later section was guessed", err)
		}
	}
}

func TestCMPRequiresConditionAndResolvesReadOnlySources(t *testing.T) {
	for _, text := range []string{"CMP #DEAD #BEEF", "LD W0.00\nCMP #DEAD #DEAD\nEND", "LD W0.00\n@CMP #DEAD #DEAD", "LD W0.00\nCMP(999) #DEAD #DEAD"} {
		if partial, err := compiler.CompileRungs(profile(), []compiler.Rung{rung("comparison", text)}); err == nil || partial != nil {
			t.Fatal("invalid comparison compiled", text)
		}
	}
	doc := bindingDocument("Left,A0,WORD,,,;\nRight,D0,UINT,,,;", "")
	body, err := compiler.CompileCXPProgramRungs(profile(), doc, symbolProgram, symbolicRung("LD W0.00\nCMP Left Right"))
	if err != nil {
		t.Fatal(err)
	}
	if len(body.Bindings) != 2 || !reflect.DeepEqual(body.Words, []uint32{0xe205e000, 0xb2400100, 0x6000b840, 0x64010000}) {
		t.Fatal("comparison source types/positions changed", body)
	}
}
