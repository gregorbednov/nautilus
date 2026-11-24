package codec_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/il"
)

func TestMOVAgainstLibraryOracle(t *testing.T) {
	encoded, rejected, unsupported := 0, 0, 0
	for _, row := range loadLibraryResults(t, "mov-manifest.json", 370) {
		source := row.Mnemonic + " " + strings.Join(row.Operands, " ")
		instruction, err := il.Parse(source)
		if err != nil {
			t.Fatal(err)
		}
		profile := codec.Profile{Model: row.Device + " " + row.CPU}
		words, err := codec.EncodeInstruction(profile, instruction)
		// The low-level library ignores incorrect labels and some modifiers.
		// Indirect source addressing remains explicitly outside this codec.
		outside := instruction.Modifier != "" || (instruction.Function != "" && instruction.Function != "021" && instruction.Function != "21") || strings.HasPrefix(instruction.Operands[0], "*") || strings.HasPrefix(instruction.Operands[0], "@")
		if outside || !row.Accepted {
			if err == nil || len(words) != 0 {
				t.Fatalf("emitted unsupported/invalid %s", source)
			}
			if outside {
				unsupported++
			} else {
				rejected++
			}
			continue
		}
		if err != nil || !reflect.DeepEqual(words, row.Words) {
			t.Fatalf("%s %s: %x/%v, oracle %x", profile.Model, source, words, err, row.Words)
		}
		// A following instruction must remain unconsumed.
		input := append(append([]uint32{}, row.Words...), 0xbb800001)
		decoded, consumed, err := codec.DecodeInstruction(profile, input)
		if err != nil || consumed != 3 {
			t.Fatalf("decode %s: %d/%v", source, consumed, err)
		}
		roundTrip, err := codec.EncodeInstruction(profile, decoded)
		if err != nil || !reflect.DeepEqual(roundTrip, row.Words) {
			t.Fatalf("changed oracle code %s", source)
		}
		encoded++
	}
	if encoded < 250 || rejected < 60 || unsupported != 10 {
		t.Fatalf("unexpected coverage %d/%d/%d", encoded, rejected, unsupported)
	}
	t.Logf("MOV oracle: encoded=%d rejected=%d outside current codec=%d", encoded, rejected, unsupported)
}

func TestMOVRejectsMalformedContainers(t *testing.T) {
	profile := codec.Profile{Model: codec.CJ1MCPU21}
	for _, words := range [][]uint32{
		{0xb2400140}, {0xb2400140, 0x61400001},
		{0xb2400140, 0x61410001, 0x54010000}, // Reserved literal tag.
		{0xb2400140, 0x60810000, 0x54010000}, // Unqualified indirect source.
		{0xb2400140, 0x6000da00, 0x54010000}, // Address gap.
		{0xb2400140, 0x61400001, 0x5400b840}, // Read-only A0 destination.
		{0xb2400140, 0x61400001, 0x54018000}, // D32768.
		{0xb2400140, 0x61400001, 0x60010000}, // Wrong destination tag.
	} {
		if _, consumed, err := codec.DecodeInstruction(profile, words); err == nil || consumed != 0 {
			t.Fatalf("consumed malformed MOV %x", words)
		}
	}
	for _, source := range []string{"MOV", "MOV D0", "MOV #1 D0 D1", "MOV(999) #1 D0", "%MOV #1 D0", "MOV &-1 D0", "MOV &+1 D0", "MOV #10000 D0", "MOV &65536 D0", "MOV D0.00 D1", "MOV D0 A447", "MOV *D0 D1", "MOV D0 @D1"} {
		instruction, err := il.Parse(source)
		if err != nil {
			t.Fatal(err)
		}
		if words, err := codec.EncodeInstruction(profile, instruction); err == nil || len(words) != 0 {
			t.Fatalf("emitted %s", source)
		}
	}
}

func TestWordOperandSemantics(t *testing.T) {
	address, err := codec.ParseWordAddress("T4095")
	if err != nil || address.Area != codec.Timer || address.Word != 4095 || address.String() != "T4095" {
		t.Fatal(address, err)
	}
	if _, err := codec.ParseWordAddress("T0.00"); err == nil {
		t.Fatal("accepted completion bit as word")
	}
	if _, err := codec.ParseWordAddress((codec.WordAddress{}).String()); err == nil {
		t.Fatal("invalid address became valid")
	}
	for _, text := range []string{"#FFFF", "&65535", "#00001", "&00001"} {
		literal, err := codec.ParseLiteral16(text)
		if err != nil {
			t.Fatal(err)
		}
		canonical, err := codec.ParseLiteral16(literal.String())
		if err != nil || canonical != literal {
			t.Fatal("literal changed radix or value")
		}
	}
}
