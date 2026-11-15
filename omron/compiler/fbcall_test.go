package compiler_test

import (
	"bytes"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/um"
)

const callRung = symbolProgram + ".Sections.Sec[0].ProgramData.R[4]"
const callHeader = "Resource[0].FunctionBlocks.FunctionBlockWrapper[0].FunctionBlock.POUHeader"

func callDocument(t testing.TB, name string) *cxp.Document {
	t.Helper()
	data, err := os.ReadFile("../cxp/testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := cxp.Read(data)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func callField(t testing.TB, doc *cxp.Document, path string) *cxp.Field {
	t.Helper()
	fields := &doc.Fields
	var selected *cxp.Field
	for _, part := range strings.Split(path, ".") {
		selected = nil
		for i := range *fields {
			if (*fields)[i].Name == part {
				selected = &(*fields)[i]
				break
			}
		}
		if selected == nil {
			t.Fatal("missing field", path)
		}
		fields = &selected.Fields
	}
	return selected
}

func checkCallSpans(t testing.TB, body *compiler.Body) {
	t.Helper()
	coverage := make([]int, len(body.Words))
	mark := func(offset, length int) {
		if offset < 0 || length < 1 || offset > len(coverage)-length {
			t.Fatal("invalid FB span")
		}
		for i := offset; i < offset+length; i++ {
			coverage[i]++
		}
	}
	for _, span := range body.Spans {
		if span.Rung != callRung || span.Line < 1 {
			t.Fatal("lost source location")
		}
		mark(span.OffsetWords, span.LengthWords)
	}
	for _, span := range body.Generated {
		if span.Section != symbolProgram+".Sections.Sec[0]" || !strings.HasPrefix(span.Kind, "fb-invocation-") {
			t.Fatal("lost generated provenance")
		}
		mark(span.OffsetWords, span.LengthWords)
	}
	for _, n := range coverage {
		if n != 1 {
			t.Fatal("missing or overlapping code provenance")
		}
	}
}

func TestCompileOriginalFBCallBodies(t *testing.T) {
	for _, spec := range []struct{ source, image, model string }{
		{"cj1m.cxp", "cj1m-input-bit.n1", codec.CJ1MCPU21}, {"cj1h.cxp", "cj1h-baseline.n1", codec.CJ1HCPU65},
	} {
		doc := callDocument(t, spec.source)
		before := bytes.Clone(doc.Source)
		body, err := compiler.CompileCXPFBCallBody(codec.Profile{Model: spec.model}, doc, symbolProgram, callRung)
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
		if !reflect.DeepEqual(body.Words, image.Tasks[0].Code[25:52]) || len(body.Words) != 27 || len(body.Bytes()) != 108 || body.HasEnd || len(body.Spans) != 6 || len(body.Generated) != 3 || len(body.Bindings) != 3 {
			t.Fatal("incomplete source FB-call body", body)
		}
		checkCallSpans(t, body)
		if !bytes.Equal(before, doc.Source) {
			t.Fatal("modified source")
		}
		if _, err := compiler.CompileCXPProgramBody(profile(), doc, symbolProgram); err == nil {
			t.Fatal("standalone lowering enabled unqualified whole-program graph compilation")
		}
	}
}

func TestFBCallCompilationRejectsUnqualifiedSource(t *testing.T) {
	for _, test := range []struct {
		path string
		edit func(*cxp.Field)
	}{
		{callHeader + ".PouVarList", func(f *cxp.Field) { f.Raw = strings.Replace(f.Raw, "1,PV,1,", "1,PV,99,", 1) }},
		{callHeader + ".PouVarList", func(f *cxp.Field) { f.Raw = strings.Replace(f.Raw, "1,PV,1,", "1,PV,3,", 1) }},
		{"Resource[0].GlobalVariables.POUInstances.Instance[0].InstanceNum", func(f *cxp.Field) { f.Value = "2048" }},
		{strings.TrimSuffix(callHeader, ".POUHeader") + ".TaskID", func(f *cxp.Field) { f.Value = "2048" }},
		{"Resource[0].GlobalVariables.VariableList", func(f *cxp.Field) { f.Raw = strings.Replace(f.Raw, "ON,W1.00,BOOL", "ON,W1.00,WORD", 1) }},
		{symbolProgram + ".LocalVariables.VariableList", func(f *cxp.Field) {
			f.Raw = strings.Replace(f.Raw, "BLINK_1_OUT,W1.01,BOOL", "BLINK_1_OUT,W1.01,WORD", 1)
		}},
		{callRung + ".FBversion", func(f *cxp.Field) { f.Value = `"old"` }},
	} {
		doc := callDocument(t, "cj1m.cxp")
		test.edit(callField(t, doc, test.path))
		if body, err := compiler.CompileCXPFBCallBody(profile(), doc, symbolProgram, callRung); err == nil || body != nil {
			t.Fatal("partial or unqualified FB body", test.path)
		}
	}
	for _, p := range []codec.Profile{{}, {Model: codec.CJ1MCPU21, UnitVersion: "1.0"}, {Model: codec.CJ1HCPU65, TimerCounterMode: "unknown"}} {
		if body, err := compiler.CompileCXPFBCallBody(p, callDocument(t, "cj1m.cxp"), symbolProgram, callRung); err == nil || body != nil {
			t.Fatal("unqualified profile produced FB body")
		}
	}
	if body, err := compiler.CompileCXPFBCallBody(profile(), nil, symbolProgram, callRung); err == nil || body != nil {
		t.Fatal("nil source accepted")
	}
}

func FuzzCXPFBCallBody(f *testing.F) {
	data, err := os.ReadFile("../cxp/testdata/cj1m.cxp")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(uint8(0), "0")
	f.Add(uint8(1), `"old"`)
	f.Add(uint8(2), "1,PV,99,,0,0,0,;")
	f.Fuzz(func(t *testing.T, which uint8, value string) {
		if len(value) > 8192 {
			t.Skip()
		}
		doc, err := cxp.Read(data)
		if err != nil {
			t.Fatal(err)
		}
		paths := []string{"Resource[0].GlobalVariables.POUInstances.Instance[0].InstanceNum", callRung + ".FBversion", callHeader + ".PouVarList"}
		field := callField(t, doc, paths[int(which)%3])
		if int(which)%3 == 2 {
			field.Raw = value
		} else {
			field.Value = value
		}
		body, err := compiler.CompileCXPFBCallBody(profile(), doc, symbolProgram, callRung)
		if err != nil {
			if body != nil {
				t.Fatal("partial code on failure")
			}
			return
		}
		if len(body.Words) != 27 || body.HasEnd {
			t.Fatal("invalid fragment shape")
		}
		checkCallSpans(t, body)
	})
}
