package compiler_test

import (
	"encoding/binary"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/um"
)

const firstDefinition = "Resource[0].FunctionBlocks.FunctionBlockWrapper[0].FunctionBlock"

func TestDefinitionSectionAndCombinedPrefixMatchNative(t *testing.T) {
	for _, spec := range []struct{ source, image, model string }{{"cj1m.cxp", "cj1m-input-bit.n1", codec.CJ1MCPU21}, {"cj1h.cxp", "cj1h-baseline.n1", codec.CJ1HCPU65}} {
		doc := callDocument(t, spec.source)
		before, err := doc.MarshalCXT()
		if err != nil {
			t.Fatal(err)
		}
		profile := codec.Profile{Model: spec.model, TimerCounterMode: codec.TimerModeBCD}
		prefix, err := compiler.CompileCXPFBPrefix(profile, doc, "Resource[0]", map[string]int{"BLINK": 0}, [7]uint32{})
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
		start := int(blocks.Definitions[0].OffsetWords) - 4
		end := int(blocks.Instances[1].OffsetWords + blocks.Instances[1].LengthWords)
		var want []uint32
		for i := start; i < end; i++ {
			want = append(want, binary.BigEndian.Uint32(data[i*4:]))
		}
		if len(prefix.Words) != 166 || prefix.DefinitionOffsetWords != 0 || prefix.InstanceOffsetWords != 50 || !reflect.DeepEqual(prefix.Words, want) {
			t.Fatal("whole FB prefix differs")
		}
		definitions := prefix.Definitions
		if len(definitions.Definitions) != 1 || len(definitions.Words) != 50 || !reflect.DeepEqual(definitions.Words, want[:50]) || !reflect.DeepEqual(definitions.Words[:4], []uint32{1, 0, 0, 0}) || len(definitions.Records) != 1 || definitions.Records[0].Definition != firstDefinition || definitions.Records[0].OffsetWords != 4 || definitions.Records[0].LengthWords != 46 {
			t.Fatal("definition section differs", definitions)
		}
		after, err := doc.MarshalCXT()
		if err != nil || !reflect.DeepEqual(before, after) {
			t.Fatal("source mutated", err)
		}
		// Record numbers are independent from the FBBN and FBBCALL body numbers.
		changed, err := compiler.CompileCXPFBPrefix(profile, doc, "Resource[0]", map[string]int{"BLINK": 7}, [7]uint32{})
		if err != nil {
			t.Fatal(err)
		}
		wantChanged := append([]uint32(nil), want...)
		wantChanged[4] = 7
		if !reflect.DeepEqual(changed.Words, wantChanged) {
			t.Fatal("record renumbering changed body identity")
		}
	}
}

func addSecondFBDefinition(t *testing.T, doc *cxp.Document, name, task string) {
	t.Helper()
	other := callDocument(t, "cj1m.cxp")
	callField(t, other, firstDefinition+".Name").Value = `"` + name + `"`
	callField(t, other, firstDefinition+".TaskID").Value = task
	wrapper := *callField(t, other, "Resource[0].FunctionBlocks.FunctionBlockWrapper[0]")
	wrapper.Name = "FunctionBlockWrapper[1]"
	blocks := callField(t, doc, "Resource[0].FunctionBlocks")
	blocks.Fields = append(blocks.Fields, wrapper)
	callField(t, doc, "Resource[0].FunctionBlocks.FunctionBlockCount").Value = "2"
}

