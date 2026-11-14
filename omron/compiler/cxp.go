package compiler

import (
	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/cxp"
)

// CompileCXPRungs compiles explicitly selected textual rungs with native source
// locations. It does not select CPU/tasks, flatten graphical FBs, trust cached
// Parsed instructions or claim that the remaining project is compilable.
func CompileCXPRungs(profile codec.Profile, rungs []cxp.Rung) (*Body, error) {
	if len(rungs) > MaxInstructions {
		return nil, failure(Diagnostic{}, "source-limit", "too many rungs", ErrSourceLimit)
	}
	selected := make([]Rung, 0, len(rungs))
	count := 0
	for _, rung := range rungs {
		if len(rung.Graphic) != 0 {
			return nil, failure(Diagnostic{Rung: rung.Path}, "graphical-rung", "graphical FB lowering is not qualified", ErrUnsupportedRung)
		}
		if len(rung.Instructions) > MaxInstructions-count {
			return nil, failure(Diagnostic{Rung: rung.Path}, "source-limit", "too many instructions", ErrSourceLimit)
		}
		count += len(rung.Instructions)
		lines := make([]Line, 0, len(rung.Instructions))
		for _, instruction := range rung.Instructions {
			lines = append(lines, Line{instruction.Line, instruction.Text})
		}
		selected = append(selected, Rung{rung.Path, lines})
	}
	return CompileRungs(profile, selected)
}
