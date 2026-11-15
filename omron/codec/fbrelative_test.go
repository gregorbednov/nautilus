package codec_test

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/um"
)

func fbRelativeProfile() codec.Profile {
	return codec.Profile{Model: codec.CJ1MCPU21, TimerCounterMode: codec.TimerModeBCD}
}

func TestFBRelativeDecodeAgainstNative(t *testing.T) {
	// Verify every literal native request against the saved source, including
	// the precisely declared synthetic mutations. Those are decode evidence only.
	var manifest oracleManifest
	metadata, err := os.ReadFile("testdata/fb-decode-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(metadata, &manifest); err != nil {
		t.Fatal(err)
	}
	var cases []inputCase
	if err := json.Unmarshal(loadOracleArtifact(t, manifest.Artifacts["cases"]), &cases); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../um/testdata/cj1h-baseline.n1")
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
	base := blocks.Definitions[0].Code
	if len(cases) != 6 {
		t.Fatal("decode case coverage changed")
	}
	for i, test := range cases {
		if test.SourceSHA256 != sha(data) || test.SourceOffsetWords != int(blocks.Definitions[0].OffsetWords)+len(blocks.Definitions[0].Header) {
			t.Fatal("source identity mismatch")
		}
		want := append([]uint32(nil), base...)
		if i == 0 {
			if test.Mutation != nil {
				t.Fatal("baseline mutated")
			}
		} else {
			mutation := test.Mutation
			if mutation == nil || mutation.Operation != "replace-relative-low12" || mutation.Value != []uint32{15, 16, 2047, 2048, 4095}[i-1] || !reflect.DeepEqual(mutation.Indices, []int{2, 3, 4, 6, 7, 8, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}) {
				t.Fatal("unexpected mutation")
			}
			for _, index := range mutation.Indices {
				want[index] = want[index]&0xfffff000 | mutation.Value
			}
		}
		if !reflect.DeepEqual(test.Code, want) {
			t.Fatal("untracked source code change")
		}
	}
	for _, row := range loadLibraryProbeResults(t, "fb-decode-manifest.json", 12, "fb-native-decode-v1") {
		profile := fbRelativeProfile()
		if row.Device == "CJ1H-H" {
			profile.Model = codec.CJ1HCPU65
		}
		instructions, err := codec.DecodeFBRelativeBody(profile, row.Words[2:21])
		if err != nil {
			t.Fatal(err)
		}
		if len(instructions) != 15 || len(row.Decoded) != 17 || row.Decoded[0].Text != "FBBN 1024" || row.Decoded[16].Text != "FBBRET" {
			t.Fatal("wrong body/wrappers")
		}
		for i, instruction := range instructions {
			text := instruction.Mnemonic
			if instruction.Function != "" {
				text += "(" + instruction.Function + ")"
			}
			for _, operand := range instruction.Operands {
				offset := ""
				if operand.Offset != 0 {
					offset = fmt.Sprint(operand.Offset)
				}
				text += fmt.Sprintf(" %s,BR%d", offset, operand.BaseRegister)
				if operand.Unit != "bit" && operand.Unit != "timer" && operand.Unit != "word" {
					t.Fatal("missing relative unit")
				}
			}
			wantUnits := []string{"bit"}
			if instruction.Mnemonic == "TIMH" {
				wantUnits = []string{"timer", "word"}
			} else if instruction.Operands[0].BaseRegister == 0 {
				wantUnits = []string{"timer"}
			}
			var units []string
			for _, operand := range instruction.Operands {
				units = append(units, operand.Unit)
			}
			if !reflect.DeepEqual(units, wantUnits) {
				t.Fatal("relative unit mismatch", units, wantUnits)
			}
			native := row.Decoded[i+1]
			if text != native.Text || instruction.OffsetWords+2 != native.OffsetWords || instruction.LengthWords != len(native.Words) {
				t.Fatalf("%s %s instruction %d: %+v != %+v", row.Device, row.ID, i, instruction, native)
			}
		}
	}
}

func TestFBRelativeBodyRejectsUnknownFormsAtomically(t *testing.T) {
	for _, words := range [][]uint32{nil, {0}, {0xe21f4000}, {0xe23f0000}, {0xe21f7000}, {0xe21f3000, 0}, {0xbb4000c1}, {0xbb4000c1, 0x513c0000}, {0xbb4000c1, 0x513c1000, 0x653c3001}, {0xbb4000c1, 0x513c0000, 0x653c4001}, {0xe21f3000, 0xbb4000c1}} {
		got, err := codec.DecodeFBRelativeBody(fbRelativeProfile(), words)
		if got != nil || !errors.Is(err, codec.ErrFBRelativeBody) {
			t.Fatal("partial or unqualified decode", got, err)
		}
	}
	for _, profile := range []codec.Profile{{Model: "unknown"}, {Model: codec.CJ1MCPU21, UnitVersion: "4.0"}} {
		if got, err := codec.DecodeFBRelativeBody(profile, []uint32{0xe21f3000}); got != nil || !errors.Is(err, codec.ErrUnqualifiedProfile) {
			t.Fatal(got, err)
		}
	}
	for _, mode := range []codec.TimerCounterMode{"", codec.TimerModeBinary} {
		profile := fbRelativeProfile()
		profile.TimerCounterMode = mode
		if got, err := codec.DecodeFBRelativeBody(profile, []uint32{0xbb4000c1, 0x513c0000, 0x653c3001}); got != nil || !errors.Is(err, codec.ErrFBRelativeBody) {
			t.Fatal("unqualified timer mode", got, err)
		}
	}
	// Decoding does not enable relative text in the ordinary instruction encoder.
	if _, err := codec.ParseBitAddress("2,BR3"); err == nil {
		t.Fatal("relative syntax leaked into physical addresses")
	}
}

func FuzzFBRelativeBody(f *testing.F) {
	f.Add([]byte{0, 0x30, 0x1f, 0xe2})
	f.Add([]byte{0xc1, 0, 0x40, 0xbb, 0, 0, 0x3c, 0x51, 1, 0x30, 0x3c, 0x65})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 || len(data)%4 != 0 {
			t.Skip()
		}
		words := make([]uint32, len(data)/4)
		for i := range words {
			words[i] = binary.LittleEndian.Uint32(data[i*4:])
		}
		got, err := codec.DecodeFBRelativeBody(fbRelativeProfile(), words)
		if err != nil {
			if got != nil {
				t.Fatal("partial result")
			}
			return
		}
		offset := 0
		for _, instruction := range got {
			if instruction.OffsetWords != offset || instruction.LengthWords < 1 || instruction.LengthWords > len(words)-offset || strings.TrimSpace(instruction.Mnemonic) == "" || len(instruction.Operands) == 0 {
				t.Fatal("invalid decoded span")
			}
			offset += instruction.LengthWords
		}
		if offset != len(words) {
			t.Fatal("incomplete decoded body")
		}
	})
}
