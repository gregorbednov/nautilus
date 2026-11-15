package compiler

import (
	"fmt"
	"sort"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/cxp"
)

type TaskRecordSpan struct {
	Program     string `json:"program"`
	Number      int    `json:"number"`
	OffsetWords int    `json:"offsetWords"`
	LengthWords int    `json:"lengthWords"`
}

// TaskBlock starts with the observed288-word directory, followed by task
// records sorted by ID. Directory and span offsets are block-relative. Source
// spans include the directory shift; individual TaskRecords retain local maps.
type TaskBlock struct {
	Resource       string           `json:"resource"`
	Tasks          []*TaskRecord    `json:"tasks"`
	DirectoryWords []uint32         `json:"directoryWords"`
	Words          []uint32         `json:"words"`
	Records        []TaskRecordSpan `json:"records"`
	SourceSpans    []Span           `json:"sourceSpans"`
	Generated      []GeneratedSpan  `json:"generated"`
}

func CompileCXPTaskBlock(profile codec.Profile, doc *cxp.Document, resource string) (*TaskBlock, error) {
	sources, err := doc.SourceForTasks(resource)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("%w: task block requires at least one task", ErrStructure)
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].Number < sources[j].Number })
	result := &TaskBlock{Resource: resource, Tasks: []*TaskRecord{}, DirectoryWords: make([]uint32, 288), Words: make([]uint32, 288), Records: []TaskRecordSpan{}, SourceSpans: []Span{}, Generated: []GeneratedSpan{{Kind: "task-directory", Section: resource, LengthWords: 288}}}
	for _, source := range sources {
		task, err := CompileCXPTaskRecord(profile, doc, source.Path)
		if err != nil {
			return nil, err
		}
		if len(result.Words) > MaxInstructions-len(task.RecordWords) {
			return nil, ErrSourceLimit
		}
		offset := len(result.Words)
		result.DirectoryWords[source.Number] = uint32(offset)
		result.Records = append(result.Records, TaskRecordSpan{Program: source.Path, Number: source.Number, OffsetWords: offset, LengthWords: len(task.RecordWords)})
		for _, span := range task.SourceSpans {
			span.OffsetWords += offset
			result.SourceSpans = append(result.SourceSpans, span)
		}
		for _, span := range task.Generated {
			span.OffsetWords += offset
			result.Generated = append(result.Generated, span)
		}
		result.Words = append(result.Words, task.RecordWords...)
		result.Tasks = append(result.Tasks, task)
	}
	copy(result.Words, result.DirectoryWords)
	return result, nil
}
