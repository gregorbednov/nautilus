package compiler_test

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/um"
)

func TestSourceDefinitionRecordsMatchEntireNativeRecords(t *testing.T) {
	const path = "Resource[0].FunctionBlocks.FunctionBlockWrapper[0].FunctionBlock"
	for _, spec := range []struct{ source, image, model string }{{"cj1m.cxp", "cj1m-input-bit.n1", codec.CJ1MCPU21}, {"cj1h.cxp", "cj1h-baseline.n1", codec.CJ1HCPU65}} {
		doc := callDocument(t, spec.source)
		profile := codec.Profile{Model: spec.model, TimerCounterMode: codec.TimerModeBCD}
		compiled, err := compiler.CompileCXPFBDefinition(profile, doc, path, 0)
		if err != nil {
			t.Fatal(err)
		}
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
		native := blocks.Definitions[0]
		want := append(append([]uint32(nil), native.Header...), native.Code...)
		if compiled.Number != 0 || compiled.BodyNumber != 1024 || compiled.Name != "BLINK" || compiled.InputsIncludingEN != 4 || compiled.OutputsIncludingENO != 2 || !reflect.DeepEqual(compiled.RecordWords, want) || !reflect.DeepEqual(compiled.Body.Words, native.Code) || len(compiled.RecordWords) != 46 || len(compiled.Body.Generated) != 2 || len(compiled.Body.Spans) != 15 || compiled.Body.HasEnd {
			t.Fatal("source definition record differs", compiled)
		}
		coverage := make([]int, 23)
		for _, span := range compiled.Body.Spans {
			if span.Line < 1 || !strings.HasPrefix(span.Rung, path+".POUBody") {
				t.Fatal("lost source origin")
			}
			for i := span.OffsetWords; i < span.OffsetWords+span.LengthWords; i++ {
				coverage[i]++
			}
		}
		for _, span := range compiled.Body.Generated {
			if span.Section != path || !strings.HasPrefix(span.Kind, "fb-definition-") {
				t.Fatal("lost generated origin")
			}
			for i := span.OffsetWords; i < span.OffsetWords+span.LengthWords; i++ {
				coverage[i]++
			}
		}
		for _, count := range coverage {
			if count != 1 {
				t.Fatal("missing/overlapping word provenance")
			}
		}
		// Header identity is explicit and separate from source body identity.
		callField(t, doc, path+".Name").Value = `"Changed_FB9"`
		callField(t, doc, path+".TaskID").Value = "1025"
		changed, err := compiler.CompileCXPFBDefinition(profile, doc, path, 7)
		if err != nil {
			t.Fatal(err)
		}
		if changed.RecordWords[0] != 7 || changed.Body.Words[1] != 0x65440401 || changed.Name != "Changed_FB9" || reflect.DeepEqual(changed.RecordWords[3:19], compiled.RecordWords[3:19]) {
			t.Fatal("source/assigned metadata ignored")
		}
		//64 explicit inputs plus EN fit the measured setter bound;65 do not.
		doc = callDocument(t, spec.source)
		list := callField(t, doc, path+".POUHeader.PouVarList")
		for i := 0; i < 61; i++ {
			list.Raw += fmt.Sprintf("\n1,Extra%d,0,,0,FALSE,0,;", i)
		}
		callField(t, doc, path+".POUHeader.PouVarCount").Value = "68"
		large, err := compiler.CompileCXPFBDefinition(profile, doc, path, 0)
		if err != nil || large.InputsIncludingEN != 65 || large.RecordWords[22] != 0x4102 {
			t.Fatal("port boundary differs", err)
		}
		list.Raw += "\n1,TooMany,0,,0,FALSE,0,;"
		callField(t, doc, path+".POUHeader.PouVarCount").Value = "69"
		if result, err := compiler.CompileCXPFBDefinition(profile, doc, path, 0); result != nil || err == nil {
			t.Fatal("excess input ports admitted")
		}
	}
}
