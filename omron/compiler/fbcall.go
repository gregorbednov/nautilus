package compiler

import (
	"strings"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/il"
	"github.com/joyautomation/nautilus/omron/internal/fbcode"
)

// CompileCXPFBCallBody lowers the observed scalar and single In Out BOOL
// graphical calls, including the enable contact, into an unlinked body.
func CompileCXPFBCallBody(profile codec.Profile, doc *cxp.Document, program, rung string) (*Body, error) {
	plan, err := doc.PlanFBCall(program, rung)
	if err != nil {
		return nil, failure(Diagnostic{Rung: rung}, "fb-call-source", err.Error(), err)
	}
	where := Diagnostic{Rung: rung, Line: plan.Enable.Line}
	if plan.Enable.Symbol != nil {
		if _, err := boundOperand(*plan.Enable.Symbol, "bit"); err != nil {
			return nil, failure(where, "fb-enable-type", err.Error(), err)
		}
	}
	enable, err := il.Parse("LD " + plan.Enable.Operand)
	if err != nil {
		return nil, failure(where, "fb-enable", err.Error(), err)
	}
	contactProfile := profile
	if len(plan.Inputs) == 1 && plan.Inputs[0].Declaration.Usage == 4 {
		if !profile.SupportsInOutBOOL() {
			return nil, codec.ErrUnqualifiedProfile
		}
		if _, err := compileInOutBOOLBody(doc, plan.DefinitionPath); err != nil {
			return nil, err
		}
		// Reuse the ordinary W-area contact encoding for this measured call.
		address, err := codec.ParseBitAddress(plan.Enable.Operand)
		if err != nil || address.Area != codec.Work {
			return nil, fbcode.ErrPostamble
		}
		contactProfile.UnitVersion = ""
	}
	enableWords, err := codec.EncodeInstruction(contactProfile, enable)
	if err != nil {
		return nil, failure(where, "fb-enable", err.Error(), err)
	}
	var instructions [][]uint32
	var sourceLines []int
	bindings := []BindingUse{}
	bind := func(value cxp.FBCallOperand, index int) {
		if s := value.Symbol; s != nil {
			bindings = append(bindings, BindingUse{rung, value.Line, index, value.Source, value.Operand, s.DataType, s.Scope, s.Line})
		}
	}
	bind(plan.Enable, 0)
	appendPin := func(pin cxp.FBCallPin) error {
		var words []uint32
		var err error
		if pin.Declaration.Usage == 4 {
			if pin.Value.Symbol != nil {
				if _, err := boundOperand(*pin.Value.Symbol, "bit"); err != nil {
					return failure(Diagnostic{Rung: rung, Line: pin.Value.Line}, "fb-pin", err.Error(), err)
				}
			}
			words, err = fbcode.InOutBOOLAddress(pin.Value.Operand)
		} else {
			words, err = fbcode.ScalarTransferForPin(8, pin)
		}
		if err != nil {
			return failure(Diagnostic{Rung: rung, Line: pin.Value.Line}, "fb-pin", err.Error(), err)
		}
		instructions = append(instructions, words)
		sourceLines = append(sourceLines, pin.Value.Line)
		operand := 0
		if pin.Declaration.Usage == 2 {
			operand = 1
		}
		bind(pin.Value, operand)
		return nil
	}
	for _, pin := range plan.Inputs {
		if err := appendPin(pin); err != nil {
			return nil, err
		}
	}
	call, err := fbcode.InstanceCall(8, plan.InstanceNumber, plan.TaskID)
	if err != nil {
		return nil, failure(Diagnostic{Rung: rung, Line: plan.CallLine}, "fb-identifier", err.Error(), err)
	}
	instructions = append(instructions, call)
	sourceLines = append(sourceLines, plan.CallLine)
	for _, pin := range plan.Outputs {
		if err := appendPin(pin); err != nil {
			return nil, err
		}
	}
	// Actual CX-P caller12713de0 zeros EBX then pushes it at12713e40 for
	// GenerateInvocationCode. This selection is qualified only for this layout.
	const option = false
	frame, err := fbcode.AssembleInvocation(8, instructions, option)
	if err != nil {
		return nil, failure(Diagnostic{Rung: rung, Line: plan.CallLine}, "fb-frame", err.Error(), err)
	}
	pre, post, _ := fbcode.InvocationSkeleton(8, option)
	header, _ := fbcode.InvocationHeader(8, len(pre)+len(instructions)+len(post))
	wordCount := func(list [][]uint32) int {
		n := 0
		for _, words := range list {
			n += len(words)
		}
		return n
	}
	section := strings.SplitN(rung, ".ProgramData.", 2)[0]
	result := &Body{Words: append(enableWords, frame...), Spans: []Span{{rung, plan.Enable.Line, 0, len(enableWords)}}, Bindings: bindings, MaxLogicDepth: 1}
	offset := len(enableWords)
	result.Generated = append(result.Generated, GeneratedSpan{"fb-invocation-header", section, offset, len(header)})
	offset += len(header)
	result.Generated = append(result.Generated, GeneratedSpan{"fb-invocation-prologue", section, offset, wordCount(pre)})
	offset += wordCount(pre)
	for i, words := range instructions {
		result.Spans = append(result.Spans, Span{rung, sourceLines[i], offset, len(words)})
		offset += len(words)
	}
	result.Generated = append(result.Generated, GeneratedSpan{"fb-invocation-epilogue", section, offset, wordCount(post)})
	return result, nil
}
