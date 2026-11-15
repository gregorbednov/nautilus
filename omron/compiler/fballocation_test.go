package compiler_test

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/um"
)

type allocationArtifact struct {
	Path, SHA256, Encoding, DecodedSHA256 string
	Bytes                                 int
}

func allocationAsset(t *testing.T, a allocationArtifact) []byte {
	t.Helper()
	if a.Path == "" || filepath.Base(a.Path) != a.Path || strings.ContainsAny(a.Path, `/\:`) {
		t.Fatal("invalid artifact path")
	}
	data, err := os.ReadFile("testdata/" + a.Path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != a.Bytes || fmt.Sprintf("%x", sha256.Sum256(data)) != a.SHA256 {
		t.Fatal("artifact hash/size mismatch", a.Path)
	}
	if a.Encoding == "gzip" {
		reader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		defer reader.Close()
		data, err = io.ReadAll(io.LimitReader(reader, (16<<20)+1))
		if err != nil || len(data) > 16<<20 || fmt.Sprintf("%x", sha256.Sum256(data)) != a.DecodedSHA256 {
			t.Fatal("decoded artifact mismatch", a.Path, err)
		}
	} else if a.Encoding != "" {
		t.Fatal("unknown encoding")
	}
	return data
}

func TestOfflineFBAllocationAgainstNativePOU(t *testing.T) {
	var manifest struct {
		SchemaVersion                                       int
		Method, PheadSHA256, RequestSHA256, EventIID        string
		OnlineBoolReserve, OnlineOtherReserve               int
		CPUSpecific, FullProgramCompiled, HardwareQualified bool
		Artifacts                                           map[string]allocationArtifact
	}
	data, err := os.ReadFile("testdata/allocation-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	const phead = "463eb4402bb6fa00453a5d7a745d77ced6be454d77aa2a683eb67de7e38ba71c"
	const events = "d9e8bf87-d3ce-4964-aa4a-71aeb47a2d18"
	if manifest.SchemaVersion != 1 || manifest.Method != "com-pou-header-offline-allocation" || manifest.PheadSHA256 != phead || manifest.EventIID != events || manifest.OnlineBoolReserve != 0 || manifest.OnlineOtherReserve != 0 || manifest.CPUSpecific || manifest.FullProgramCompiled || manifest.HardwareQualified || len(manifest.Artifacts) != 3 {
		t.Fatal("unqualified allocation provenance")
	}
	assets := map[string][]byte{}
	for _, key := range []string{"cases", "events", "worker"} {
		assets[key] = allocationAsset(t, manifest.Artifacts[key])
	}
	if fmt.Sprintf("%x", sha256.Sum256(assets["cases"])) != manifest.RequestSHA256 {
		t.Fatal("request hash mismatch")
	}
	type declaration struct {
		Usage, TypeCode int
		Name, Initial   string
	}
	type inputCase struct {
		ID        string
		Variables []declaration
	}
	var cases []inputCase
	if err := json.Unmarshal(assets["cases"], &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) != 40 {
		t.Fatal("native allocation coverage changed")
	}
	var original []declaration
	source, err := callDocument(t, "cj1m.cxp").SourceForFBDefinition("Resource[0].FunctionBlocks.FunctionBlockWrapper[0].FunctionBlock")
	if err != nil {
		t.Fatal(err)
	}
	for _, variable := range source.Variables {
		original = append(original, declaration{variable.Usage, variable.TypeCode, variable.Name, variable.RemainingColumns[2]})
	}
	if cases[0].ID != "blink-original" || !reflect.DeepEqual(cases[0].Variables, original) {
		t.Fatal("native baseline differs from original source")
	}
	lines := strings.Split(strings.TrimSpace(string(assets["events"])), "\n")
	if len(lines) != len(cases)+2 {
		t.Fatal("event lifecycle mismatch")
	}
	for sequence, line := range lines {
		if !strings.HasPrefix(line, "CJ1_ALLOCATION ") {
			t.Fatal("unexpected oracle output")
		}
		var row struct {
			SchemaVersion, Sequence, PointerBytes, CaseCount               int
			Kind, ID, Method, PheadSHA256, RequestSHA256, EventIID, ProgID string
			OnlineBoolReserve, OnlineOtherReserve                          int
			CPUSpecific, FullProgramCompiled, HardwareQualified            bool
			OnlineEditing                                                  *bool
			LoadedPheadSHA256                                              string
			ApiRVAs                                                        map[string]string
			Variables                                                      []declaration
			Bindings                                                       []struct{ Name, Reference string }
			Memory                                                         []struct{ BaseRegister, SizeBits int }
			Packed                                                         []struct{ BaseRegister, BoolWords, OtherWords int }
			EventRequests                                                  []int
		}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "CJ1_ALLOCATION ")), &row); err != nil {
			t.Fatal(err)
		}
		if row.SchemaVersion != 1 || row.Sequence != sequence {
			t.Fatal("event sequence mismatch")
		}
		if sequence == 0 {
			if row.Kind != "begin" || row.PointerBytes != 4 || row.PheadSHA256 != phead || row.RequestSHA256 != manifest.RequestSHA256 || row.Method != manifest.Method || row.EventIID != events || row.ProgID != "Omron.CXP.POUHeadDoc.Doc2" || row.OnlineBoolReserve != 0 || row.OnlineOtherReserve != 0 || row.CPUSpecific || row.FullProgramCompiled || row.HardwareQualified {
				t.Fatal("native configuration mismatch")
			}
			continue
		}
		if sequence == len(lines)-1 {
			if row.Kind != "done" || row.CaseCount != len(cases) {
				t.Fatal("incomplete allocation run")
			}
			continue
		}
		input := cases[sequence-1]
		if row.Kind != "case" || row.ID != input.ID || !reflect.DeepEqual(row.Variables, input.Variables) || row.OnlineEditing == nil || *row.OnlineEditing {
			t.Fatal("input/configuration correlation mismatch")
		}
		var variables []cxp.FBVariable
		if row.LoadedPheadSHA256 != phead || !reflect.DeepEqual(row.ApiRVAs, map[string]string{"getAddressOffset": "6ac0", "getMemorySize": "73e0", "getUsedBOOLAndWORDSize": "eb60"}) {
			t.Fatal("actual COM module/API differs")
		}
		for _, variable := range input.Variables {
			variables = append(variables, cxp.FBVariable{Name: variable.Name, Usage: variable.Usage, TypeCode: variable.TypeCode, RemainingColumns: [4]string{"", "0", variable.Initial, "0"}})
		}
		plan, err := compiler.PlanOfflineFBAllocation(variables)
		if err != nil {
			t.Fatal(row.ID, err)
		}
		if len(plan.Variables) != len(row.Bindings) || len(row.Memory) != 5 || len(row.Packed) != 2 {
			t.Fatal("incomplete allocation")
		}
		for i, binding := range plan.Variables {
			native := row.Bindings[i]
			if binding.Name != native.Name || fmt.Sprintf("%d,BR%d", binding.Reference.Offset, binding.Reference.BaseRegister) != native.Reference {
				t.Fatal(row.ID, "relative allocation differs", binding, native)
			}
		}
		for i, memory := range row.Memory {
			if memory.BaseRegister != i || plan.Memory[i].SizeBits != memory.SizeBits {
				t.Fatal(row.ID, "memory footprint differs", plan.Memory[i], memory)
			}
		}
		for i, packed := range row.Packed {
			if packed.BaseRegister != i+3 || plan.Memory[i+3].BoolWords != packed.BoolWords || plan.Memory[i+3].OtherWords != packed.OtherWords {
				t.Fatal(row.ID, "packing differs")
			}
		}
		for _, base := range row.EventRequests {
			if base != 3 {
				t.Fatal("unexpected reservation request", base)
			}
		}
	}
}

