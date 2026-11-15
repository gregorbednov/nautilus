package compiler_test

import (
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/um"
)

const instanceRoot = "Resource[0].GlobalVariables.POUInstances"

func TestAuditedFBInstancesAgainstBothSavedImages(t *testing.T) {
	for _, spec := range []struct{ source, image, model string }{{"cj1m.cxp", "cj1m-input-bit.n1", codec.CJ1MCPU21}, {"cj1h.cxp", "cj1h-baseline.n1", codec.CJ1HCPU65}} {
		doc := callDocument(t, spec.source)
		original, err := doc.MarshalCXT()
		if err != nil {
			t.Fatal(err)
		}
		result, err := compiler.CompileCXPFBInstances(codec.Profile{Model: spec.model, TimerCounterMode: codec.TimerModeBCD}, doc, "Resource[0]")
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
		if result.Resource != "Resource[0]" || len(result.Instances) != 2 || len(result.Regions) != 4 {
			t.Fatal("incomplete resource", result)
		}
		for i, instance := range result.Instances {
			want := append(append([]uint32(nil), blocks.Instances[i].Header...), blocks.Instances[i].Code...)
			if !reflect.DeepEqual(instance.RecordWords, want) {
				t.Fatal("instance mismatch", i)
			}
		}
		want := []compiler.FBInstanceRegion{
			{Instance: instanceRoot + ".Instance[0]", BaseRegister: 0, Area: "T", StartWord: 3072, EndWord: 3074},
			{Instance: instanceRoot + ".Instance[0]", BaseRegister: 3, Area: "H", StartWord: 512, EndWord: 515},
			{Instance: instanceRoot + ".Instance[1]", BaseRegister: 0, Area: "T", StartWord: 3074, EndWord: 3076},
			{Instance: instanceRoot + ".Instance[1]", BaseRegister: 3, Area: "H", StartWord: 515, EndWord: 518},
		}
		if !reflect.DeepEqual(result.Regions, want) {
			t.Fatal("wrong range units", result.Regions)
		}
		after, err := doc.MarshalCXT()
		if err != nil || !reflect.DeepEqual(original, after) {
			t.Fatal("source changed", err)
		}
	}
}

func TestFBInstanceAuditRejectsOverlapsAtomically(t *testing.T) {
	for _, edit := range []struct{ field, value string }{{"BR0", `"T3073,2"`}, {"BR3", `"H514,3"`}, {"BR0", `"T3072,2"`}, {"BR3", `"H512,3"`}} {
		doc := callDocument(t, "cj1m.cxp")
		callField(t, doc, instanceRoot+".Instance[1]."+edit.field).Value = edit.value
		result, err := compiler.CompileCXPFBInstances(profile(), doc, "Resource[0]")
		if result != nil || err == nil || !strings.Contains(err.Error(), "overlaps") {
			t.Fatal("overlap admitted or misdiagnosed", edit, result, err)
		}
	}
	// Memory order need not be source order. Adjacent and gapped intervals pass.
	for _, edit := range [][2]string{{`"T3076,2"`, `"H518,3"`}, {`"T4000,2"`, `"H1300,3"`}} {
		doc := callDocument(t, "cj1m.cxp")
		callField(t, doc, instanceRoot+".Instance[0].BR0").Value = edit[0]
		callField(t, doc, instanceRoot+".Instance[0].BR3").Value = edit[1]
		result, err := compiler.CompileCXPFBInstances(profile(), doc, "Resource[0]")
		if err != nil || len(result.Instances) != 2 || result.Instances[0].Source.Number != 0 {
			t.Fatal("valid placement rejected or reordered", err)
		}
	}
}

func TestFBInstanceAuditRejectsSymbolAndLocalDisagreement(t *testing.T) {
	for _, edit := range [][2]string{
		{`BLINK_L1," [Auto]",FUNCTION BLOCK,BLINK,A,`, `BLINK_L1," [Auto]",FUNCTION BLOCK,OTHER,A,`},
		{`BLINK_L1," [Auto]",FUNCTION BLOCK,BLINK,A,`, `BLINK_L1,H512,FUNCTION BLOCK,BLINK,A,`},
		{`BLINK_L1," [Auto]",FUNCTION BLOCK,BLINK,A,`, `BLINK_L1," [Auto]",BOOL,BLINK,A,`},
		{`BLINK_L1," [Auto]",FUNCTION BLOCK,BLINK,A,`, `BLINK_L1," [Auto]",FUNCTION BLOCK,BLINK,,`},
		{`BLINK_L1," [Auto]",FUNCTION BLOCK,BLINK,A,`, `blink_l1," [Auto]",FUNCTION BLOCK,BLINK,A,`},
	} {
		doc := callDocument(t, "cj1m.cxp")
		list := callField(t, doc, "Resource[0].GlobalVariables.VariableList")
		changed := strings.ReplaceAll(list.Raw, edit[0], edit[1])
		if changed == list.Raw {
			t.Fatal("mutation failed")
		}
		list.Raw = changed
		if result, err := compiler.CompileCXPFBInstances(profile(), doc, "Resource[0]"); result != nil || err == nil {
			t.Fatal("inconsistent symbol admitted", edit)
		}
	}
	for _, edit := range []struct{ scope, row string }{
		{"Resource[0].GlobalVariables", `Orphan," [Auto]",FUNCTION BLOCK,BLINK,A,;`},
		{"Resource[0].GlobalVariables", `blink_l1,W0.00,BOOL,,,;`},
		{"Resource[0].Programs.Program[0].LocalVariables", `LocalFB," [Auto]",FUNCTION BLOCK,BLINK,A,;`},
		{"Resource[0].Programs.Program[0].LocalVariables", `BLINK_L1,W0.00,BOOL,,,;`},
		{"Resource[0].Programs.Program[0].LocalVariables", `blink_l1,W0.00,BOOL,,,;`},
	} {
		doc := callDocument(t, "cj1m.cxp")
		callField(t, doc, edit.scope+".VariableList").Raw += "\n" + edit.row
		count := callField(t, doc, edit.scope+".VariableCount")
		n, _ := strconv.Atoi(count.Value)
		count.Value = strconv.Itoa(n + 1)
		if result, err := compiler.CompileCXPFBInstances(profile(), doc, "Resource[0]"); result != nil || err == nil {
			t.Fatal("orphan/local symbol admitted", edit)
		}
	}
	doc := callDocument(t, "cj1m.cxp")
	local := callField(t, doc, "Resource[0].Programs.Program[0].LocalVariables.POUInstances")
	local.Fields = append(local.Fields, *callField(t, doc, instanceRoot+".Instance[0]"))
	callField(t, doc, "Resource[0].Programs.Program[0].LocalVariables.POUInstances.NumInstances").Value = "1"
	if result, err := compiler.CompileCXPFBInstances(profile(), doc, "Resource[0]"); result != nil || err == nil {
		t.Fatal("local record omitted")
	}
}

