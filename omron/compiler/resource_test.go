package compiler_test

import (
	"encoding/binary"
	"os"
	"reflect"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/um"
)

func nativeWordsFrom(raw []byte, start int) []uint32 {
	words := make([]uint32, len(raw)/4-start)
	for i := range words {
		words[i] = binary.BigEndian.Uint32(raw[(start+i)*4:])
	}
	return words
}

func TestTaskBlockAndResourceBodyMatchWholeNativeTail(t *testing.T) {
	for _, spec := range []struct{ source, image, model, first string }{
		{"cj1m.cxp", "cj1m-input-bit.n1", codec.CJ1MCPU21, "LD W0.15\nOUT W0.01"},
		{"cj1h.cxp", "cj1h-baseline.n1", codec.CJ1HCPU65, "LD W0.00\nOUT W0.01"},
	} {
		doc := callDocument(t, spec.source)
		if err := doc.SetRung(symbolProgram+".Sections.Sec[0].ProgramData.R[0]", spec.first); err != nil {
			t.Fatal(err)
		}
		before, err := doc.MarshalCXT()
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
		profile := codec.Profile{Model: spec.model, TimerCounterMode: codec.TimerModeBCD}
		block, err := compiler.CompileCXPTaskBlock(profile, doc, "Resource[0]")
		if err != nil {
			t.Fatal(err)
		}
		want := nativeWordsFrom(raw, int(image.DirectoryOffsetWords))
		if len(block.Words) != 652 || !reflect.DeepEqual(block.Words, want) || !reflect.DeepEqual(block.DirectoryWords, want[:288]) || block.DirectoryWords[256] != 288 || block.DirectoryWords[257] != 476 || len(block.Tasks) != 2 {
			t.Fatal("complete task block differs", spec.source)
		}
		covered := make([]int, len(block.Words))
		mark := func(offset, length int) {
			if offset < 0 || length < 1 || offset > len(covered)-length {
				t.Fatal("bad block span")
			}
			for i := offset; i < offset+length; i++ {
				covered[i]++
			}
		}
		for _, span := range block.SourceSpans {
			mark(span.OffsetWords, span.LengthWords)
		}
		for _, span := range block.Generated {
			mark(span.OffsetWords, span.LengthWords)
		}
		for i, n := range covered {
			if n != 1 {
				t.Fatal("task block coverage differs", i, n)
			}
		}
		for i, span := range block.Records {
			if span.Number != 256+i || span.OffsetWords != []int{288, 476}[i] || span.LengthWords != []int{188, 176}[i] {
				t.Fatal("wrong block record offsets", span)
			}
		}
		body, err := compiler.CompileCXPResourceBody(profile, doc, "Resource[0]", map[string]int{"BLINK": 0}, [7]uint32{})
		if err != nil {
			t.Fatal(err)
		}
		if len(body.Words) != 818 || !reflect.DeepEqual(body.Words, nativeWordsFrom(raw, 330)) || body.DefinitionOffsetWords != 0 || body.InstanceOffsetWords != 50 || body.TaskDirectoryOffsetWords != 166 || len(body.Calls) != 1 || body.Calls[0].Rung != callRung || body.Calls[0].InstanceNumber != 0 || body.Calls[0].BodyNumber != 1024 {
			t.Fatal("whole resource body or call references differ", spec.source)
		}
		after, err := doc.MarshalCXT()
		if err != nil || !reflect.DeepEqual(before, after) {
			t.Fatal("source mutated", err)
		}
	}
	original, err := compiler.CompileCXPResourceBody(profile(), callDocument(t, "cj1m.cxp"), "Resource[0]", map[string]int{"BLINK": 0}, [7]uint32{})
	if err != nil || len(original.Words) != 821 || original.TaskBlock.DirectoryWords[257] != 479 {
		t.Fatal("original source length not reflected", err)
	}
}