func TestPOUAllocationAPIEvidence(t *testing.T) {
	data := allocationAsset(t, allocationArtifact{Path: "ghidra-phead-allocation-08.json.gz", SHA256: "ab73e69b05ce2bcd59d0b8269fd5f6d5221f2aee28e412f2e10efd520b15b71e", Bytes: 9751, Encoding: "gzip", DecodedSHA256: "6b24c2257eda2a3cceec2532d3f33a868e9aa2000cd45d9604deaead4e1238d3"})
	var proof struct {
		Engine, EngineVersion, SourceSHA256 string
		Functions                           []struct {
			Entry, Pseudocode  string
			DecompileCompleted bool
			Instructions       []struct{ Address, Text string }
		}
	}
	if err := json.Unmarshal(data, &proof); err != nil {
		t.Fatal(err)
	}
	if proof.Engine != "ghidra" || proof.EngineVersion != "12.1.4" || proof.SourceSHA256 != "463eb4402bb6fa00453a5d7a745d77ced6be454d77aa2a683eb67de7e38ba71c" || len(proof.Functions) != 3 {
		t.Fatal("POU static provenance mismatch")
	}
	for i, entry := range []string{"10006ac0", "100073e0", "1000eb60"} {
		function := proof.Functions[i]
		if function.Entry != entry || !function.DecompileCompleted || function.Pseudocode == "" || len(function.Instructions) == 0 {
			t.Fatal("incomplete allocation API evidence")
		}
	}
}

