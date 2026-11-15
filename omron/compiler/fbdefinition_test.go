package compiler_test

import (
	"os"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/il"
	"github.com/joyautomation/nautilus/omron/um"
)

// Check graph order/negation independently of unqualified relative addresses.
// This deliberately ignores operand bits and does NOT qualify body compilation.
func TestFBDefinitionOpcodeSkeletonAgainstOriginalN1(t *testing.T) {
	for _, spec := range []struct{ source, image, model string }{
		{"cj1m.cxp", "cj1m-input-bit.n1", codec.CJ1MCPU21},
		{"cj1h.cxp", "cj1h-baseline.n1", codec.CJ1HCPU65},
	} {
		source, err := callDocument(t, spec.source).SourceForFBDefinition("Resource[0].FunctionBlocks.FunctionBlockWrapper[0].FunctionBlock")
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile("../um/testdata/" + spec.image)
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
		if len(blocks.Definitions) != 1 || len(blocks.Definitions[0].Code) != 23 {
			t.Fatal("unexpected definition layout")
		}
		body := blocks.Definitions[0].Code[2:21] // Exclude FBBN and FBBRET.
		offset, count := 0, 0
		for _, rung := range source.Rungs {
			for _, instruction := range rung.Instructions {
				operands := []string{"W0.00"}
				if instruction.Mnemonic == "TIMH" {
					operands = []string{"0", "D0"}
				}
				words, err := codec.EncodeInstruction(codec.Profile{Model: spec.model, TimerCounterMode: codec.TimerModeBCD}, il.Instruction{Mnemonic: instruction.Mnemonic, Operands: operands})
				if err != nil {
					t.Fatal(err)
				}
				if offset+len(words) > len(body) {
					t.Fatal("graph longer than native body")
				}
				mask := uint32(0xffe00000)
				if instruction.Mnemonic == "TIMH" {
					mask = 0xffffffff
				}
				if words[0]&mask != body[offset]&mask {
					t.Fatalf("%s instruction %d (%s): opcode %08x != %08x", spec.model, count, instruction.Mnemonic, words[0], body[offset])
				}
				offset += len(words)
				count++
			}
		}
		if count != 15 || offset != 19 {
			t.Fatalf("incomplete skeleton: %d instructions, %d words", count, offset)
		}
	}
}
