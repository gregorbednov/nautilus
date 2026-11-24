package codec_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/il"
)

// These are evidence for internal FB primitives, not public source qualification.
// The normal-context emitter differs from the actual FB call/initialization
// buffers. Keep it separate from the instruction codec until that ABI is known.
func TestFBPrimitiveResearchDoesNotQualifyPublicCodec(t *testing.T) {
	for _, spec := range []struct {
		manifest        string
		count, accepted int
	}{{"fb-primitives-manifest.json", 200, 46}, {"fb-identifiers-manifest.json", 36, 8}} {
		accepted := 0
		identityControls := 0
		for _, row := range loadLibraryResults(t, spec.manifest, spec.count) {
			source := row.Mnemonic
			if len(row.Operands) > 0 {
				source += " " + strings.Join(row.Operands, " ")
			}
			instruction, err := il.Parse(source)
			if err != nil {
				t.Fatal(err)
			}
			if words, err := codec.EncodeInstruction(codec.Profile{Model: row.Device + " " + row.CPU}, instruction); err == nil || len(words) != 0 {
				t.Fatal("internal research form escaped public qualification", source)
			}
			if row.Accepted {
				accepted++
			}
			if row.Mnemonic == "FBIN" && reflect.DeepEqual(row.Operands, []string{"#0400"}) {
				// Unlike the public hex-literal parser, this internal form
				// interprets the digit string as decimal400, not hex1024.
				if !row.Accepted || !reflect.DeepEqual(row.Words, []uint32{0xb040080a, 0x65440190}) {
					t.Fatal("internal identifier control changed")
				}
				identityControls++
			}
			if row.Mnemonic == "FBBN" && reflect.DeepEqual(row.Operands, []string{"1024"}) {
				if !row.Accepted || !reflect.DeepEqual(row.Words, []uint32{0xb0400809, 0x65440400}) {
					t.Fatal("body entry control changed")
				}
				identityControls++
			}
		}
		if accepted != spec.accepted || identityControls != 2 {
			t.Fatal("research coverage changed", spec.manifest, accepted, identityControls)
		}
	}
}
