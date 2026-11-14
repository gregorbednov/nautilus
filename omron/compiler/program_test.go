package compiler_test

import (
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/um"
)

func TestOriginalProgramSourceBodyAgainstCapturedTask(t *testing.T) {
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
		// Inventory every native source rung, not only hand-picked anchors.
		textual, graphical := 0, 0
		for _, owner := range []string{"Resource[0].Programs.Program[0]", "Resource[0].Programs.Program[1]"} {
			source, err := doc.SourceForProgram(owner)
			if err != nil {
				t.Fatal(err)
			}
			for _, section := range source.Sections {
				for _, r := range section.Rungs {
					if len(r.Graphic) > 0 {
						graphical++
						continue
					}
					if len(r.Instructions) == 0 {
						continue
					}
					if _, err := compiler.CompileCXPProgramRungs(p, doc, owner, []cxp.Rung{r}); err != nil {
						t.Fatal("uncompiled source rung", r.Path, err)
					}
					textual++
				}
			}
		}
		if textual != 41 || graphical != 1 {
			t.Fatal("original coverage changed", textual, graphical)
		}
		body, err := compiler.CompileCXPProgramBody(p, doc, "Resource[0].Programs.Program[1]")
		if err != nil {
			t.Fatal(err)
		}
		if !body.HasEnd || len(body.Words) != 133 || len(body.Spans) != 133 || len(body.Bindings) == 0 {
			t.Fatal("incomplete program body", body)
		}
		raw, err := os.ReadFile("../um/testdata/" + spec.image)
		if err != nil {
			t.Fatal(err)
		}
		image, err := um.ParseN1(raw)
		if err != nil {
			t.Fatal(err)
		}
		captured := image.Tasks[1].Code
		// Compare all source instructions in two explicitly anchored ranges.
		// Do not strip unknown sequences generically or emit them in production.
		marker := []uint32{0xb2400100, 0x6140dead, 0x6540dead}
		if len(captured) != 139 || !reflect.DeepEqual(captured[:3], marker) || !reflect.DeepEqual(captured[135:138], marker) || !reflect.DeepEqual(body.Words[:132], captured[3:135]) || !reflect.DeepEqual(body.Words[132:], captured[138:]) {
			t.Fatal("source program differs from captured task", spec.project)
		}
		if partial, err := compiler.CompileCXPProgramBody(p, doc, "Resource[0].Programs.Program[0]"); !errors.Is(err, compiler.ErrUnsupportedRung) || partial != nil {
			t.Fatal("graphical call omitted", err)
		}
		endPath := "Resource[0].Programs.Program[1].Sections.Sec[2].ProgramData.R[0]"
		if err := doc.SetRung(endPath, "NOP"); err != nil {
			t.Fatal(err)
		}
		if partial, err := compiler.CompileCXPProgramBody(p, doc, "Resource[0].Programs.Program[1]"); !errors.Is(err, compiler.ErrStructure) || partial != nil {
			t.Fatal("program without END emitted", err)
		}
	}
}
