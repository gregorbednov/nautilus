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

// CompileCXPFBInstance uses saved placement for the scalar or single
// In Out BOOL runtime frame.
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
	var inputs, outputs, indirect []FBAllocatedVariable
	for _, variable := range allocation.Variables {
		if variable.Implicit {
			continue
		}
		switch variable.UsageCode {
		case 1:
			inputs = append(inputs, variable)
		case 2:
			outputs = append(outputs, variable)
		case 4:
			indirect = append(indirect, variable)
		}
	}
	if len(indirect) == 0 {
		boolInput := (profile.Model == codec.CJ1HCPU65 || profile.Model == codec.CJ1MCPU21) && profile.TimerCounterMode == codec.TimerModeBCD && len(inputs) == 1 && inputs[0].Reference.Unit == "bit"
		mixedInputs := len(inputs) == 3 && inputs[0].Reference.Unit == "word" && inputs[1].Reference.Unit == "word" && inputs[2].Reference.Unit == "bit"
		if (!boolInput && !mixedInputs) || len(outputs) != 1 || outputs[0].Reference.Unit != "bit" {
			return nil, fbcode.ErrInstance
		}
	}
	if len(indirect) != 0 && (len(indirect) != 1 || len(inputs)+len(outputs) != 0 || indirect[0].Reference.BaseRegister != 3 || indirect[0].Reference.Offset != 1) {
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
	for _, v := range indirect {
		appendCode("fb-instance-address-"+v.Name, []uint32{0xbb7f0141, 0x6917f002, 0x5d3c3000 | uint32(v.Reference.Offset)})
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
	for _, output := range outputs {
		words, err = fbcode.InstanceTransfer(true, true, output.Reference.Offset)
		if err != nil {
			return nil, err
		}
		appendCode("fb-instance-output-"+output.Name, words)
	}
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
