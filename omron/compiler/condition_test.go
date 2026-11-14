package compiler_test

import (
	"errors"
	"testing"

	"github.com/joyautomation/nautilus/omron/compiler"
)

func TestConditionTransformersRequireInputAndOutput(t *testing.T) {
	for _, instruction := range []string{"UP", "UP(521)", "DOWN", "DOWN(522)", "NOT(520)", "up(521)", "down(522)"} {
		for _, source := range []string{instruction + "\nOUT W1.00", "LD W0.00\n" + instruction, "LD W0.00\nOUT W1.00\n" + instruction} {
			body, err := compiler.CompileRungs(profile(), []compiler.Rung{rung("condition", source)})
			if !errors.Is(err, compiler.ErrStructure) || body != nil {
				t.Fatalf("emitted incomplete %s: %v", source, err)
			}
		}
		body, err := compiler.CompileRungs(profile(), []compiler.Rung{rung("condition", "LD W0.00\n"+instruction+"\nOUT W1.00")})
		if err != nil || body.MaxLogicDepth != 1 || len(body.Spans) != 3 {
			t.Fatal("condition transformation changed depth", body, err)
		}
	}
}
