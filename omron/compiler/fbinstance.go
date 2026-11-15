package compiler

import (
	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/internal/fbcode"
)

type FBInstance struct {
	Source      *cxp.FBInstanceSource `json:"source"`
	Allocation  *FBAllocation         `json:"allocation"`
	Body        *Body                 `json:"body"`
	RecordWords []uint32              `json:"recordWords"`
}

// CompileCXPFBInstance covers the observed two-word-input/BOOL-input/BOOL-output
// runtime frame, using saved placement. It does not initialize variable values,
// choose new addresses, link tasks or qualify the complete executable project.
func CompileCXPFBInstance(profile codec.Profile, doc *cxp.Document, path string) (*FBInstance, error) {
	instance, err := doc.SourceForFBInstance(path)
	if err != nil {
		return nil, err
	}
	definition, err := CompileCXPFBDefinitionBody(profile, doc, instance.DefinitionPath)
	if err != nil {
		return nil, err
	}
	source, err := doc.SourceForFBDefinition(instance.DefinitionPath)
	if err != nil {
		return nil, err
	}
	allocation := definition.Allocation
	var inputs, outputs []FBAllocatedVariable
	for _, variable := range allocation.Variables {
		if variable.Implicit {
			continue
		}
		if variable.UsageCode == 1 {
			inputs = append(inputs, variable)
		}
		if variable.UsageCode == 2 {
			outputs = append(outputs, variable)
		}
	}
	if len(inputs) != 3 || len(outputs) != 1 || inputs[0].Reference.Unit != "word" || inputs[1].Reference.Unit != "word" || inputs[2].Reference.Unit != "bit" || outputs[0].Reference.Unit != "bit" {
		return nil, fbcode.ErrInstance
	}
	var sizes [5]int
	for i := range sizes {
		sizes[i] = allocation.Memory[i].Words
	}
	if err := fbcode.ValidateInstanceBases(instance.Bases, sizes); err != nil {
		return nil, err
	}
	entry, exit, err := fbcode.InstanceEnvelope(instance.Number)
	if err != nil {
		return nil, err
	}
	body := &Body{Words: []uint32{}, Spans: []Span{}, Warnings: []Diagnostic{}, MaxLogicDepth: 1}
	appendCode := func(kind string, words []uint32) {
		body.Generated = append(body.Generated, GeneratedSpan{Kind: kind, Section: path, OffsetWords: len(body.Words), LengthWords: len(words)})
		body.Words = append(body.Words, words...)
	}
	appendCode("fb-instance-entry", entry)
	for bank, base := range instance.Bases {
		words, err := fbcode.InstanceBaseMove(bank, base.Address)
		if err != nil {
			return nil, err
		}
		appendCode("fb-instance-base-setup", words)
	}
	control, err := fbcode.InstanceControl(instance.Number)
	if err != nil {
		return nil, err
	}
	for _, words := range control {
		appendCode("fb-instance-enable-control", words)
	}
	for i := len(inputs) - 1; i >= 0; i-- {
		variable := inputs[i]
		words, err := fbcode.InstanceTransfer(variable.Reference.Unit == "bit", false, variable.Reference.Offset)
		if err != nil {
			return nil, err
		}
		appendCode("fb-instance-input-"+variable.Name, words)
	}
	words, _ := fbcode.InstanceTransfer(true, false, 0)
	appendCode("fb-instance-enable-input", words)
	call, err := fbcode.InstanceBodyCall(source.TaskID)
	if err != nil {
		return nil, err
	}
	appendCode("fb-instance-body-call", call)
	words, _ = fbcode.InstanceTransfer(true, true, 1)
	appendCode("fb-instance-enable-output", words)
	words, err = fbcode.InstanceTransfer(true, true, outputs[0].Reference.Offset)
	if err != nil {
		return nil, err
	}
	appendCode("fb-instance-output-"+outputs[0].Name, words)
	appendCode("fb-instance-return", exit)
	record := make([]uint32, 10)
	record[0] = uint32(instance.Number)
	record[2] = uint32(10 + len(body.Words))
	for i, size := range sizes {
		record[3+i] = uint32(size)
	}
	record = append(record, body.Words...)
	return &FBInstance{Source: instance, Allocation: allocation, Body: body, RecordWords: record}, nil
}
