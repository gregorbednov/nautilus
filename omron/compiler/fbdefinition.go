package compiler

import (
	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/internal/fbcode"
)

type FBDefinition struct {
	Definition          string        `json:"definition"`
	Number              int           `json:"number"`
	BodyNumber          int           `json:"bodyNumber"`
	Name                string        `json:"name"`
	InputsIncludingEN   int           `json:"inputsIncludingEN"`
	OutputsIncludingENO int           `json:"outputsIncludingENO"`
	Allocation          *FBAllocation `json:"allocation"`
	Body                *Body         `json:"body"`
	RecordWords         []uint32      `json:"recordWords"`
}

// CompileCXPFBDefinition emits one bounded offline scalar definition record.
// The caller supplies its number; source TaskID supplies the FBBN body number.
// It does not place instances, assign project identifiers or link an N1 image.
func CompileCXPFBDefinition(profile codec.Profile, doc *cxp.Document, path string, number int) (*FBDefinition, error) {
	source, err := doc.SourceForFBDefinition(path)
	if err != nil {
		return nil, err
	}
	compiled, err := CompileCXPFBDefinitionBody(profile, doc, path)
	if err != nil {
		return nil, err
	}
	entry, exit, err := fbcode.DefinitionEnvelope(8, source.TaskID)
	if err != nil {
		return nil, err
	}
	if len(compiled.Body.Words) > MaxInstructions-len(entry)-len(exit) {
		return nil, ErrSourceLimit
	}
	body := compiled.Body
	words := make([]uint32, 0, len(entry)+len(body.Words)+len(exit))
	words = append(words, entry...)
	words = append(words, body.Words...)
	words = append(words, exit...)
	for i := range body.Spans {
		body.Spans[i].OffsetWords += len(entry)
	}
	body.Generated = []GeneratedSpan{{Kind: "fb-definition-entry", Section: path, OffsetWords: 0, LengthWords: len(entry)}, {Kind: "fb-definition-return", Section: path, OffsetWords: len(words) - len(exit), LengthWords: len(exit)}}
	body.Words = words
	inputs, outputs := 1, 1
	for _, variable := range source.Variables {
		if variable.Usage == 1 {
			inputs++
		}
		if variable.Usage == 2 {
			outputs++
		}
	}
	record, err := fbcode.AssembleDefinitionRecord(profile, number, source.Name, inputs, outputs, words)
	if err != nil {
		return nil, err
	}
	return &FBDefinition{Definition: path, Number: number, BodyNumber: source.TaskID, Name: source.Name, InputsIncludingEN: inputs, OutputsIncludingENO: outputs, Allocation: compiled.Allocation, Body: body, RecordWords: record}, nil
}