func TestCompileFBDefinitionFromOriginalSource(t *testing.T) {
	const definition = "Resource[0].FunctionBlocks.FunctionBlockWrapper[0].FunctionBlock"
	for _, spec := range []struct{ source, image, model string }{{"cj1m.cxp", "cj1m-input-bit.n1", codec.CJ1MCPU21}, {"cj1h.cxp", "cj1h-baseline.n1", codec.CJ1HCPU65}} {
		doc := callDocument(t, spec.source)
		before := bytes.Clone(doc.Source)
		compiled, err := compiler.CompileCXPFBDefinitionBody(codec.Profile{Model: spec.model, TimerCounterMode: codec.TimerModeBCD}, doc, definition)
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
		body := compiled.Body
		if !reflect.DeepEqual(body.Words, blocks.Definitions[0].Code[2:21]) || len(body.Spans) != 15 || body.HasEnd || len(body.Generated) != 0 || len(body.Words) != 19 || compiled.Allocation.Memory[3].Words != 3 || compiled.Allocation.Memory[0].Words != 2 {
			t.Fatal("source body differs", compiled)
		}
		coverage := 0
		for _, span := range body.Spans {
			if span.Line < 1 || span.OffsetWords != coverage || !strings.HasPrefix(span.Rung, definition+".POUBody.FBProgramData.FBR[") {
				t.Fatal("lost source provenance")
			}
			coverage += span.LengthWords
		}
		if coverage != 19 {
			t.Fatal("incomplete source coverage")
		}
		if !bytes.Equal(before, doc.Source) {
			t.Fatal("source changed")
		}
		// A source declaration permutation must change allocation and generated
		// operands, rather than replaying saved binary/template words.
		list := callField(t, doc, definition+".POUHeader.PouVarList")
		list.Raw = strings.ReplaceAll(list.Raw, "1,PV,1,", "1,TEMP,1,")
		list.Raw = strings.ReplaceAll(list.Raw, "1,PN,1,", "1,PV,1,")
		list.Raw = strings.ReplaceAll(list.Raw, "1,TEMP,1,", "1,PN,1,")
		changed, err := compiler.CompileCXPFBDefinitionBody(codec.Profile{Model: spec.model, TimerCounterMode: codec.TimerModeBCD}, doc, definition)
		if err != nil {
			t.Fatal(err)
		}
		if changed.Body.Words[5] != 0x653c3002 || changed.Body.Words[9] != 0x653c3001 || reflect.DeepEqual(changed.Body.Words, body.Words) {
			t.Fatal("source allocation changes ignored", changed.Body.Words)
		}
		// Crossing the packed BOOL word boundary must move all word operands.
		doc = callDocument(t, spec.source)
		list = callField(t, doc, definition+".POUHeader.PouVarList")
		for i := 0; i < 13; i++ {
			list.Raw += fmt.Sprintf("\n0,Padding%d,0,,0,FALSE,0,;", i)
		}
		callField(t, doc, definition+".POUHeader.PouVarCount").Value = "20"
		changed, err = compiler.CompileCXPFBDefinitionBody(codec.Profile{Model: spec.model, TimerCounterMode: codec.TimerModeBCD}, doc, definition)
		if err != nil || changed.Body.Words[5] != 0x653c3002 || changed.Body.Words[9] != 0x653c3003 || changed.Allocation.Memory[3].BoolWords != 2 || changed.Allocation.Memory[3].Words != 4 {
			t.Fatal("BOOL packing boundary ignored", err)
		}
	}
}