func TestTaskBlockUsesSortedIDsAndSparseDirectory(t *testing.T) {
	const second = "Resource[0].Programs.Program[1]"
	doc := callDocument(t, "cj1m.cxp")
	callField(t, doc, symbolProgram+".TaskID").Value = "257"
	callField(t, doc, second+".TaskID").Value = "256"
	block, err := compiler.CompileCXPTaskBlock(profile(), doc, "Resource[0]")
	if err != nil {
		t.Fatal(err)
	}
	if block.Tasks[0].Source.Path != second || block.Tasks[1].Source.Path != symbolProgram || block.DirectoryWords[256] != 288 || block.DirectoryWords[257] != 464 || block.Records[0].LengthWords != 176 || block.Records[1].LengthWords != 191 {
		t.Fatal("task records follow source order instead of IDs")
	}
	for _, record := range block.Records {
		independent, err := compiler.CompileCXPTaskRecord(profile(), doc, record.Program)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(block.Words[record.OffsetWords:record.OffsetWords+record.LengthWords], independent.RecordWords) {
			t.Fatal("reordered task record changed")
		}
	}
	callField(t, doc, symbolProgram+".TaskID").Value = "260"
	callField(t, doc, second+".TaskID").Value = "287"
	block, err = compiler.CompileCXPTaskBlock(profile(), doc, "Resource[0]")
	if err != nil {
		t.Fatal(err)
	}
	for number, offset := range block.DirectoryWords {
		want := uint32(0)
		if number == 260 {
			want = 288
		}
		if number == 287 {
			want = 479
		}
		if offset != want {
			t.Fatal("sparse task directory incorrect", number, offset, want)
		}
	}
}

func TestResourceCallLinksFollowExplicitSourceIdentities(t *testing.T) {
	doc := callDocument(t, "cj1m.cxp")
	callField(t, doc, instanceRoot+".Instance[0].InstanceNum").Value = "7"
	callField(t, doc, firstDefinition+".TaskID").Value = "1025"
	body, err := compiler.CompileCXPResourceBody(profile(), doc, "Resource[0]", map[string]int{"BLINK": 9}, [7]uint32{})
	if err != nil {
		t.Fatal(err)
	}
	if len(body.Calls) != 1 || body.Calls[0].InstanceNumber != 7 || body.Calls[0].BodyNumber != 1025 || body.FBPrefix.Definitions.Definitions[0].Number != 9 || body.FBPrefix.Instances.Instances.Instances[0].Source.Number != 7 || body.TaskBlock.DirectoryWords[257] != 479 {
		t.Fatal("source identity changes were not linked")
	}
}

func TestResourceCompilationRejectsAmbiguityAtomically(t *testing.T) {
	const second = "Resource[0].Programs.Program[1]"
	for _, edit := range []struct{ path, value string }{
		{second + ".TaskID", "256"}, {second + ".TaskID", "288"}, {second + ".Name", `"motordrive_lift_1"`},
		{"Resource[0].Programs.ProgramCount", "1"}, {second + ".Protection.Enabled", "1"},
	} {
		doc := callDocument(t, "cj1m.cxp")
		callField(t, doc, edit.path).Value = edit.value
		if result, err := compiler.CompileCXPTaskBlock(profile(), doc, "Resource[0]"); result != nil || err == nil {
			t.Fatal("invalid task set admitted", edit)
		}
	}
	doc := callDocument(t, "cj1m.cxp")
	callField(t, doc, firstDefinition+".TaskID").Value = "256"
	if result, err := compiler.CompileCXPResourceBody(profile(), doc, "Resource[0]", map[string]int{"BLINK": 0}, [7]uint32{}); result != nil || err == nil {
		t.Fatal("unqualified FB/task ID overlap admitted")
	}
	doc = callDocument(t, "cj1m.cxp")
	callField(t, doc, callRung+".FBversion").Value = `"old"`
	if result, err := compiler.CompileCXPResourceBody(profile(), doc, "Resource[0]", map[string]int{"BLINK": 0}, [7]uint32{}); result != nil || err == nil {
		t.Fatal("unresolved call emitted partial resource body")
	}
	if result, err := compiler.CompileCXPTaskBlock(profile(), nil, "Resource[0]"); result != nil || err == nil {
		t.Fatal("nil source admitted")
	}
	for _, path := range []string{"", symbolProgram, "Resource[1]"} {
		if result, err := compiler.CompileCXPTaskBlock(profile(), callDocument(t, "cj1m.cxp"), path); result != nil || err == nil {
			t.Fatal("bad resource scope admitted", path)
		}
	}
	doc = callDocument(t, "cj1m.cxp")
	programs := callField(t, doc, "Resource[0].Programs")
	var count cxp.Field
	for _, field := range programs.Fields {
		if field.Name == "ProgramCount" {
			count = field
		}
	}
	count.Value = "0"
	programs.Fields = []cxp.Field{count}
	if result, err := compiler.CompileCXPTaskBlock(profile(), doc, "Resource[0]"); result != nil || err == nil {
		t.Fatal("empty task block emitted")
	}
}
