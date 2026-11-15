package um

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/internal/fbcode"
)

func TestFunctionBlockPrefixAndCapturedBuffers(t *testing.T) {
	for _, spec := range []struct{ image, events, hash string }{{"cj1h-baseline.n1", "cj1h-baseline.events.jsonl.gz", "65cfc04aaf2336e6dedd1c595b8580d8c0106de86817c8a7420a3f955df543c9"}, {"cj1m-input-bit.n1", "cj1m-baseline.events.jsonl.gz", "58918c8c7b8e8e2628b0f463e3bb50b2c89a492c05a94a64dbbf582dd184ec23"}} {
		data, err := os.ReadFile("testdata/" + spec.image)
		if err != nil {
			t.Fatal(err)
		}
		image, err := ParseN1(data)
		if err != nil {
			t.Fatal(err)
		}
		fb, err := image.FunctionBlocks()
		if err != nil {
			t.Fatal(err)
		}
		if len(fb.Definitions) != 1 || len(fb.Instances) != 2 || len(fb.InstanceSectionHeader) != 7 || fb.HardwareQualified {
			t.Fatal("wrong FB inventory", fb)
		}
		def := fb.Definitions[0]
		if def.Number != 0 || def.Flags != 0 || def.BodyNumber != 1024 || def.NameASCII != "BLINK" || def.OffsetWords != 334 || def.LengthWords != 46 || def.PortCountsWord != 0x402 || len(def.Code) != 23 {
			t.Fatal("definition fields changed", def)
		}
		for i, instance := range fb.Instances {
			if instance.Number != []uint16{0, 3}[i] || instance.OffsetWords != []uint32{388, 442}[i] || instance.LengthWords != 54 || len(instance.Code) != 44 || !reflect.DeepEqual(instance.Counts, []uint32{2, 0, 0, 3, 0, 0, 0}) {
				t.Fatal("instance fields changed", instance)
			}
		}
		// Exact old runtime program buffers corroborate the complete instance
		// code ranges independently of this prefix parser. Ignore corrupt text
		// mnemonic fields; numeric little-endian buffers are the evidence here.
		events, err := os.ReadFile("../../tools/omron-research/oracle/testdata/evidence/" + spec.events)
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(events)) != spec.hash {
			t.Fatal("runtime capture hash changed")
		}
		reader, err := gzip.NewReader(bytes.NewReader(events))
		if err != nil {
			t.Fatal(err)
		}
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 4096), 1<<20)
		matched := map[int]bool{}
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "CJ1_ORACLE ") {
				continue
			}
			var event struct {
				Kind         string
				ByteLength   int
				ContainerHex string `json:"containerHexLE"`
			}
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "CJ1_ORACLE ")), &event); err != nil {
				t.Fatal(err)
			}
			if event.Kind != "program" || event.ByteLength != 176 {
				continue
			}
			raw, err := hex.DecodeString(event.ContainerHex)
			if err != nil || len(raw) != 176 {
				t.Fatal("invalid captured instance buffer", err)
			}
			words := make([]uint32, 44)
			for i := range words {
				words[i] = binary.LittleEndian.Uint32(raw[i*4:])
			}
			for i, instance := range fb.Instances {
				if reflect.DeepEqual(instance.Code, words) {
					matched[i] = true
				}
			}
		}
		if err := scanner.Err(); err != nil {
			t.Fatal(err)
		}
		reader.Close()
		if len(matched) != 2 {
			t.Fatal("captured buffers did not corroborate both instance code ranges", spec.events)
		}
		// Public inspection fields and returned slices cannot alter the raw image.
		image.DirectoryOffsetWords = 0
		fb.Instances[0].Code[0] = 0
		fb.Definitions[0].Header[0] = 123
		again, err := image.FunctionBlocks()
		if err != nil || again.Instances[0].Code[0] != 0xb040080a || again.Definitions[0].Header[0] != 0 || !bytes.Equal(image.Original(), data) {
			t.Fatal("inspection aliases retained bytes", err)
		}
	}
}

