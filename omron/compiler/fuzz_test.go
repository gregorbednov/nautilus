package compiler_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/il"
)

func FuzzCompileRungs(f *testing.F) {
	for _, text := range []string{
		"LD W0.00\nOUT W0.01",
		"LD W0.00\nLD W0.01\nORLD\nOUT W0.02\n\nEND",
		"LD W0.00\nOUT TR0\nAND W0.01\nOUT W0.02\nLD TR0\nOUT W0.03",
		"LD W0.00\nOUT TR0\nAND W0.01\nOUT TR1\nOUT W0.02\nLD TR1\nANDNOT W0.03\nLD TR0\nAND W0.04\nORLD\nOUT W0.05",
		"LD W0.00\nIL\n\nLD W0.01\nIL\n\nILC\nEND",
		"@LD W0.00\n%SET W0.01\nMOV #1 D0\nTIM 0 #0010",
		"LD W0.00\nLD W0.01\nCNT 0 #1",
		"LD W0.00\nUP(521)\nSET W1.00",
		"LD W0.00\nDOWN(522)\nNOT(520)\nOUT W1.00",
		"UP(521)\nOUT W1.00", "LD W0.00\nDOWN(522)",
		"LD TR0\nOUT W0.00", "ANDLD", "END\nNOP", "",
	} {
		f.Add(text)
	}
	f.Fuzz(func(t *testing.T, text string) {
		if len(text) > 4096 {
			return
		}
		var rungs []compiler.Rung
		for i, part := range strings.Split(text, "\n\n") {
			rungs = append(rungs, rung(string(rune('A'+i)), part))
		}
		body, err := compiler.CompileRungs(profile(), rungs)
		if err != nil {
			if body != nil {
				t.Fatal("partial output on failure")
			}
			return
		}
		if len(body.Words) == 0 || body.MaxLogicDepth > compiler.MaxLogicDepth {
			t.Fatal("invalid body bounds")
		}
		position := 0
		for _, span := range body.Spans {
			if span.OffsetWords != position || span.LengthWords < 1 || span.LengthWords > len(body.Words)-position {
				t.Fatal("invalid source span")
			}
			decoded, count, err := codec.DecodeInstruction(profile(), body.Words[position:])
			if err != nil || count != span.LengthWords {
				t.Fatal("compiled instruction cannot be decoded", err)
			}
			words, err := codec.EncodeInstruction(profile(), decoded)
			if err != nil || !reflect.DeepEqual(words, body.Words[position:position+count]) {
				t.Fatal("compiled containers changed on round trip")
			}
			found := false
			for _, network := range rungs {
				if network.Path == span.Rung {
					for _, line := range network.Lines {
						if line.Number == span.Line {
							original, err := il.Parse(line.Text)
							if err != nil {
								t.Fatal(err)
							}
							expected, err := codec.EncodeInstruction(profile(), original)
							if err != nil || !reflect.DeepEqual(expected, words) {
								t.Fatal("source map refers to a different instruction")
							}
							found = true
						}
					}
				}
			}
			if !found {
				t.Fatal("source location is absent")
			}
			position += count
		}
		if position != len(body.Words) || body.HasEnd != (body.Words[len(body.Words)-1] == 0xbb800001) {
			t.Fatal("lost code or END metadata")
		}
	})
}