func TestDefinitionSectionSourceOrderAndInstanceReferences(t *testing.T) {
	doc := callDocument(t, "cj1m.cxp")
	addSecondFBDefinition(t, doc, "OTHER", "1025")
	callField(t, doc, instanceRoot+".Instance[1].POUType").Value = `"OTHER"`
	list := callField(t, doc, "Resource[0].GlobalVariables.VariableList")
	// Keep the symbol and saved instance declarations in agreement.
	changed := strings.ReplaceAll(list.Raw, `BLINK_L2," [Auto]",FUNCTION BLOCK,BLINK,A,`, `BLINK_L2," [Auto]",FUNCTION BLOCK,OTHER,A,`)
	if changed == list.Raw {
		t.Fatal("symbol mutation failed")
	}
	list.Raw = changed
	prefix, err := compiler.CompileCXPFBPrefix(profile(), doc, "Resource[0]", map[string]int{"BLINK": 9, "OTHER": 2}, [7]uint32{})
	if err != nil {
		t.Fatal(err)
	}
	definitions := prefix.Definitions
	if len(definitions.Words) != 96 || definitions.Words[0] != 2 || prefix.InstanceOffsetWords != 96 || len(prefix.Words) != 212 || definitions.Definitions[0].Number != 9 || definitions.Definitions[1].Number != 2 || definitions.Definitions[0].BodyNumber != 1024 || definitions.Definitions[1].BodyNumber != 1025 || definitions.Records[1].OffsetWords != 50 {
		t.Fatal("source order/numbering lost", definitions)
	}
	if prefix.Instances.Instances.Instances[0].Body.Words[34] != 0x65440400 || prefix.Instances.Instances.Instances[1].Body.Words[34] != 0x65440401 {
		t.Fatal("instance body reference ignored")
	}
}

func TestDefinitionSectionRejectsAmbiguousOrIncompleteInputs(t *testing.T) {
	for _, numbers := range []map[string]int{nil, {}, {"BLINK": -1}, {"BLINK": 1024}, {"blink": 0}, {"Other": 0}, {"BLINK": 0, "Extra": 1}} {
		if result, err := compiler.CompileCXPFBDefinitionSection(profile(), callDocument(t, "cj1m.cxp"), "Resource[0]", numbers); result != nil || err == nil {
			t.Fatal("invalid mapping admitted", numbers)
		}
	}
	for _, spec := range []struct {
		name, task string
		numbers    map[string]int
	}{
		{"OTHER", "1024", map[string]int{"BLINK": 0, "OTHER": 1}},
		{"blink", "1025", map[string]int{"BLINK": 0, "blink": 1}},
		{"OTHER", "1025", map[string]int{"BLINK": 0, "OTHER": 0}},
		{"OTHER", "1025", map[string]int{"BLINK": 0}},
	} {
		doc := callDocument(t, "cj1m.cxp")
		addSecondFBDefinition(t, doc, spec.name, spec.task)
		if result, err := compiler.CompileCXPFBDefinitionSection(profile(), doc, "Resource[0]", spec.numbers); result != nil || err == nil {
			t.Fatal("ambiguous definitions admitted", spec)
		}
	}
	var absent *cxp.Document
	if result, err := compiler.CompileCXPFBDefinitionSection(profile(), absent, "Resource[0]", map[string]int{"BLINK": 0}); result != nil || err == nil {
		t.Fatal("nil document admitted")
	}
	for _, scope := range []string{"", "Resource[1]", firstDefinition} {
		if result, err := compiler.CompileCXPFBDefinitionSection(profile(), callDocument(t, "cj1m.cxp"), scope, map[string]int{"BLINK": 0}); result != nil || err == nil {
			t.Fatal("invalid scope admitted", scope)
		}
	}
	// A late source error must not return the already compiled first record.
	doc := callDocument(t, "cj1m.cxp")
	addSecondFBDefinition(t, doc, "OTHER", "1025")
	callField(t, doc, "Resource[0].FunctionBlocks.FunctionBlockWrapper[1].FunctionBlock.OnlineReserveMemory.BR[0]").Value = `"1,0"`
	if result, err := compiler.CompileCXPFBDefinitionSection(profile(), doc, "Resource[0]", map[string]int{"BLINK": 0, "OTHER": 1}); result != nil || err == nil {
		t.Fatal("partial section emitted")
	}
	doc = callDocument(t, "cj1m.cxp")
	callField(t, doc, instanceRoot+".Instance[1].BR3").Value = `"H514,3"`
	if result, err := compiler.CompileCXPFBPrefix(profile(), doc, "Resource[0]", map[string]int{"BLINK": 0}, [7]uint32{}); result != nil || err == nil {
		t.Fatal("partial prefix emitted on placement error")
	}
	next := [7]uint32{1}
	if result, err := compiler.CompileCXPFBPrefix(profile(), callDocument(t, "cj1m.cxp"), "Resource[0]", map[string]int{"BLINK": 0}, next); result != nil || err == nil {
		t.Fatal("unqualified state admitted")
	}
}