func TestSavedPOUBasesMatchNativeInstances(t *testing.T) {
	data, err := os.ReadFile("../cxp/testdata/cj1m.cxp")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := cxp.Read(data)
	if err != nil {
		t.Fatal(err)
	}
	fields := cxp.Field{Fields: doc.Fields}
	for _, name := range []string{"Resource[0]", "GlobalVariables", "POUInstances"} {
		var found bool
		fields, found = fields.Get(name)
		if !found {
			t.Fatal("missing POU source metadata", name)
		}
	}
	image, err := ParseN1(oracle(t))
	if err != nil {
		t.Fatal(err)
	}
	blocks, err := image.FunctionBlocks()
	if err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"Instance[0]", "Instance[1]"} {
		instance, found := fields.Get(name)
		if !found {
			t.Fatal("missing instance", name)
		}
		for key, want := range map[string]string{"Name": []string{"BLINK_L1", "BLINK_L2"}[i], "POUType": "BLINK", "InstanceNum": []string{"0", "3"}[i], "BR0": []string{"T3072,2", "T3074,2"}[i], "BR3": []string{"H512,3", "H515,3"}[i]} {
			field, found := instance.Get(key)
			if !found || field.Text() != want {
				t.Fatal("source allocation anchor changed", key, field.Text())
			}
		}
		code := blocks.Instances[i].Code
		if code[3] != []uint32{0x5000ec00, 0x5000ec02}[i] || code[13] != []uint32{0x5000da00, 0x5000da03}[i] {
			t.Fatal("saved POU bases differ from native initialization")
		}
	}
	// This is a five-field relocation observation, not a general allocator.
	want := map[int]bool{1: true, 3: true, 13: true, 18: true, 19: true}
	found := map[int]bool{}
	for i, a := range blocks.Instances[0].Code {
		if a != blocks.Instances[1].Code[i] {
			found[i] = true
		}
	}
	if !reflect.DeepEqual(found, want) {
		t.Fatal("instance relocation fields changed", found)
	}
}

func TestInvocationFrameMatchesCapturedCall(t *testing.T) {
	// Body inputs are known initial encoder controls, not yet generated from
	// CXP ports. This qualifies frame assembly, not private-operand lowering.
	body := [][]uint32{
		{0xb2410140, 0x61400100, 0x5517f009},
		{0xb2410140, 0x61400100, 0x5517f009},
		{0xb2410147, 0x620c004e, 0x5717e009},
		{0xb2410806, 0x61440000, 0x65440400, 0x80000000},
		{0xb2410147, 0x6317e001, 0x560de011},
	}
	frame, err := fbcode.AssembleInvocation(8, body, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"cj1h-baseline.n1", "cj1m-input-bit.n1"} {
		data, err := os.ReadFile("testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		image, err := ParseN1(data)
		if err != nil {
			t.Fatal(err)
		}
		if len(frame) != 26 || !reflect.DeepEqual(frame, image.Tasks[0].Code[26:52]) {
			t.Fatal("invocation frame differs from captured call", name)
		}
	}
}

func TestFBWriterStaticProvenance(t *testing.T) {
	for _, spec := range []struct{ file, compressed, decoded, entry, method string }{
		{"fb-writer-ghidra.json.gz", "fa082fa617be0adb3b556f1eaec222b1e20912f79c679474b423f20a0f9260de", "93c66727eb129335a7f91a71d3aff0a3020c46746d6e2484605882aa19126c5b", "121c28ef", "CompileMultipleProgramsAndProduceBinaryFile"},
		{"fb-postencode-ghidra.json.gz", "98ec81ca46c842e63c8bfae0fe989a209c1339048fe10494306793eef7a04b43", "0833532e63ab80aa59023442f449fe942607130ac34e9b62f2ed37ec70bd26d2", "121dea55", "CreateTemporariesAndDoAsicFix"},
	} {
		data, err := os.ReadFile("testdata/" + spec.file)
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != spec.compressed {
			t.Fatal("static evidence hash changed")
		}
		r, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		raw, err := io.ReadAll(io.LimitReader(r, 1<<20))
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(raw)) != spec.decoded {
			t.Fatal("decoded static evidence changed")
		}
		var proof struct {
			Engine, EngineVersion, SourceSHA256 string
			Functions                           []struct {
				Entry, Name, Pseudocode string
				DecompileCompleted      bool
			}
		}
		if err := json.Unmarshal(raw, &proof); err != nil {
			t.Fatal(err)
		}
		if proof.Engine != "ghidra" || proof.EngineVersion != "12.1.4" || proof.SourceSHA256 != "a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141" {
			t.Fatal("unreviewed static provenance")
		}
		found := false
		for _, f := range proof.Functions {
			if f.Entry == spec.entry {
				found = f.DecompileCompleted && strings.Contains(f.Name, spec.method)
				if spec.entry == "121c28ef" {
					found = found && strings.Contains(f.Pseudocode, "0x5c") && strings.Contains(f.Pseudocode, "+ 10")
				}
			}
		}
		if !found {
			t.Fatal("native writer evidence missing")
		}
	}
}

