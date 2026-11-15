package compiler

import (
	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/cxp"
)

// TaskRecord is one standalone zero-flag N1 task record. SourceSpans/Generated
// address RecordWords; Code retains its own body-relative spans. Task
// configuration, directory, global header, FB records and initial data are absent.
type TaskRecord struct {
	Source      *cxp.TaskSource `json:"source"`
	Code        *Body           `json:"code"`
	RecordWords []uint32        `json:"recordWords"`
	SourceSpans []Span          `json:"sourceSpans"`
	Generated   []GeneratedSpan `json:"generated"`
}

func CompileCXPTaskRecord(profile codec.Profile, doc *cxp.Document, program string) (*TaskRecord, error) {
	source, err := doc.SourceForTask(program)
	if err != nil {
		return nil, err
	}
	code, err := CompileCXPMixedProgramBody(profile, doc, program, true)
	if err != nil {
		return nil, err
	}
	if len(code.Words) > MaxInstructions-37 {
		return nil, ErrSourceLimit
	}
	header := make([]uint32, 32)
	header[0] = uint32(source.Number)
	header[26] = uint32(37 + len(code.Words))
	header[27] = 32
	for i, b := range []byte(source.Name) {
		header[1+i/3] |= uint32(b) << uint(16-8*(i%3))
	}
	entry := []uint32{0, 0xbbc00f43, 0x61440000 + uint32(source.Number), 0x65440000}
	words := make([]uint32, 0, len(code.Words)+37)
	words = append(words, header...)
	words = append(words, entry...)
	words = append(words, code.Words...)
	words = append(words, 0xbb800f48)
	result := &TaskRecord{Source: source, Code: code, RecordWords: words, SourceSpans: []Span{}, Generated: []GeneratedSpan{
		{Kind: "task-header", Section: program, OffsetWords: 0, LengthWords: 32},
		{Kind: "task-entry", Section: program, OffsetWords: 32, LengthWords: 4},
	}}
	for _, span := range code.Spans {
		span.OffsetWords += 36
		result.SourceSpans = append(result.SourceSpans, span)
	}
	for _, span := range code.Generated {
		span.OffsetWords += 36
		result.Generated = append(result.Generated, span)
	}
	result.Generated = append(result.Generated, GeneratedSpan{Kind: "task-return", Section: program, OffsetWords: len(words) - 1, LengthWords: 1})
	return result, nil
}
