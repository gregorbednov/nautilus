package codec_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/il"
	"github.com/joyautomation/nautilus/omron/um"
)

func TestBranchesAgainstLibraryOracle(t *testing.T) {
	encoded, rejected, outside := 0, 0, 0
	for _, row := range loadLibraryResults(t, "branch-manifest.json", 524) {
		source := row.Mnemonic
		if len(row.Operands) > 0 {
			source += " " + strings.Join(row.Operands, " ")
		}
		instruction, err := il.Parse(source)
		if err != nil {
			t.Fatal(err)
		}
		profile := codec.Profile{Model: row.Device + " " + row.CPU}
		words, err := codec.EncodeInstruction(profile, instruction)
		unsupported := instruction.Modifier != "" || instruction.Function == "999" || instruction.Mnemonic == "MPS" || instruction.Mnemonic == "MRD" || instruction.Mnemonic == "MPP"
		if unsupported || !row.Accepted {
			if err == nil || len(words) != 0 {
				t.Fatalf("emitted unsupported/invalid %s", source)
			}
			if unsupported {
				outside++
			} else {
				rejected++
			}
			continue
		}
		if err != nil || !reflect.DeepEqual(words, row.Words) {
			t.Fatalf("%s %s: %x/%v, oracle %x", profile.Model, source, words, err, row.Words)
		}
		decoded, count, err := codec.DecodeInstruction(profile, append(append([]uint32{}, row.Words...), 0xbb800001))
		if err != nil || count != 1 {
			t.Fatalf("decode %s: %d/%v", source, count, err)
		}
		again, err := codec.EncodeInstruction(profile, decoded)
		if err != nil || !reflect.DeepEqual(again, row.Words) {
			t.Fatal("branch round trip changed native word")
		}
		encoded++
	}
	if encoded < 80 || rejected < 350 || outside < 30 {
		t.Fatalf("unexpected branch coverage %d/%d/%d", encoded, rejected, outside)
	}
	t.Logf("branch oracle: encoded=%d rejected=%d outside current forms=%d", encoded, rejected, outside)
}

func TestTemporaryRelayIsSeparateOperandType(t *testing.T) {
	for i := 0; i < 16; i++ {
		want := codec.TemporaryRelay(i)
		relay, err := codec.ParseTemporaryRelay(want.String())
		if err != nil || relay != want {
			t.Fatal("relay index changed", i)
		}
		if _, err := codec.ParseBitAddress(want.String()); err == nil {
			t.Fatal("relay became general memory bit")
		}
		if _, err := codec.ParseWordAddress(want.String()); err == nil {
			t.Fatal("relay became word address")
		}
	}
	for _, text := range []string{"TR", "TR16", "TR0.00", "TR-1", "TR+1", "0", "TR65536", codec.TemporaryRelay(16).String()} {
		if _, err := codec.ParseTemporaryRelay(text); err == nil {
			t.Fatalf("accepted invalid relay %q", text)
		}
	}
	profile := codec.Profile{Model: codec.CJ1MCPU21}
	for _, word := range []uint32{0xe2122010, 0xfa122010, 0xe2322000, 0xe2522000, 0xe2922000, 0xfa322000, 0xf0522000, 0xf0722000, 0xe3200001, 0xe3400001, 0xe3000001, 0xb1000003, 0xbb000002} {
		if _, count, err := codec.DecodeInstruction(profile, []uint32{word}); err == nil || count != 0 {
			t.Fatalf("consumed reserved/invalid branch %x", word)
		}
	}
}

func TestBranchProjectContainerAnchors(t *testing.T) {
	for _, name := range []string{"cj1h-baseline.n1", "cj1m-input-bit.n1"} {
		data, err := os.ReadFile(filepath.Join("..", "um", "testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		image, err := um.ParseN1(data)
		if err != nil {
			t.Fatal(err)
		}
		for _, source := range []string{"ANDLD", "ORLD", "OUT TR0", "OUT TR1", "LD TR0", "LD TR1"} {
			instruction, err := il.Parse(source)
			if err != nil {
				t.Fatal(err)
			}
			words, err := codec.EncodeInstruction(codec.Profile{Model: codec.CJ1MCPU21}, instruction)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, task := range image.Tasks {
				for _, word := range task.Code {
					if word == words[0] {
						found = true
					}
				}
			}
			if !found {
				t.Fatalf("missing branch numeric anchor %s in %s", source, name)
			}
		}
	}
}
