package codec_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/il"
)

func TestCMPAgainstLibraryOracle(t *testing.T) {
	encoded, invalid, outside := 0, 0, 0
	for _, row := range loadLibraryResults(t, "compare-manifest.json", 212) {
		source := row.Mnemonic
		if len(row.Operands) > 0 {
			source += " " + strings.Join(row.Operands, " ")
		}
		instruction, err := il.Parse(source)
		if err != nil {
			t.Fatal(err)
		}
		p := codec.Profile{Model: row.Device + " " + row.CPU}
		words, err := codec.EncodeInstruction(p, instruction)
		allowed := instruction.Modifier == "" && instruction.Function != "999"
		for _, operand := range row.Operands {
			if strings.HasPrefix(operand, "*") || strings.HasPrefix(operand, "@") {
				allowed = false
			}
		}
		if !allowed || !row.Accepted {
			if err == nil || len(words) != 0 {
				t.Fatal("emitted unqualified comparison", source)
			}
			if !allowed {
				outside++
			} else {
				invalid++
			}
			continue
		}
		if err != nil || !reflect.DeepEqual(words, row.Words) {
			t.Fatalf("%s %s: %x/%v oracle%x", p.Model, source, words, err, row.Words)
		}
		decoded, n, err := codec.DecodeInstruction(p, append(append([]uint32{}, words...), 0xbb800001))
		if err != nil || n != 3 || decoded.Mnemonic != "CMP" {
			t.Fatal("comparison decode", decoded, n, err)
		}
		again, err := codec.EncodeInstruction(p, decoded)
		if err != nil || !reflect.DeepEqual(again, words) {
			t.Fatal("comparison round trip changed")
		}
		encoded++
	}
	if encoded != 96 || invalid != 84 || outside != 32 {
		t.Fatal("comparison coverage changed", encoded, invalid, outside)
	}
}

func TestCMPRejectsMalformedContainers(t *testing.T) {
	p := codec.Profile{Model: codec.CJ1MCPU21}
	for _, words := range [][]uint32{
		{0xb2400100}, {0xb2400100, 0x6140dead},
		{0xb2400100, 0x6140dead, 0x6140dead}, // Wrong second-operand position.
		{0xb2400100, 0x6140dead, 0x54010000}, // MOV destination is not a CMP source.
		{0xb2400100, 0x6140dead, 0x65410000}, // Unqualified literal flag bits.
		{0xb2400100, 0x6000e000, 0x64018000}, // Unqualified direct address.
		{0xb2400100, 0x60810000, 0x64010000}, // Indirection remains opaque.
	} {
		if _, n, err := codec.DecodeInstruction(p, words); err == nil || n != 0 {
			t.Fatalf("consumed malformed CMP %x", words)
		}
	}
}
