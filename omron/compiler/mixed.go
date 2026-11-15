package compiler

import (
	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/il"
)

// CompileCXPMixedProgramBody composes textual rungs and the qualified scalar
// seven-cell FB call layout, at most once per program. Interlocks must close
// before FB calls and section boundaries. It requires terminal END and explicit
// BCD, but emits no task
// envelope, definitions/instances, initialization or complete N1 image.
// Existing textual-only entry points deliberately keep rejecting graphics.
func CompileCXPMixedProgramBody(profile codec.Profile, doc *cxp.Document, program string, sectionMarkers bool) (*Body, error) {
	if profile.TimerCounterMode != codec.TimerModeBCD {
		return nil, failure(Diagnostic{Rung: program}, "profile", "mixed FB program bodies require explicit BCD mode", codec.ErrUnqualifiedInstruction)
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
		for _, rung := range section.Rungs {
			totalLines += len(rung.Instructions) + len(rung.Graphic)
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
			if calls > 1 {
				return nil, failure(Diagnostic{Rung: rung.Path}, "fb-calls-unqualified", "multiple FB calls per program require separate native qualification", ErrUnsupportedRung)
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
