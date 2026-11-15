package compiler_test

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/um"
)

func TestSavedScalarInstanceRecordsFromSource(t *testing.T) {
	const root = "Resource[0].GlobalVariables.POUInstances."
	for _, spec := range []struct{ source, image, model string }{{"cj1m.cxp", "cj1m-input-bit.n1", codec.CJ1MCPU21}, {"cj1h.cxp", "cj1h-baseline.n1", codec.CJ1HCPU65}} {
		doc := callDocument(t, spec.source)
		profile := codec.Profile{Model: spec.model, TimerCounterMode: codec.TimerModeBCD}
		data, err := os.ReadFile("../um/testdata/" + spec.image)
		if err != nil {
			t.Fatal(err)
		}
		image, err := um.ParseN1(data)
		if err != nil {
			t.Fatal(err)
		}
		blocks, err := image.FunctionBlocks()
		if err != nil {
			t.Fatal(err)
		}
		for i, path := range []string{root + "Instance[0]", root + "Instance[1]"} {
			compiled, err := compiler.CompileCXPFBInstance(profile, doc, path)
			if err != nil {
				t.Fatal(err)
			}
			native := blocks.Instances[i]
			want := append(append([]uint32(nil), native.Header...), native.Code...)
			if !reflect.DeepEqual(compiled.RecordWords, want) || !reflect.DeepEqual(compiled.Body.Words, native.Code) || len(compiled.RecordWords) != 54 || compiled.Body.HasEnd || len(compiled.Body.Spans) != 0 || len(compiled.Body.Generated) != 17 {
				t.Fatal("complete instance frame differs", compiled)
			}
			covered := 0
			for _, span := range compiled.Body.Generated {
				if span.Section != path || span.OffsetWords != covered || span.LengthWords < 1 {
					t.Fatal("invalid generated range")
				}
				covered += span.LengthWords
			}
			if covered != 44 {
				t.Fatal("incomplete generated coverage")
			}
		}
		// Fields are generated from saved placement and source identity, not copied
		// from either native buffer.
		path := root + "Instance[0]"
		callField(t, doc, path+".InstanceNum").Value = "7"
		callField(t, doc, path+".BR0").Value = `"T3080,2"`
		callField(t, doc, path+".BR3").Value = `"H520,3"`
		callField(t, doc, "Resource[0].FunctionBlocks.FunctionBlockWrapper[0].FunctionBlock.TaskID").Value = "1025"
		changed, err := compiler.CompileCXPFBInstance(profile, doc, path)
		if err != nil {
			t.Fatal(err)
		}
		words := changed.Body.Words
		if changed.RecordWords[0] != 7 || words[1] != 0x65440007 || words[3] != 0x5000ec08 || words[13] != 0x5000da08 || words[18] != 0xe203c007 || words[19] != 0xf863c007 || words[34] != 0x65440401 {
			t.Fatal("instance source changes ignored", words)
		}
	}
}

func TestInstanceCompilerRejectsUnqualifiedPlacementAndShape(t *testing.T) {
	const path = "Resource[0].GlobalVariables.POUInstances.Instance[0]"
	for _, edit := range []struct{ path, value string }{
		{path + ".BR0", `"T4095,2"`}, {path + ".BR3", `"H1407,3"`}, {path + ".BR3", `"H512,2"`}, {path + ".BR1", `"C3073,0"`}, {path + ".BR4", `"H1408,1"`}, {path + ".MemoryAllocated", "false"}, {path + ".OnlineEditDiffExpand", "1"}, {path + ".InstanceNum", "2048"}, {path + ".InstanceNum", "3"}, {path + ".POUType", `"Missing"`},
	} {
		doc := callDocument(t, "cj1m.cxp")
		callField(t, doc, edit.path).Value = edit.value
		if result, err := compiler.CompileCXPFBInstance(profile(), doc, path); result != nil || err == nil {
			t.Fatal("unqualified instance admitted", edit, result, err)
		}
	}
	var absent *cxp.Document
	if result, err := compiler.CompileCXPFBInstance(profile(), absent, path); result != nil || err == nil {
		t.Fatal("nil document admitted")
	}
	for _, invalid := range []string{"", "Resource[0].GlobalVariables.POUInstances.Instance[2]", path + ".BR0", "Resource[1].GlobalVariables.POUInstances.Instance[0]"} {
		if result, err := compiler.CompileCXPFBInstance(profile(), callDocument(t, "cj1m.cxp"), invalid); result != nil || err == nil {
			t.Fatal("invalid path admitted", invalid)
		}
	}
	const declaration = "Resource[0].FunctionBlocks.FunctionBlockWrapper[0].FunctionBlock.POUHeader.PouVarList"
	for _, edit := range [][2]string{{"1,PV,1,", "0,PV,1,"}, {"2,OUT,0,", "0,OUT,0,"}} {
		doc := callDocument(t, "cj1m.cxp")
		list := callField(t, doc, declaration)
		before := list.Raw
		list.Raw = strings.ReplaceAll(list.Raw, edit[0], edit[1])
		if list.Raw == before {
			t.Fatal("test declaration not found", edit)
		}
		if result, err := compiler.CompileCXPFBInstance(profile(), doc, path); result != nil || err == nil {
			t.Fatal("unsupported port shape admitted", edit)
		}
	}
}
