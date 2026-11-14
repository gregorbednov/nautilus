package codec_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/il"
)

func TestConditionsAgainstLibraryOracle(t *testing.T) {
	for _, mode := range []struct {
		prefix string
		timer  codec.TimerCounterMode
	}{
		{"condition", ""}, {"condition-bcd", codec.TimerModeBCD}, {"condition-binary", codec.TimerModeBinary},
	} {
		encoded, invalid, outside := 0, 0, 0
		for _, row := range loadLibraryResults(t, mode.prefix+"-manifest.json", 66) {
			source := row.Mnemonic
			if len(row.Operands) > 0 {
				source += " " + strings.Join(row.Operands, " ")
			}
			instruction, err := il.Parse(source)
			if err != nil {
				t.Fatal(err)
			}
			profile := codec.Profile{Model: row.Device + " " + row.CPU, TimerCounterMode: mode.timer}
			words, err := codec.EncodeInstruction(profile, instruction)
			allowed := instruction.Modifier == "" && instruction.Function != "999"
			if !allowed || !row.Accepted {
				if err == nil || len(words) != 0 {
					t.Fatalf("emitted %s", source)
				}
				if !allowed {
					outside++
				} else {
					invalid++
				}
				continue
			}
			if err != nil || !reflect.DeepEqual(words, row.Words) {
				t.Fatalf("%s %s: %x/%v, oracle %x", mode.prefix, source, words, err, row.Words)
			}
			decoded, count, err := codec.DecodeInstruction(profile, append(append([]uint32{}, words...), 0xbb800001))
			if err != nil || count != len(words) || decoded.Mnemonic != instruction.Mnemonic || decoded.Modifier != "" || len(decoded.Operands) != 0 {
				t.Fatalf("decode %s: %+v %d/%v", source, decoded, count, err)
			}
			again, err := codec.EncodeInstruction(profile, decoded)
			if err != nil || !reflect.DeepEqual(again, words) {
				t.Fatal("condition round trip changed", err)
			}
			encoded++
		}
		if encoded != 10 || invalid != 20 || outside != 36 {
			t.Fatalf("%s: unexpected coverage %d/%d/%d", mode.prefix, encoded, invalid, outside)
		}
	}
}

func TestConditionsRejectPartialAndAlteredMacros(t *testing.T) {
	profile := codec.Profile{Model: codec.CJ1MCPU21}
	for _, words := range [][]uint32{
		{0xe2d20000, 0xcb800000, 0xc2525002},
		{0xe2d20000, 0xcba00000, 0xc3000000, 0xc2525002},
	} {
		for size := 0; size < len(words); size++ {
			if _, n, err := codec.DecodeInstruction(profile, words[:size]); err == nil || n != 0 {
				t.Fatal("consumed incomplete condition macro", size)
			}
		}
		for index := range words {
			altered := append([]uint32{}, words...)
			altered[index] ^= 1
			if _, n, err := codec.DecodeInstruction(profile, altered); err == nil || n != 0 {
				t.Fatalf("consumed altered condition macro: %x", altered)
			}
		}
	}
	// The exact UP sequence does not qualify other uses of its internal field.
	for _, words := range [][]uint32{
		{0xe2120000}, {0xe2d20000, 0xcb800000, 0xc2125002},
		{0xe2d20000, 0xcba00000, 0xc2525002},
	} {
		if _, n, err := codec.DecodeInstruction(profile, words); err == nil || n != 0 {
			t.Fatal("internal operand escaped qualification", words)
		}
	}
}
