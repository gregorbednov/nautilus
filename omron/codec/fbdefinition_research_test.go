package codec_test

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/joyautomation/nautilus/omron/internal/fbcode"
)

func TestDefinitionEnvelopeAgainstNativeCompile(t *testing.T) {
	accepted, rejected := 0, 0
	for _, row := range loadLibraryProbeResults(t, "definition-wrapper-manifest.json", 38, "fb-instruction-compile-v1") {
		if row.Mnemonic == "FBBRET" {
			_, exit, err := fbcode.DefinitionEnvelope(8, 1024)
			if err != nil || !row.Accepted || !reflect.DeepEqual(exit, row.Words) {
				t.Fatal("return differs", exit, err)
			}
			accepted++
			continue
		}
		value, err := strconv.Atoi(row.Operands[0])
		if err != nil {
			if row.Accepted {
				t.Fatal("noncanonical identifier accepted")
			}
			rejected++
			continue
		}
		entry, _, err := fbcode.DefinitionEnvelope(8, value)
		if row.Accepted {
			accepted++
			if err != nil || !reflect.DeepEqual(entry, row.Words) {
				t.Fatal("entry differs", entry, row.Words, err)
			}
		} else {
			rejected++
			if err == nil {
				t.Fatal("rejected identifier emitted", value)
			}
		}
	}
	if accepted != 14 || rejected != 24 {
		t.Fatal("wrapper coverage changed", accepted, rejected)
	}
}

func TestDefinitionPropertyNativeReadbacks(t *testing.T) {
	rows := loadLibraryProbeResults(t, "definition-properties-manifest.json", 16, "fb-definition-properties-v1")
	for _, row := range rows {
		if !row.Accepted || !reflect.DeepEqual(row.Words, []uint32{0xb0400809, 0x65440400}) || row.Properties == nil {
			t.Fatal("incomplete property control")
		}
	}
}
