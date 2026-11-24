package compiler

import (
	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/il"
)

// CompileCXPMixedProgramBody composes textual rungs and the qualified scalar
// seven-cell FB call layout. Two calls to one scalar definition can be adjacent
// or separated by one LD/OUT rung in the same section, or straddle one
// section boundary. Interlocks must close
// before FB calls and section boundaries. It requires terminal END and explicit
// BCD for FB calls; binary is supported for textual-only programs. It emits no task
// envelope, definitions/instances, initialization or complete N1 image.
// Existing textual-only entry points deliberately keep rejecting graphics.
func CompileCXPMixedProgramBody(profile codec.Profile, doc *cxp.Document, program string, sectionMarkers bool) (*Body, error) {
	if profile.TimerCounterMode != codec.TimerModeBCD && profile.TimerCounterMode != codec.TimerModeBinary {
		return nil, failure(Diagnostic{Rung: program}, "profile", "program bodies require explicit BCD or binary mode", codec.ErrUnqualifiedInstruction)
	}
	if _, err := codec.EncodeInstruction(profile, il.Instruction{Mnemonic: "NOP"}); err != nil {
		return nil, failure(Diagnostic{Rung: program}, "profile", err.Error(), err)
	}
	source, err := doc.SourceForProgram(program)
	if err != nil {
		return nil, failure(Diagnostic{Rung: program}, "program-source", err.Error(), err)
	}
	result := &Body{Words: []uint32{}, Spans: []Span{}, Warnings: []Diagnostic{}}
	appendBody := func(piece *Body, where string) error {
		if len(piece.Words) == 0 {
			return nil
		}
		if result.HasEnd {
			return failure(Diagnostic{Rung: where}, "after-end", "instruction follows END", ErrStructure)
		}
		if len(result.Words) > MaxInstructions-len(piece.Words) {
			return failure(Diagnostic{Rung: where}, "source-limit", "mixed body exceeds word limit", ErrSourceLimit)
		}
		offset := len(result.Words)
		result.Words = append(result.Words, piece.Words...)
		for _, span := range piece.Spans {
			span.OffsetWords += offset
			result.Spans = append(result.Spans, span)
		}
		for _, span := range piece.Generated {
			span.OffsetWords += offset
			result.Generated = append(result.Generated, span)
		}
		result.Bindings = append(result.Bindings, piece.Bindings...)
		result.Warnings = append(result.Warnings, piece.Warnings...)
		result.MaxLogicDepth = max(result.MaxLogicDepth, piece.MaxLogicDepth)
		result.HasEnd = piece.HasEnd
		return nil
	}
	totalLines, totalBytes, calls := 0, 0, 0
	var firstCall string
	for index, section := range source.Sections {
		nonempty := false
		for _, rung := range section.Rungs {
			if len(rung.Instructions) != 0 || len(rung.Graphic) != 0 {
				nonempty = true
			}
		}
		if !nonempty {
			if sectionMarkers && index > 0 {
				return nil, failure(Diagnostic{Rung: section.Path}, "section-unqualified", "empty sections after the first require separate qualification", ErrUnsupportedRung)
			}
			continue
		}
		if sectionMarkers && index > 0 {
			marker, err := codec.EncodeInstruction(profile, il.Instruction{Mnemonic: "CMP", Function: "020", Operands: []string{"#DEAD", "#DEAD"}})
			if err != nil {
				return nil, err
			}
			piece := &Body{Words: marker, Generated: []GeneratedSpan{{Kind: "section-separator", Section: section.Path, LengthWords: len(marker)}}}
			if err := appendBody(piece, section.Path); err != nil {
				return nil, err
			}
		}
		var text []cxp.Rung
		flush := func() error {
			if len(text) == 0 {
				return nil
			}
			piece, err := CompileCXPProgramRungs(profile, doc, program, text)
			if err != nil {
				return err
			}
			if err := appendBody(piece, text[0].Path); err != nil {
				return err
			}
			text = nil
			return nil
		}
		for rungIndex, rung := range section.Rungs {
			totalLines += len(rung.Instructions) + len(rung.Graphic)
			if profile.TimerCounterMode == codec.TimerModeBinary && len(rung.Graphic) != 0 {
				return nil, failure(Diagnostic{Rung: rung.Path}, "profile", "binary program bodies require textual rungs without FB calls", codec.ErrUnqualifiedInstruction)
			}
			totalBytes += len(rung.Path)
			for _, instruction := range rung.Instructions {
				totalBytes += len(instruction.Text)
			}
			for _, field := range rung.Graphic {
				totalBytes += len(field.Name) + len(field.Raw) + len(field.Value)
			}
			if totalLines > MaxInstructions || totalBytes > MaxSourceBytes {
				return nil, failure(Diagnostic{Rung: rung.Path}, "source-limit", "mixed source exceeds limits", ErrSourceLimit)
			}
			if len(rung.Graphic) == 0 {
				if len(rung.Instructions) > 0 {
					text = append(text, rung)
				}
				continue
			}
			calls++
			if calls == 1 {
				firstCall = rung.Path
			} else {
				adjacent := rungIndex > 0 && section.Rungs[rungIndex-1].Path == firstCall
				acrossBoundary := false
				if index > 0 && rungIndex == 0 {
					previous := source.Sections[index-1].Rungs
					acrossBoundary = len(previous) > 0 && previous[len(previous)-1].Path == firstCall
				}
				withContactCoil := rungIndex >= 2 && section.Rungs[rungIndex-2].Path == firstCall && isContactCoil(section.Rungs[rungIndex-1])
				if calls > 2 || !(adjacent || withContactCoil || acrossBoundary) {
					return nil, failure(Diagnostic{Rung: rung.Path}, "fb-calls-unqualified", "two scalar calls may be separated by one LD/OUT rung or one section boundary", ErrUnsupportedRung)
				}
				first, err := doc.PlanFBCall(program, firstCall)
				if err != nil {
					return nil, err
				}
				second, err := doc.PlanFBCall(program, rung.Path)
				if err != nil {
					return nil, err
				}
				if first.DefinitionPath != second.DefinitionPath || len(first.Inputs) != 3 || len(first.Outputs) != 1 {
					return nil, failure(Diagnostic{Rung: rung.Path}, "fb-calls-unqualified", "paired calls require the same qualified scalar definition", ErrUnsupportedRung)
				}
			}
			if err := flush(); err != nil {
				return nil, err
			}
			piece, err := CompileCXPFBCallBody(profile, doc, program, rung.Path)
			if err != nil {
				return nil, err
			}
			if err := appendBody(piece, rung.Path); err != nil {
				return nil, err
			}
		}
		if err := flush(); err != nil {
			return nil, err
		}
	}
	if !result.HasEnd {
		return nil, failure(Diagnostic{Rung: program}, "missing-end", "program source requires explicit terminal END", ErrStructure)
	}
	return result, nil
}

func isContactCoil(rung cxp.Rung) bool {
	if len(rung.Graphic) != 0 || len(rung.Instructions) != 2 {
		return false
	}
	for index, mnemonic := range [...]string{"LD", "OUT"} {
		instruction, err := il.Parse(rung.Instructions[index].Text)
		if err != nil || instruction.Mnemonic != mnemonic || instruction.Modifier != "" || instruction.Function != "" {
			return false
		}
	}
	return true
}
