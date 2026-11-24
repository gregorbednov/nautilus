package compiler

import (
	"strings"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/il"
)

// CompileCXPSectionedProgramBody includes the measured CJ1 section separator.
// It still excludes native task headers, entry/exit wrappers and N1 metadata.
// Empty sections after the first and cross-section interlocks remain unqualified.
func CompileCXPSectionedProgramBody(profile codec.Profile, doc *cxp.Document, program string) (*Body, error) {
	body, err := CompileCXPProgramBody(profile, doc, program)
	if err != nil {
		return nil, err
	}
	source, err := doc.SourceForProgram(program)
	if err != nil {
		return nil, err
	}
	marker, err := codec.EncodeInstruction(profile, il.Instruction{Mnemonic: "CMP", Function: "020", Operands: []string{"#DEAD", "#DEAD"}})
	if err != nil {
		return nil, err
	}
	result := &Body{Words: []uint32{}, Spans: []Span{}, Warnings: body.Warnings, MaxLogicDepth: body.MaxLogicDepth, HasEnd: body.HasEnd, Bindings: body.Bindings}
	next := 0
	for index, section := range source.Sections {
		count := 0
		for _, rung := range section.Rungs {
			count += len(rung.Instructions)
		}
		if count == 0 {
			if index == 0 {
				continue
			}
			return nil, failure(Diagnostic{Rung: section.Path}, "section-unqualified", "empty sections after the first require separate qualification", ErrUnsupportedRung)
		}
		// The source body is already validated and encoded. Check its qualified
		// IL/ILC containers without resolving the whole symbol table per section.
		end, interlocks := next, 0
		for end < len(body.Spans) && strings.HasPrefix(body.Spans[end].Rung, section.Path+".") {
			switch body.Words[body.Spans[end].OffsetWords] {
			case 0xb1000002:
				interlocks++
			case 0xbb000003:
				interlocks = 0
			}
			end++
		}
		if interlocks != 0 {
			return nil, failure(Diagnostic{Rung: section.Path}, "section-interlock", "interlocks crossing section separators require separate qualification", ErrStructure)
		}
		if index > 0 {
			result.Generated = append(result.Generated, GeneratedSpan{Kind: "section-separator", Section: section.Path, OffsetWords: len(result.Words), LengthWords: len(marker)})
			result.Words = append(result.Words, marker...)
		}
		for next < end {
			span := body.Spans[next]
			words := body.Words[span.OffsetWords : span.OffsetWords+span.LengthWords]
			span.OffsetWords = len(result.Words)
			result.Spans = append(result.Spans, span)
			result.Words = append(result.Words, words...)
			next++
		}
	}
	if next != len(body.Spans) {
		return nil, failure(Diagnostic{Rung: program}, "section-unqualified", "source spans do not follow declared sections", ErrStructure)
	}
	return result, nil
}