func TestFBICALLChangesBetweenEncodingAndSerialization(t *testing.T) {
	data, err := os.ReadFile("../../tools/omron-research/oracle/testdata/evidence/cj1m-input-bit.events.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != "036497726d2135cfe5f93759055e46c953baaf7d0619bd5f94cb6d4282b975f6" {
		t.Fatal("capture provenance changed")
	}
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	type event struct {
		Kind, Instance string
		Flags          uint32
		Code           string `json:"containerHexLE"`
	}
	encoded := map[string]event{}
	found := 0
	changed := 0
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "CJ1_ORACLE ") {
			continue
		}
		var row event
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "CJ1_ORACLE ")), &row); err != nil {
			t.Fatal(err)
		}
		if row.Kind == "instruction" {
			encoded[row.Instance] = row
		}
		if row.Kind == "serialized-instruction" {
			before, ok := encoded[row.Instance]
			if ok && before.Code != row.Code {
				decode := func(text string) []uint32 {
					raw, err := hex.DecodeString(text)
					if err != nil || len(raw) == 0 || len(raw)%4 != 0 {
						t.Fatal("invalid changed code buffer", err)
					}
					words := make([]uint32, len(raw)/4)
					for i := range words {
						words[i] = binary.LittleEndian.Uint32(raw[i*4:])
					}
					return words
				}
				initial, final := decode(before.Code), decode(row.Code)
				rebuilt, err := fbcode.InvocationPostamble(8, initial, len(initial) == 1 && initial[0] == 0xb000080e)
				if err != nil || !reflect.DeepEqual(rebuilt, final) {
					t.Fatal("postamble does not explain captured transformation", before.Code, row.Code, err)
				}
				changed++
			}
			if ok && before.Code == "060841b2000044610004446500000080" {
				if row.Code != "06087f92000044610004446500000080" || row.Flags != before.Flags|0x80 {
					t.Fatal("FBICALL post-encode control changed")
				}
				found++
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if found != 1 || changed != 9 {
		t.Fatal("missing invocation lifecycle controls", found, changed)
	}
}

func TestFunctionBlockPrefixRejectsAmbiguousLayouts(t *testing.T) {
	for _, test := range []struct {
		name         string
		index, value uint32
	}{
		{"shifted definitions", 0x23, 331}, {"reversed sections", 0x24, 333},
		{"definition count overflow", 330, 0xffffffff}, {"zero definitions", 330, 0}, {"definition reserved", 331, 1},
		{"definition length overflow", 336, 0xffffffff}, {"definition too short", 336, 26}, {"definition gap", 336, 45},
		{"definition entry", 357, 0}, {"definition return", 378, 0}, {"definition reserved record", 353, 1},
		{"instance count overflow", 380, 0xffffffff}, {"instance length overflow", 390, 0xffffffff}, {"instance short", 390, 13},
		{"instance identity mismatch", 388, 1}, {"instance reserved", 389, 1}, {"instance entry", 398, 0}, {"instance return", 440, 0},
		{"duplicate instance", 442, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := oracle(t)
			binary.BigEndian.PutUint32(data[test.index*4:], test.value)
			image, err := ParseN1(data)
			if err != nil {
				t.Fatal("negative control should retain a parseable opaque prefix", err)
			}
			fb, err := image.FunctionBlocks()
			if !errors.Is(err, ErrN1FunctionBlocks) || fb != nil {
				t.Fatal("accepted malformed FB prefix", err)
			}
		})
	}
	var image *N1Image
	if _, err := image.FunctionBlocks(); !errors.Is(err, ErrN1FunctionBlocks) {
		t.Fatal(err)
	}
}

func TestFunctionBlockCodeRemainsOpaque(t *testing.T) {
	data := oracle(t)
	binary.BigEndian.PutUint32(data[363*4:], 0xdeadbeef)
	data[337*4] = 0xc1
	image, err := ParseN1(data)
	if err != nil {
		t.Fatal(err)
	}
	fb, err := image.FunctionBlocks()
	if err != nil {
		t.Fatal(err)
	}
	if fb.Definitions[0].Code[6] != 0xdeadbeef || fb.Definitions[0].NameASCII != "" || !strings.HasPrefix(fb.Definitions[0].NameHex, "c1") || !bytes.Equal(image.Original(), data) {
		t.Fatal("opaque relative instruction/name changed")
	}
}