func FuzzOfflineFBAllocation(f *testing.F) {
	f.Add([]byte{0, 1, 1, 2, 7, 0, 13, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		var variables []cxp.FBVariable
		for i := 0; i+1 < len(data); i += 2 {
			variables = append(variables, cxp.FBVariable{Name: fmt.Sprintf("V%d", i), Usage: int(data[i+1] % 4), TypeCode: int(data[i] % 16), RemainingColumns: [4]string{"", "0", "", "0"}})
		}
		plan, err := compiler.PlanOfflineFBAllocation(variables)
		if err != nil {
			if plan != nil {
				t.Fatal("partial allocation")
			}
			return
		}
		seen := map[string]bool{}
		bitWords := map[int]bool{}
		dataWords := map[int]bool{}
		for _, variable := range plan.Variables {
			reference := variable.Reference
			key := fmt.Sprintf("%d/%s/%d", reference.BaseRegister, reference.Unit, reference.Offset)
			if seen[key] || reference.Offset < 0 || reference.Offset > 2047 {
				t.Fatal("duplicate or out-of-range allocation")
			}
			seen[key] = true
			if reference.BaseRegister == 3 {
				if reference.Unit == "bit" {
					bitWords[reference.Offset/16] = true
				} else if reference.Unit == "word" {
					dataWords[reference.Offset] = true
				} else {
					t.Fatal("untyped data allocation")
				}
			}
		}
		for word := range dataWords {
			if bitWords[word] {
				t.Fatal("BOOL and word storage overlap")
			}
		}
	})
}

func TestOfflineFBAllocationRejectsUnqualifiedDeclarations(t *testing.T) {
	for _, variable := range []cxp.FBVariable{
		{Name: "EN", Usage: 0, TypeCode: 0, RemainingColumns: [4]string{"", "0", "", "0"}},
		{Name: "Array", Usage: 0, TypeCode: 0, RemainingColumns: [4]string{"", "2", "", "0"}},
		{Name: "Retained", Usage: 0, TypeCode: 0, RemainingColumns: [4]string{"", "0", "", "1"}},
		{Name: "AT", Usage: 0, TypeCode: 0, RemainingColumns: [4]string{"H0", "0", "", "0"}},
		{Name: "PortTimer", Usage: 1, TypeCode: 13, RemainingColumns: [4]string{"", "0", "", "0"}},
		{Name: "Dint", Usage: 0, TypeCode: 3, RemainingColumns: [4]string{"", "0", "", "0"}},
		{Name: "External", Usage: 4, TypeCode: 0, RemainingColumns: [4]string{"", "0", "", "0"}},
	} {
		if plan, err := compiler.PlanOfflineFBAllocation([]cxp.FBVariable{variable}); plan != nil || !errors.Is(err, compiler.ErrFBAllocation) {
			t.Fatal("unsupported allocation accepted", plan, err)
		}
	}
	doc := callDocument(t, "cj1m.cxp")
	callField(t, doc, "Resource[0].FunctionBlocks.FunctionBlockWrapper[0].FunctionBlock.OnlineReserveMemory.BR[3]").Value = `"1,0"`
	if got, err := compiler.CompileCXPFBDefinitionBody(profile(), doc, "Resource[0].FunctionBlocks.FunctionBlockWrapper[0].FunctionBlock"); got != nil || err == nil {
		t.Fatal("online reservation accepted", got, err)
	}
}
