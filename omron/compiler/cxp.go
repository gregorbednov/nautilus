package compiler

import (
	"errors"
	"strings"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/cxp"
)

// CompileCXPProgramBody compiles all declared source rungs in section order.
// Empty source rungs remain boundaries. It requires an explicit terminal END
// and does not generate native section markers, task wrappers or an N1 image.
func CompileCXPProgramBody(profile codec.Profile, doc *cxp.Document, program string) (*Body, error) {
	source, err := doc.SourceForProgram(program)
	if err != nil {
		where := Diagnostic{Rung: program}
		var detail *cxp.ProgramSourceError
		if errors.As(err, &detail) {
			where.Rung = detail.Path
			where.Line = detail.Line
		}
		return nil, failure(where, "program-source", err.Error(), err)
	}
	var rungs []cxp.Rung
	for _, section := range source.Sections {
		rungs = append(rungs, section.Rungs...)
	}
	body, err := CompileCXPProgramRungs(profile, doc, program, rungs)
	if err != nil {
		return nil, err
	}
	if !body.HasEnd {
		return nil, failure(Diagnostic{Rung: program}, "missing-end", "program source requires explicit terminal END", ErrStructure)
	}
	return body, nil
}

// CompileCXPRungs compiles explicitly selected textual rungs with native source
// locations. It does not select CPU/tasks, flatten graphical FBs, trust cached
// Parsed instructions or claim that the remaining project is compilable.
func CompileCXPRungs(profile codec.Profile, rungs []cxp.Rung) (*Body, error) {
	selected, err := sourceCXPRungs(rungs)
	if err != nil {
		return nil, err
	}
	return CompileRungs(profile, selected)
}

// CompileCXPProgramRungs resolves exact names from this program's own locals and
// resource globals. Callers select the source rungs; no other scope is searched.
func CompileCXPProgramRungs(profile codec.Profile, doc *cxp.Document, program string, rungs []cxp.Rung) (*Body, error) {
	tables, err := doc.SymbolsForProgram(program)
	if err != nil {
		where := Diagnostic{Rung: program}
		var diagnostic *cxp.SymbolError
		if errors.As(err, &diagnostic) {
			where.Rung = diagnostic.Scope
			where.Line = diagnostic.Line
		}
		return nil, failure(where, "symbol-table", err.Error(), err)
	}
	for _, rung := range rungs {
		if !strings.HasPrefix(rung.Path, program+".Sections.") {
			return nil, failure(Diagnostic{Rung: rung.Path}, "symbol-scope", "rung does not belong to selected program", cxp.ErrSymbolTable)
		}
	}
	resolver, err := newSymbolResolver(tables)
	if err != nil {
		return nil, failure(Diagnostic{Rung: program}, "symbol-table", err.Error(), err)
	}
	selected, err := sourceCXPRungs(rungs)
	if err != nil {
		return nil, err
	}
	return compileRungs(profile, selected, resolver.resolve)
}

func sourceCXPRungs(rungs []cxp.Rung) ([]Rung, error) {
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
	return selected, nil
}
