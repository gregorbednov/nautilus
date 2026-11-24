package compiler_test

import (
	"encoding/binary"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/um"
)

func checkTaskCoverage(t *testing.T, task *compiler.TaskRecord) {
	t.Helper()
	covered := make([]int, len(task.RecordWords))
	mark := func(offset, length int) {
		if offset < 0 || length < 1 || offset > len(covered)-length {
			t.Fatal("invalid task range")
		}
		for i := offset; i < offset+length; i++ {
			covered[i]++
		}
	}
	for _, span := range task.SourceSpans {
		if span.Line < 1 {
			t.Fatal("lost source line")
		}
		mark(span.OffsetWords, span.LengthWords)
	}
	for _, span := range task.Generated {
		mark(span.OffsetWords, span.LengthWords)
	}
	for i, n := range covered {
		if n != 1 {
			t.Fatal("missing or overlapping task provenance", i, n)
		}
	}
}

func TestGeneratedTaskRecordsMatchWholeNativeRecords(t *testing.T) {
	for _, spec := range []struct{ source, image, model, first string }{
		{"cj1m.cxp", "cj1m-input-bit.n1", codec.CJ1MCPU21, "LD W0.15\nOUT W0.01"},
		{"cj1h.cxp", "cj1h-baseline.n1", codec.CJ1HCPU65, "LD W0.00\nOUT W0.01"},
	} {
		doc := callDocument(t, spec.source)
		if err := doc.SetRung(symbolProgram+".Sections.Sec[0].ProgramData.R[0]", spec.first); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile("../um/testdata/" + spec.image)
		if err != nil {
			t.Fatal(err)
		}
		// Use the qualified read-only parser solely to locate the captured records.
		image, err := um.ParseN1(raw)
		if err != nil {
			t.Fatal(err)
		}
		for i, program := range []string{symbolProgram, "Resource[0].Programs.Program[1]"} {
			task, err := compiler.CompileCXPTaskRecord(codec.Profile{Model: spec.model, TimerCounterMode: codec.TimerModeBCD}, doc, program)
			if err != nil {
				t.Fatal(err)
			}
			native := image.Tasks[i]
			var want []uint32
			for j := native.OffsetWords; j < native.OffsetWords+native.LengthWords; j++ {
				want = append(want, binary.BigEndian.Uint32(raw[int(j)*4:]))
			}
			if !reflect.DeepEqual(task.RecordWords, want) || len(task.RecordWords) != []int{188, 176}[i] || task.Source.Number != 256+i || !reflect.DeepEqual(task.Code.Words, native.Code) {
				t.Fatal("whole task record differs", spec.source, i)
			}
			if !reflect.DeepEqual(task.RecordWords[:32], native.Header) || task.RecordWords[26] != uint32(len(want)) || task.RecordWords[27] != 32 {
				t.Fatal("task header differs")
			}
			checkTaskCoverage(t, task)
		}
	}
	original, err := compiler.CompileCXPTaskRecord(profile(), callDocument(t, "cj1m.cxp"), symbolProgram)
	if err != nil || len(original.RecordWords) != 191 {
		t.Fatal("original source task length incorrect", err)
	}
}

func TestTaskRecordMetadataAndBoundaries(t *testing.T) {
	doc := callDocument(t, "cj1m.cxp")
	callField(t, doc, symbolProgram+".TaskID").Value = "287"
	callField(t, doc, symbolProgram+".Name").Value = `"ABCDEFGHIJKLMNOPQRSTUVWX"`
	task, err := compiler.CompileCXPTaskRecord(profile(), doc, symbolProgram)
	if err != nil {
		t.Fatal(err)
	}
	if task.RecordWords[0] != 287 || task.RecordWords[34] != 0x6144011f || !reflect.DeepEqual(task.RecordWords[1:9], []uint32{0x414243, 0x444546, 0x474849, 0x4a4b4c, 0x4d4e4f, 0x505152, 0x535455, 0x565758}) {
		t.Fatal("source ID/name not emitted correctly")
	}
	checkTaskCoverage(t, task)
	for _, edit := range []struct{ field, value string }{
		{"TaskID", "255"}, {"TaskID", "288"}, {"TaskID", "289"}, {"TaskID", "-1"}, {"TaskID", "0256"},
		{"Name", `""`}, {"Name", `"` + strings.Repeat("A", 25) + `"`}, {"Name", `"Задача"`}, {"Name", `"A B"`},
		{"Protection.Enabled", "1"}, {"OpStart", "2"},
	} {
		doc := callDocument(t, "cj1m.cxp")
		callField(t, doc, symbolProgram+"."+edit.field).Value = edit.value
		if result, err := compiler.CompileCXPTaskRecord(profile(), doc, symbolProgram); result != nil || err == nil {
			t.Fatal("unqualified task metadata admitted", edit)
		}
	}
}