func TestFBInstanceAuditScopesAndEmptyResource(t *testing.T) {
	var absent *cxp.Document
	if result, err := compiler.CompileCXPFBInstances(profile(), absent, "Resource[0]"); result != nil || err == nil {
		t.Fatal("nil document admitted")
	}
	for _, path := range []string{"", "Resource[0].Programs.Program[0]", "Resource[1]"} {
		if result, err := compiler.CompileCXPFBInstances(profile(), callDocument(t, "cj1m.cxp"), path); result != nil || err == nil {
			t.Fatal("invalid resource admitted", path)
		}
	}
	doc := callDocument(t, "cj1m.cxp")
	instances := callField(t, doc, instanceRoot)
	for _, field := range instances.Fields {
		if field.Name == "NumInstances" {
			instances.Fields = []cxp.Field{field}
			break
		}
	}
	callField(t, doc, instanceRoot+".NumInstances").Value = "0"
	list := callField(t, doc, "Resource[0].GlobalVariables.VariableList")
	var retained []string
	for _, line := range strings.Split(list.Raw, "\n") {
		if !strings.Contains(line, "FUNCTION BLOCK") {
			retained = append(retained, line)
		}
	}
	list.Raw = strings.Join(retained, "\n")
	callField(t, doc, "Resource[0].GlobalVariables.VariableCount").Value = "14"
	result, err := compiler.CompileCXPFBInstances(profile(), doc, "Resource[0]")
	if err != nil || result == nil || len(result.Instances) != 0 || len(result.Regions) != 0 {
		t.Fatal("empty instance set rejected", result, err)
	}
	for _, p := range []codec.Profile{{Model: "unknown", TimerCounterMode: codec.TimerModeBCD}, {Model: codec.CJ1MCPU21}, {Model: codec.CJ1MCPU21, TimerCounterMode: codec.TimerModeBinary}} {
		if result, err := compiler.CompileCXPFBInstances(p, doc, "Resource[0]"); result != nil || err == nil {
			t.Fatal("empty set bypassed profile validation", p)
		}
	}
}

func TestFBInstanceAuditRejectsInvalidLaterRecords(t *testing.T) {
	for _, edit := range []struct{ path, value string }{
		{instanceRoot + ".Instance[1].MemoryAllocated", "false"},
		{instanceRoot + ".Instance[1].InstanceNum", "0"},
		{instanceRoot + ".Instance[1].BR3", `"H1407,3"`},
		{instanceRoot + ".NumInstances", "1"},
		{"Resource[0].Programs.ProgramCount", "1"},
	} {
		doc := callDocument(t, "cj1m.cxp")
		callField(t, doc, edit.path).Value = edit.value
		if result, err := compiler.CompileCXPFBInstances(profile(), doc, "Resource[0]"); result != nil || err == nil {
			t.Fatal("invalid later record produced a partial set", edit)
		}
	}
}

func TestFBInstanceAuditRejectsCaseAmbiguousDefinitions(t *testing.T) {
	doc := callDocument(t, "cj1m.cxp")
	other := callDocument(t, "cj1m.cxp")
	callField(t, other, "Resource[0].FunctionBlocks.FunctionBlockWrapper[0].FunctionBlock.Name").Value = `"blink"`
	wrapper := *callField(t, other, "Resource[0].FunctionBlocks.FunctionBlockWrapper[0]")
	wrapper.Name = "FunctionBlockWrapper[1]"
	blocks := callField(t, doc, "Resource[0].FunctionBlocks")
	blocks.Fields = append(blocks.Fields, wrapper)
	callField(t, doc, "Resource[0].FunctionBlocks.FunctionBlockCount").Value = "2"
	if result, err := compiler.CompileCXPFBInstances(profile(), doc, "Resource[0]"); result != nil || err == nil {
		t.Fatal("case-ambiguous definition admitted")
	}
}
