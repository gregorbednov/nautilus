package codec_test

import (
	"errors"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/um"
)

func TestFBRelativeEncodingAgainstNativeCompile(t *testing.T) {
	supported, excluded := 0, 0
	for _, row := range loadLibraryProbeResults(t, "br-compile-manifest.json", 292, "fb-instruction-compile-v1") {
		instruction := codec.FBRelativeInstruction{Mnemonic: row.Mnemonic}
		if instruction.Mnemonic == "TIMH(015)" {
			instruction.Mnemonic = "TIMH"
			instruction.Function = "015"
		}
		canonical := true
		for i, source := range row.Operands {
			parts := strings.Split(strings.ToUpper(source), ",BR")
			if len(parts) != 2 {
				canonical = false
				break
			}
			if parts[0] == "" {
				parts[0] = "0"
			}
			offset, e1 := strconv.Atoi(parts[0])
			bank, e2 := strconv.Atoi(parts[1])
			if e1 != nil || e2 != nil {
				canonical = false
				break
			}
			unit := "bit"
			if instruction.Mnemonic == "TIMH" {
				unit = "timer"
				if i == 1 {
					unit = "word"
				}
			} else if bank == 0 {
				unit = "timer"
			}
			instruction.Operands = append(instruction.Operands, codec.FBRelativeOperand{BaseRegister: bank, Offset: offset, Unit: unit})
		}
		if !canonical {
			if row.Accepted {
				t.Fatal("unclassified accepted input", row.ID)
			}
			excluded++
			continue
		}
		profile := fbRelativeProfile()
		if row.Device == "CJ1H-H" {
			profile.Model = codec.CJ1HCPU65
		}
		words, err := codec.EncodeFBRelativeInstruction(profile, instruction)
		inScope := true
		for i, operand := range instruction.Operands {
			if operand.Offset < -2048 || operand.Offset > 2047 {
				inScope = false
			}
			if instruction.Mnemonic == "TIMH" {
				if (i == 0 && operand.BaseRegister != 0) || (i == 1 && operand.BaseRegister != 3) {
					inScope = false
				}
			} else if operand.BaseRegister != 3 && !(instruction.Mnemonic == "LD" && operand.BaseRegister == 0) {
				inScope = false
			}
		}
		if !inScope {
			excluded++
			if words != nil || !errors.Is(err, codec.ErrFBRelativeBody) {
				t.Fatal("unqualified extended/bank form admitted", row.ID, words, err)
			}
			continue
		}
		supported++
		if err != nil || !row.Accepted || !reflect.DeepEqual(words, row.Words) {
			t.Fatal("relative instruction differs from real Compile", row.Device, row.ID, words, row.Words, err)
		}
	}
	if supported != 218 || excluded != 74 {
		t.Fatal("native compile coverage changed", supported, excluded)
	}
}

func TestFBRelativeBodyAssemblyMatchesBothSavedBodies(t *testing.T) {
	for _, spec := range []struct{ file, model string }{{"cj1m-input-bit.n1", codec.CJ1MCPU21}, {"cj1h-baseline.n1", codec.CJ1HCPU65}} {
		data, err := os.ReadFile("../um/testdata/" + spec.file)
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
		body := blocks.Definitions[0].Code[2:21]
		profile := fbRelativeProfile()
		profile.Model = spec.model
		instructions, err := codec.DecodeFBRelativeBody(profile, body)
		if err != nil {
			t.Fatal(err)
		}
		words, err := codec.EncodeFBRelativeBody(profile, instructions)
		if err != nil || !reflect.DeepEqual(words, body) || len(words) != 19 {
			t.Fatal("body reconstruction differs", words, err)
		}
		instructions[1].Operands[0].BaseRegister = 7
		if partial, err := codec.EncodeFBRelativeBody(profile, instructions); partial != nil || !errors.Is(err, codec.ErrFBRelativeBody) {
			t.Fatal("partial body returned", partial, err)
		}
	}
}

func TestFBRelativeEncoderRejectsWrongOperandRoles(t *testing.T) {
	for _, instruction := range []codec.FBRelativeInstruction{
		{Mnemonic: "OR", Operands: []codec.FBRelativeOperand{{BaseRegister: 3, Unit: "bit"}}},
		{Mnemonic: "LD", Function: "000", Operands: []codec.FBRelativeOperand{{BaseRegister: 3, Unit: "bit"}}},
		{Mnemonic: "LD", Operands: []codec.FBRelativeOperand{{BaseRegister: 3, Unit: "word"}}},
		{Mnemonic: "OUT", Operands: []codec.FBRelativeOperand{{BaseRegister: 0, Unit: "timer"}}},
		{Mnemonic: "TIMH", Function: "550", Operands: []codec.FBRelativeOperand{{BaseRegister: 0, Unit: "timer"}, {BaseRegister: 3, Unit: "word"}}},
		{Mnemonic: "TIMH", Operands: []codec.FBRelativeOperand{{BaseRegister: 0, Unit: "word"}, {BaseRegister: 3, Unit: "bit"}}},
		{Mnemonic: "LD"},
	} {
		if words, err := codec.EncodeFBRelativeInstruction(fbRelativeProfile(), instruction); words != nil || !errors.Is(err, codec.ErrFBRelativeBody) {
			t.Fatal("wrong operand roles accepted", words, err)
		}
	}
	for _, mode := range []codec.TimerCounterMode{"", codec.TimerModeBinary} {
		profile := fbRelativeProfile()
		profile.TimerCounterMode = mode
		if words, err := codec.EncodeFBRelativeInstruction(profile, codec.FBRelativeInstruction{Mnemonic: "TIMH", Operands: []codec.FBRelativeOperand{{BaseRegister: 0, Unit: "timer"}, {BaseRegister: 3, Unit: "word"}}}); words != nil || !errors.Is(err, codec.ErrFBRelativeBody) {
			t.Fatal("unqualified timer mode", words, err)
		}
	}
	if words, err := codec.EncodeFBRelativeBody(fbRelativeProfile(), nil); words != nil || !errors.Is(err, codec.ErrFBRelativeBody) {
		t.Fatal("empty body", words, err)
	}
}
