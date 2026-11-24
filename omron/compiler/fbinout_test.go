package compiler_test

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/cxp"
)

func TestFullInOutBOOLImageMatchesF7(t *testing.T) {
	project, err := os.ReadFile("testdata/cj1h-inout-v4.cxp")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := cxp.Read(project)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/cj1h-f7-ladder-inout-v4.n1")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/n1-inout-v4-settings.json")
	if err != nil {
		t.Fatal(err)
	}
	var settings compiler.N1ImageSettings
	if err := json.Unmarshal(raw, &settings); err != nil {
		t.Fatal(err)
	}
	profile := codec.Profile{Model: codec.CJ1HCPU65, TimerCounterMode: codec.TimerModeBCD}
	image, err := compiler.CompileCXPImage(profile, doc, "Resource[0]", settings)
	if err != nil {
		t.Fatal(err)
	}
	if len(want) != 5004 || !bytes.Equal(image.Bytes(), want) {
		t.Fatal("complete N1 differs from F7")
	}
	proof, err := compiler.VerifyCXPImageBuild(profile, project, "auto", "Resource[0]", settings, want)
	if err != nil || proof.Report().InOutBOOLVersion != "4.0" {
		t.Fatal("verified build lost the FB version", err)
	}
	for _, tc := range []struct{ old, new string }{
		{"W100.01", "W100.02"},
		{"W100.01", "H100.01"}, {"W100.01", "W100"}, {"W100.01", "#1"},
		{"FB_INPUT\x11W100.01", "FB_OUTPUT\x11W100.01"},
		{`FBversion:="11/20/26 02:52:10"`, `FBversion:="stale"`},
		{`BR3:="H518,3"`, `BR3:="H518,2"`},
	} {
		t.Run(tc.new, func(t *testing.T) {
			if bytes.Count(doc.Source, []byte(tc.old)) != 1 {
				t.Fatal("ambiguous source edit", tc.old)
			}
			changed, err := cxp.Read(bytes.Replace(doc.Source, []byte(tc.old), []byte(tc.new), 1))
			if err != nil {
				t.Fatal(err)
			}
			got, err := compiler.CompileCXPImage(profile, changed, "Resource[0]", settings)
			if tc.new != "W100.02" {
				if err == nil || got != nil {
					t.Fatal("accepted unsupported source")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			oldOperand := []byte{0x52, 0x0d, 0xe6, 0x41}
			newOperand := []byte{0x52, 0x0d, 0xe6, 0x42}
			if bytes.Count(want, oldOperand) != 1 || !bytes.Equal(got.Bytes(), bytes.Replace(want, oldOperand, newOperand, 1)) {
				t.Fatal("pin edit must change only the address in the caller")
			}
		})
	}
	for _, version := range []string{"", "3.0"} {
		settings.InOutBOOLVersion = version
		if got, err := compiler.CompileCXPImage(profile, doc, "Resource[0]", settings); err == nil || got != nil {
			t.Fatal("accepted In Out BOOL without version 4.0", version)
		}
	}
	ordinary := imageSettings(t)
	ordinary.InOutBOOLVersion = "4.0"
	scalar, err := compiler.CompileCXPImage(profile, callDocument(t, "cj1h.cxp"), "Resource[0]", ordinary)
	if err != nil || scalar.HeaderWords[6] != 0 {
		t.Fatal("scalar-only definitions must leave the In Out flag clear", err)
	}
}

func TestInOutBOOLBodyFromLadder20(t *testing.T) {
	source, err := os.ReadFile("testdata/cj1h-inout-bool-definition.cxt")
	if err != nil {
		t.Fatal(err)
	}
	image, err := os.ReadFile("testdata/cj1h-f7-ladder-inout-v4.n1")
	if err != nil {
		t.Fatal(err)
	}
	pos := bytes.Index(image, []byte("LD_Probe "))
	if pos < 12 {
		t.Fatal("missing native definition")
	}
	record := image[pos-12:]
	end := 4 * binary.BigEndian.Uint32(record[8:])
	want := record[4*(23+2) : end-8] // record header, FBBN and FBBRET
	const path = "Resource[0].FunctionBlocks.FunctionBlockWrapper[0].FunctionBlock"
	profile := codec.Profile{Model: codec.CJ1HCPU65, UnitVersion: "4.0", TimerCounterMode: codec.TimerModeBCD}
	for _, tc := range []struct {
		old, new string
		accepted bool
	}{
		{"SharedBit", "RenamedPin", true},
		{"SharedBit, 8", "SharedBit, 0", false},             // positive contact
		{"SharedBit, 0", "ENO, 0", false},                   // different destination
		{"4,SharedBit", "3,SharedBit", false},               // external
		{"4,SharedBit", "0,SharedBit", false},               // internal
		{"1,1,0,1,1,SharedBit", "1,1,1,1,SharedBit", false}, // v3 cell in 2.0
	} {
		changed := bytes.ReplaceAll(source, []byte(tc.old), []byte(tc.new))
		if bytes.Equal(source, changed) {
			t.Fatal("test input unchanged", tc)
		}
		doc, err := cxp.Read(changed)
		if err != nil {
			t.Fatal(err)
		}
		got, err := compiler.CompileCXPFBDefinitionBody(profile, doc, path)
		if !tc.accepted {
			if err == nil || got != nil {
				t.Fatal("accepted unsupported form", tc)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err := binary.Write(&buf, binary.BigEndian, got.Body.Words); err != nil {
			t.Fatal(err)
		}
		if len(want) != 52 || !bytes.Equal(buf.Bytes(), want) || got.Allocation.Memory[3].SizeBits != 34 {
			t.Fatal("native body or allocation mismatch")
		}
		for _, other := range []codec.Profile{
			{Model: codec.CJ1HCPU65, TimerCounterMode: codec.TimerModeBCD},
			{Model: codec.CJ1HCPU65, UnitVersion: "3.0", TimerCounterMode: codec.TimerModeBCD},
			{Model: codec.CJ1MCPU21, UnitVersion: "4.0", TimerCounterMode: codec.TimerModeBCD},
		} {
			if result, err := compiler.CompileCXPFBDefinitionBody(other, doc, path); err == nil || result != nil {
				t.Fatal("accepted unqualified In Out profile", other)
			}
		}
	}
}
