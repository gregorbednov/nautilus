package compiler_test

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/um"
)

func TestBOOLControlImageMatchesUnit30F7(t *testing.T) {
	for _, tc := range []struct{ family, model string }{{"cj1m", codec.CJ1MCPU21}, {"cj1h", codec.CJ1HCPU65}} {
		t.Run(tc.model, func(t *testing.T) { checkBOOLControlImage(t, tc.family, tc.model) })
	}
}

func checkBOOLControlImage(t *testing.T, family, model string) {
	t.Helper()
	read := func(name string) []byte {
		t.Helper()
		data, err := os.ReadFile("testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	doc, err := cxp.Read(read(family + "-bool-control-unit30.cxp"))
	if err != nil {
		t.Fatal(err)
	}
	golden := read(family + "-f7-bool-control-unit30.n1")
	var settings compiler.N1ImageSettings
	if err := json.Unmarshal(read("n1-bool-control-settings.json"), &settings); err != nil {
		t.Fatal(err)
	}
	profile := codec.Profile{Model: model, TimerCounterMode: codec.TimerModeBCD}
	check := func(source *cxp.Document, want []byte) {
		t.Helper()
		image, err := compiler.CompileCXPImage(profile, source, "Resource[0]", settings)
		if err != nil {
			t.Fatal(err)
		}
		if len(want) != 4984 || !bytes.Equal(image.Bytes(), want) {
			t.Fatal("complete image differs")
		}
	}
	check(doc, golden)
	for _, pin := range []struct{ address, operand string }{
		{"W100.01", "\x62\x0d\xe6\x41"},
		{"W100.02", "\x56\x0d\xe6\x42"},
	} {
		if bytes.Count(doc.Source, []byte(pin.address)) != 1 || bytes.Count(golden, []byte(pin.operand)) != 1 {
			t.Fatal("ambiguous pin edit", pin.address)
		}
		changed, err := cxp.Read(bytes.Replace(doc.Source, []byte(pin.address), []byte("W100.03"), 1))
		if err != nil {
			t.Fatal(err)
		}
		check(changed, bytes.Replace(golden, []byte(pin.operand), []byte(pin.operand[:3]+"\x43"), 1))
	}
}

func imageSettings(t *testing.T) compiler.N1ImageSettings {
	t.Helper()
	raw, err := os.ReadFile("testdata/n1-example-settings.json")
	if err != nil {
		t.Fatal(err)
	}
	var settings compiler.N1ImageSettings
	if err = json.Unmarshal(raw, &settings); err != nil {
		t.Fatal(err)
	}
	return settings
}

func TestCompleteN1ImageMatchesBothCapturedFiles(t *testing.T) {
	for _, spec := range []struct{ source, image, model, first string }{{"cj1m.cxp", "cj1m-input-bit.n1", codec.CJ1MCPU21, "LD W0.15\nOUT W0.01"}, {"cj1h.cxp", "cj1h-baseline.n1", codec.CJ1HCPU65, "LD W0.00\nOUT W0.01"}} {
		doc := callDocument(t, spec.source)
		if err := doc.SetRung(symbolProgram+".Sections.Sec[0].ProgramData.R[0]", spec.first); err != nil {
			t.Fatal(err)
		}
		before, err := doc.MarshalCXT()
		if err != nil {
			t.Fatal(err)
		}
		settings := imageSettings(t)
		settingsBefore, _ := json.Marshal(settings)
		image, err := compiler.CompileCXPImage(codec.Profile{Model: spec.model, TimerCounterMode: codec.TimerModeBCD}, doc, "Resource[0]", settings)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile("../um/testdata/" + spec.image)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(image.Bytes(), raw) {
			t.Fatal("complete N1 differs", spec.source)
		}
		if image.Summary().ByteLength != 4592 || len(image.HeaderWords) != 42 || image.Tasks[0].OffsetWords != 784 || image.Tasks[1].OffsetWords != 972 {
			t.Fatal("image geometry differs")
		}
		settingsAfter, _ := json.Marshal(settings)
		after, err := doc.MarshalCXT()
		if err != nil || !bytes.Equal(before, after) || !bytes.Equal(settingsBefore, settingsAfter) {
			t.Fatal("source/settings mutated", err)
		}
		copy := image.Bytes()
		copy[0] ^= 255
		report := image.Summary()
		report.Tasks[0].NameASCII = "changed"
		image.HeaderWords[0] = 123
		if !bytes.Equal(image.Bytes(), raw) || image.Summary().Tasks[0].NameASCII == "changed" {
			t.Fatal("mutable generated artifact")
		}
	}
}

func TestCompleteN1TracksUnchangedSourceAndStartup(t *testing.T) {
	doc := callDocument(t, "cj1m.cxp")
	image, err := compiler.CompileCXPImage(profile(), doc, "Resource[0]", imageSettings(t))
	if err != nil {
		t.Fatal(err)
	}
	if image.Summary().ByteLength != 4604 || image.Tasks[1].OffsetWords != 975 {
		t.Fatal("original source extent lost")
	}
	callField(t, doc, symbolProgram+".OpStart").Value = "0"
	changed, err := compiler.CompileCXPImage(profile(), doc, "Resource[0]", imageSettings(t))
	if err != nil {
		t.Fatal(err)
	}
	a, b := image.Bytes(), changed.Bytes()
	a[(42+256)*4+2] &^= 0x40
	if !bytes.Equal(a, b) {
		t.Fatal("startup changed unrelated bytes")
	}
	parsed, err := um.ParseN1(changed.Bytes())
	if err != nil || len(parsed.Tasks) != 2 {
		t.Fatal("invalid linked file", err)
	}
}

func TestCompleteN1RejectsInconsistentSettingsAtomically(t *testing.T) {
	for _, change := range []func(*compiler.N1ImageSettings){
		func(s *compiler.N1ImageSettings) { s.Options.IndependentIRDR = true },
		func(s *compiler.N1ImageSettings) { s.Memory.Stack.Address = "D0" },
		func(s *compiler.N1ImageSettings) { s.Memory.Bases[0].Size = 1 },
		func(s *compiler.N1ImageSettings) { s.Memory.Bases[3].Size = 1 },
		func(s *compiler.N1ImageSettings) { s.Memory.Bases[0].Address = "T3100"; s.Memory.Bases[0].Size = 10 },
		func(s *compiler.N1ImageSettings) { s.NextEnable[0] = 1 },
		func(s *compiler.N1ImageSettings) { s.DefinitionNumbers = map[string]int{"Other": 0} },
	} {
		settings := imageSettings(t)
		change(&settings)
		if image, err := compiler.CompileCXPImage(profile(), callDocument(t, "cj1m.cxp"), "Resource[0]", settings); image != nil || err == nil {
			t.Fatal("accepted inconsistent settings", settings, err)
		}
	}
	for _, scope := range []string{"Resource[1]", symbolProgram} {
		if image, err := compiler.CompileCXPImage(profile(), callDocument(t, "cj1m.cxp"), scope, imageSettings(t)); image != nil || err == nil {
			t.Fatal("bad scope")
		}
	}
	if image, err := compiler.CompileCXPImage(profile(), nil, "Resource[0]", imageSettings(t)); image != nil || err == nil {
		t.Fatal("nil source")
	}
	if image, err := compiler.CompileCXPImage(codec.Profile{}, callDocument(t, "cj1m.cxp"), "Resource[0]", imageSettings(t)); image != nil || err == nil {
		t.Fatal("bad profile")
	}
	// Native directory offsets stay directory-relative after image linking.
	image, err := compiler.CompileCXPImage(profile(), callDocument(t, "cj1m.cxp"), "Resource[0]", imageSettings(t))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(image.ResourceBody.TaskBlock.DirectoryWords[256:258], []uint32{288, 479}) {
		t.Fatal("directory was relocated incorrectly")
	}
}
