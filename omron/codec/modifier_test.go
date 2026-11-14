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

func TestModifiersAgainstLibraryOracle(t *testing.T) {
	encoded, rejected, outside := 0, 0, 0
	for _, row := range loadLibraryResults(t, "modifier-manifest.json", 908) {
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
		allowed := false
		switch instruction.Mnemonic {
		case "LD", "LDNOT", "AND", "ANDNOT", "OR", "ORNOT", "SET", "RSET":
			allowed = true
		}
		// Keep low-level success separate from documented CPU and operand scope.
		if len(instruction.Operands) == 1 && (strings.HasPrefix(instruction.Operands[0], "D") || strings.HasPrefix(instruction.Operands[0], "TR")) {
			allowed = false
		}
		if !allowed || !row.Accepted {
			if err == nil || len(words) != 0 {
				t.Fatalf("emitted unsupported/invalid %s", source)
			}
			if !allowed {
				outside++
			} else {
				rejected++
			}
			continue
		}
		if err != nil || !reflect.DeepEqual(words, row.Words) {
			t.Fatalf("%s %s: got %x/%v, oracle %x", profile.Model, source, words, err, row.Words)
		}
		decoded, count, err := codec.DecodeInstruction(profile, append(append([]uint32{}, row.Words...), 0xbb800001))
		if err != nil || count != len(row.Words) || decoded.Modifier != instruction.Modifier || decoded.Mnemonic != instruction.Mnemonic {
			t.Fatalf("decode %s: %+v %d/%v", source, decoded, count, err)
		}
		again, err := codec.EncodeInstruction(profile, decoded)
		if err != nil || !reflect.DeepEqual(again, row.Words) {
			t.Fatal("modifier round trip changed native containers")
		}
		encoded++
	}
	if encoded != 496 || rejected != 176 || outside != 236 {
		t.Fatalf("unexpected modifier coverage %d/%d/%d", encoded, rejected, outside)
	}
	t.Logf("modifier oracle: encoded=%d rejected=%d outside qualified forms=%d", encoded, rejected, outside)
}

func TestModifiersRejectMalformedContainers(t *testing.T) {
	profile := codec.Profile{Model: codec.CJ1MCPU21}
	for _, words := range [][]uint32{
		{0xe2c5e000}, {0xe2c5e000, 0xcb800000},
		{0xe2c5e000, 0xcb800001, 0xc2125002}, // Unqualified helper bits.
		{0xe2c5e000, 0xcb800000, 0xc2125003}, // Altered suffix.
		{0xe2c80000, 0xcb800000, 0xc2125002}, // CJ2-only DM bit.
		{0xe2d22000, 0xcb800000, 0xc2125002}, // Unqualified differentiated TR.
		{0xfac00000}, {0xfae00000},
		{0xfac00000, 0xde05e000}, // OUT is not a differentiated form.
		{0xfae00000, 0xde25e000}, // OUT NOT is not a differentiated form.
		{0xfac00000, 0xd4480000}, // CJ2-only DM bit SET.
		{0xfac00000, 0xd4438400}, // Read-only A0 destination.
		{0xfae00000, 0xd463e000}, // Timer completion destination.
	} {
		if _, count, err := codec.DecodeInstruction(profile, words); err == nil || count != 0 {
			t.Fatalf("consumed unsupported modifier %x", words)
		}
	}
	for _, source := range []string{"@LD(999) W0.00", "@LD TR0", "%LD D0.00", "@SET T0", "%RSET A0.00", "@OUT W0.00", "%OUTNOT W0.00", "@ANDLD", "%NOT", "@MOV #1 D0", "%MOV #1 D0", "@TIMH(015) 0 #0010"} {
		instruction, err := il.Parse(source)
		if err != nil {
			t.Fatal(err)
		}
		if words, err := codec.EncodeInstruction(profile, instruction); err == nil || len(words) != 0 {
			t.Fatalf("emitted %s", source)
		}
	}
}

func TestConditionUPMatchesCapturedProjectHelpers(t *testing.T) {
	for _, name := range []string{"cj1h-baseline.n1", "cj1m-input-bit.n1"} {
		data, err := os.ReadFile(filepath.Join("..", "um", "testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		image, err := um.ParseN1(data)
		if err != nil {
			t.Fatal(err)
		}
		profile := codec.Profile{Model: codec.CJ1MCPU21}
		instruction, _ := il.Parse("@AND W0.00")
		words, err := codec.EncodeInstruction(profile, instruction)
		if err != nil {
			t.Fatal(err)
		}
		found := 0
		for _, task := range image.Tasks {
			for i := 0; i+3 <= len(task.Code); i++ {
				// Only the complete UP macro is now qualified; its internal
				// field remains unavailable as a generic direct address.
				if task.Code[i] != 0xe2d20000 {
					continue
				}
				triple := task.Code[i : i+3]
				if !reflect.DeepEqual(triple[1:], words[1:]) {
					t.Fatal("project helper/suffix differs from oracle")
				}
				if decoded, count, err := codec.DecodeInstruction(profile, triple); err != nil || count != 3 || decoded.Mnemonic != "UP" || len(decoded.Operands) != 0 {
					t.Fatal("captured UP macro did not decode", err)
				}
				found++
			}
		}
		if found != 2 {
			t.Fatalf("project helper coverage changed: %s: %d", name, found)
		}
	}
}
