package compiler_test

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/um"
)

func noFBDocument(t *testing.T) *cxp.Document {
	t.Helper()
	raw, err := os.ReadFile("testdata/n1-no-fb.cxt")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := cxp.Read(raw)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestCompleteN1WithoutFBMatchesNativeFiles(t *testing.T) {
	for _, spec := range []struct{ model, file string }{{codec.CJ1MCPU21, "cj1m"}, {codec.CJ1HCPU65, "cj1h-h"}} {
		t.Run(spec.file, func(t *testing.T) {
			settings := imageSettings(t)
			settings.DefinitionNumbers = map[string]int{}
			image, err := compiler.CompileCXPImage(codec.Profile{Model: spec.model, TimerCounterMode: codec.TimerModeBCD}, noFBDocument(t), "Resource[0]", settings)
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile("testdata/n1-no-fb-" + spec.file + ".n1")
			if err != nil {
				t.Fatal(err)
			}
			if fmt.Sprintf("%x", sha256.Sum256(want)) != "598e01ac119b890c94f5495e267269fa9546aff32e130f481d68e1ed35601b84" || !bytes.Equal(image.Bytes(), want) {
				t.Fatal("complete image differs from archived native no-FB file")
			}
			if image.Summary().ByteLength != 2624 || image.Summary().DirectoryOffsetWords != 330 || len(image.Tasks) != 1 || image.Tasks[0].OffsetWords != 618 || image.Tasks[0].LengthWords != 38 || image.Summary().HardwareQualified {
				t.Fatal("incorrect no-FB image geometry/qualification")
			}
			parsed, err := um.ParseN1(image.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			blocks, err := parsed.FunctionBlocks()
			if err != nil || len(blocks.Definitions) != 0 || len(blocks.Instances) != 0 || len(image.ResourceBody.FBPrefix.Words) != 0 {
				t.Fatal("empty sections emitted", err)
			}
			for _, w := range image.HeaderWords[15:32] {
				if w != 0 {
					t.Fatal("unused configured FB memory emitted")
				}
			}
		})
	}
}

func TestCompleteN1FromCXProgrammerProbeWithoutCounts(t *testing.T) {
	raw, err := os.ReadFile("testdata/n1-probe-cj1m.cxp")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := cxp.Read(raw)
	if err != nil {
		t.Fatal(err)
	}
	settings := imageSettings(t)
	settings.DefinitionNumbers = map[string]int{}
	image, err := compiler.CompileCXPImage(profile(), doc, "Resource[0]", settings)
	if err != nil {
		t.Fatal(err)
	}
	// SHA-256 of the complete N1 captured during the CX-Programmer Probe transfer.
	const native = "07ea1c03b4f065d2fcc29433cf21ce05aa3ff45bed03f7a69eebfa7ddd010257"
	if got := fmt.Sprintf("%x", sha256.Sum256(image.Bytes())); got != native {
		t.Fatalf("Probe image SHA-256 = %s, want %s", got, native)
	}
}

func TestCompleteN1WithoutFBRejectsUnmatchedState(t *testing.T) {
	for _, mutate := range []func(*cxp.Document, *compiler.N1ImageSettings){
		func(_ *cxp.Document, s *compiler.N1ImageSettings) { s.DefinitionNumbers["Unused"] = 0 },
		func(_ *cxp.Document, s *compiler.N1ImageSettings) { s.NextEnable[6] = 1 },
		func(_ *cxp.Document, s *compiler.N1ImageSettings) { s.Memory.Bases = s.Memory.Bases[:4] },
		func(_ *cxp.Document, s *compiler.N1ImageSettings) { s.Memory.Bases[0].Address = "T4096" },
		func(_ *cxp.Document, s *compiler.N1ImageSettings) { s.Memory.Stack.Address = "D0" },
		func(d *cxp.Document, _ *compiler.N1ImageSettings) {
			callField(t, d, "Resource[0].FunctionBlocks.FunctionBlockCount").Value = "1"
		},
		func(d *cxp.Document, _ *compiler.N1ImageSettings) {
			callField(t, d, instanceRoot+".NumInstances").Value = "1"
		},
		func(d *cxp.Document, _ *compiler.N1ImageSettings) {
			callField(t, d, symbolProgram+".LocalVariables.POUInstances.NumInstances").Value = "1"
		},
		func(d *cxp.Document, _ *compiler.N1ImageSettings) {
			callField(t, d, "Resource[0].GlobalVariables.VariableCount").Value = "1"
			callField(t, d, "Resource[0].GlobalVariables.VariableList").Raw = `"Lost","[Auto]","FUNCTION BLOCK","Missing","A",""`
		},
		func(d *cxp.Document, _ *compiler.N1ImageSettings) {
			callField(t, d, symbolProgram+".LocalVariables.VariableCount").Value = "1"
			callField(t, d, symbolProgram+".LocalVariables.VariableList").Raw = `"Lost","[Auto]","FUNCTION BLOCK","Missing","A",""`
		},
		func(d *cxp.Document, _ *compiler.N1ImageSettings) {
			callField(t, d, symbolProgram+".Sections.Sec[0].ProgramData.R[0].SL").Value = `"UNKNOWN"`
		},
	} {
		for _, mode := range []codec.TimerCounterMode{codec.TimerModeBCD, codec.TimerModeBinary} {
			doc, settings := noFBDocument(t), imageSettings(t)
			settings.DefinitionNumbers = map[string]int{}
			mutate(doc, &settings)
			if result, err := compiler.CompileCXPImage(codec.Profile{Model: codec.CJ1MCPU21, TimerCounterMode: mode}, doc, "Resource[0]", settings); result != nil || err == nil {
				t.Fatal("unmatched FB state, memory or invalid code silently discarded", mode)
			}
		}
	}
}
