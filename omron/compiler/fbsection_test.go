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

func TestInstanceSectionMatchesWholeNativeSection(t *testing.T) {
	for _, spec := range []struct{ source, image, model string }{{"cj1m.cxp", "cj1m-input-bit.n1", codec.CJ1MCPU21}, {"cj1h.cxp", "cj1h-baseline.n1", codec.CJ1HCPU65}} {
		doc := callDocument(t, spec.source)
		section, err := compiler.CompileCXPFBInstanceSection(codec.Profile{Model: spec.model, TimerCounterMode: codec.TimerModeBCD}, doc, "Resource[0]", [7]uint32{})
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
		start := int(blocks.Instances[0].OffsetWords) - 8
		end := int(blocks.Instances[1].OffsetWords + blocks.Instances[1].LengthWords)
		var want []uint32
		for i := start; i < end; i++ {
			want = append(want, binary.BigEndian.Uint32(data[i*4:]))
		}
		if len(section.Words) != 116 || !reflect.DeepEqual(section.Words, want) || len(section.Records) != 2 {
			t.Fatal("complete section differs")
		}
		for i, span := range section.Records {
			if span.Instance != instanceRoot+[]string{".Instance[0]", ".Instance[1]"}[i] || span.OffsetWords != 8+i*54 || span.LengthWords != 54 || !reflect.DeepEqual(section.Words[span.OffsetWords:span.OffsetWords+span.LengthWords], section.Instances.Instances[i].RecordWords) {
				t.Fatal("invalid record offsets", span)
			}
		}
	}
}

func TestInstanceSectionRejectsUnqualifiedStateAndPlacement(t *testing.T) {
	for i := 0; i < 7; i++ {
		next := [7]uint32{}
		next[i] = 1
		if result, err := compiler.CompileCXPFBInstanceSection(profile(), callDocument(t, "cj1m.cxp"), "Resource[0]", next); result != nil || err == nil {
			t.Fatal("nonzero state generalized", i)
		}
	}
	doc := callDocument(t, "cj1m.cxp")
	callField(t, doc, instanceRoot+".Instance[1].BR3").Value = `"H514,3"`
	if result, err := compiler.CompileCXPFBInstanceSection(profile(), doc, "Resource[0]", [7]uint32{}); result != nil || err == nil {
		t.Fatal("invalid placement emitted section")
	}
	// Remove the second saved declaration and symbol: count, length and offsets
	// must be generated rather than copied from either captured section.
	doc = callDocument(t, "cj1m.cxp")
	records := callField(t, doc, instanceRoot)
	var kept []cxp.Field
	for _, field := range records.Fields {
		if field.Name != "Instance[1]" {
			kept = append(kept, field)
		}
	}
	records.Fields = kept
	callField(t, doc, instanceRoot+".NumInstances").Value = "1"
	list := callField(t, doc, "Resource[0].GlobalVariables.VariableList")
	var lines []string
	for _, line := range strings.Split(list.Raw, "\n") {
		if !strings.Contains(line, "BLINK_L2,") {
			lines = append(lines, line)
		}
	}
	list.Raw = strings.Join(lines, "\n")
	callField(t, doc, "Resource[0].GlobalVariables.VariableCount").Value = "15"
	section, err := compiler.CompileCXPFBInstanceSection(profile(), doc, "Resource[0]", [7]uint32{})
	if err != nil || section == nil || len(section.Words) != 62 || section.Words[0] != 1 || len(section.Records) != 1 {
		t.Fatal("source set change ignored", section, err)
	}
	// An absent section is handled by the future linker, not a count-zero record.
	records = callField(t, doc, instanceRoot)
	kept = nil
	for _, field := range records.Fields {
		if field.Name == "NumInstances" {
			kept = append(kept, field)
		}
	}
	records.Fields = kept
	callField(t, doc, instanceRoot+".NumInstances").Value = "0"
	list = callField(t, doc, "Resource[0].GlobalVariables.VariableList")
	lines = nil
	for _, line := range strings.Split(list.Raw, "\n") {
		if !strings.Contains(line, "BLINK_L1,") {
			lines = append(lines, line)
		}
	}
	list.Raw = strings.Join(lines, "\n")
	callField(t, doc, "Resource[0].GlobalVariables.VariableCount").Value = "14"
	if section, err := compiler.CompileCXPFBInstanceSection(profile(), doc, "Resource[0]", [7]uint32{}); section != nil || err == nil {
		t.Fatal("empty section emitted")
	}
}
