package compiler

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/il"
)

var ErrFBAllocation = errors.New("unqualified offline scalar FB allocation")
var fbVariableName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)

type FBAllocatedVariable struct {
	Name            string                  `json:"name"`
	UsageCode       int                     `json:"usageCode"`
	TypeCode        int                     `json:"typeCode"`
	DeclarationLine int                     `json:"declarationLine,omitempty"`
	Implicit        bool                    `json:"implicit,omitempty"`
	Reference       codec.FBRelativeOperand `json:"reference"`
}

type FBMemoryFootprint struct {
	BaseRegister int `json:"baseRegister"`
	SizeBits     int `json:"sizeBits"`
	Words        int `json:"words"`
	BoolWords    int `json:"boolWords"`
	OtherWords   int `json:"otherWords"`
}

// FBAllocation is a logical relative frame. It does not place instances in PLC
// memory, reserve online-edit space or establish CPU resource capacity.
type FBAllocation struct {
	Variables []FBAllocatedVariable `json:"variables"`
	Memory    []FBMemoryFootprint   `json:"memory"`
}

// PlanOfflineFBAllocation covers scalar BOOL/INT/UINT/WORD, internal TIMER,
// usage input/output/internal, and zero online reservations. Within each usage,
// declaration order is retained. EN/ENO occupy the first two BR3 bits.
func PlanOfflineFBAllocation(variables []cxp.FBVariable) (*FBAllocation, error) {
	if len(variables) > MaxInstructions {
		return nil, fmt.Errorf("%w: declaration limit", ErrFBAllocation)
	}
	boolCount, wordCount, timerCount := 2, 0, 0
	seen := map[string]bool{"EN": true, "ENO": true}
	for _, variable := range variables {
		key := strings.ToUpper(variable.Name)
		if !fbVariableName.MatchString(variable.Name) || seen[key] || variable.Usage < 0 || variable.Usage > 2 || variable.RemainingColumns[0] != "" || variable.RemainingColumns[1] != "0" || variable.RemainingColumns[3] != "0" {
			return nil, fmt.Errorf("%w: variable %s metadata/name/usage", ErrFBAllocation, variable.Name)
		}
		seen[key] = true
		switch variable.TypeCode {
		case 0:
			boolCount++
		case 1, 2, 7:
			wordCount++
		case 13:
			if variable.Usage != 0 {
				return nil, fmt.Errorf("%w: TIMER ports", ErrFBAllocation)
			}
			timerCount++
		default:
			return nil, fmt.Errorf("%w: type %d", ErrFBAllocation, variable.TypeCode)
		}
	}
	boolWords := (boolCount + 15) / 16
	if boolCount > 2048 || boolWords+wordCount > 2048 || timerCount > 2048 {
		return nil, fmt.Errorf("%w: short relative displacement limit", ErrFBAllocation)
	}
	result := &FBAllocation{Variables: make([]FBAllocatedVariable, len(variables)+2), Memory: make([]FBMemoryFootprint, 5)}
	result.Variables[0] = FBAllocatedVariable{Name: "EN", UsageCode: 1, TypeCode: 0, Implicit: true, Reference: codec.FBRelativeOperand{BaseRegister: 3, Offset: 0, Unit: "bit"}}
	result.Variables[1] = FBAllocatedVariable{Name: "ENO", UsageCode: 2, TypeCode: 0, Implicit: true, Reference: codec.FBRelativeOperand{BaseRegister: 3, Offset: 1, Unit: "bit"}}
	bitOffset, wordOffset, timerOffset := 2, boolWords, 0
	for _, usage := range []int{1, 2, 0} {
		for i, variable := range variables {
			if variable.Usage != usage {
				continue
			}
			var reference codec.FBRelativeOperand
			switch variable.TypeCode {
			case 0:
				reference = codec.FBRelativeOperand{BaseRegister: 3, Offset: bitOffset, Unit: "bit"}
				bitOffset++
			case 1, 2, 7:
				reference = codec.FBRelativeOperand{BaseRegister: 3, Offset: wordOffset, Unit: "word"}
				wordOffset++
			case 13:
				reference = codec.FBRelativeOperand{BaseRegister: 0, Offset: timerOffset, Unit: "timer"}
				timerOffset++
			}
			result.Variables[i+2] = FBAllocatedVariable{Name: variable.Name, UsageCode: variable.Usage, TypeCode: variable.TypeCode, DeclarationLine: variable.Line, Reference: reference}
		}
	}
	for base := range result.Memory {
		result.Memory[base].BaseRegister = base
	}
	result.Memory[0] = FBMemoryFootprint{BaseRegister: 0, SizeBits: timerCount * 16, Words: timerCount}
	result.Memory[3] = FBMemoryFootprint{BaseRegister: 3, SizeBits: boolCount + wordCount*16, Words: boolWords + wordCount, BoolWords: boolWords, OtherWords: wordCount}
	return result, nil
}

type FBDefinitionBody struct {
	Definition string        `json:"definition"`
	Allocation *FBAllocation `json:"allocation"`
	Body       *Body         `json:"body"`
}

// CompileCXPFBDefinitionBody allocates the qualified offline scalar frame and
// compiles observed series networks directly from source. The result excludes
// FBBN/FBBRET, instance initialization, task linking and the complete N1 image.
func CompileCXPFBDefinitionBody(profile codec.Profile, doc *cxp.Document, path string) (*FBDefinitionBody, error) {
	if profile == (codec.Profile{Model: codec.CJ1HCPU65, UnitVersion: "4.0", TimerCounterMode: codec.TimerModeBCD}) {
		return compileInOutBOOLBody(doc, path)
	}
	if _, err := codec.EncodeInstruction(profile, il.Instruction{Mnemonic: "NOP"}); err != nil {
		return nil, err
	}
	source, err := doc.SourceForFBDefinition(path)
	if err != nil {
		return nil, err
	}
	for _, v := range source.Variables {
		if v.Usage == 4 {
			if profile.InOutBOOLVersion == "4.0" {
				return compileInOutBOOLBody(doc, path)
			}
			return nil, fmt.Errorf("%w: %s line %d: In Out BOOL body requires CJ1H-H CPU65, explicit unit version 4.0 and BCD", ErrFBAllocation, path, v.Line)
		}
	}
	allocation, err := PlanOfflineFBAllocation(source.Variables)
	if err != nil {
		return nil, err
	}
	bindings := map[string]FBAllocatedVariable{}
	for _, variable := range allocation.Variables {
		bindings[variable.Name] = variable
	}
	body := &Body{Words: []uint32{}, Spans: []Span{}, Warnings: []Diagnostic{}, MaxLogicDepth: 1}
	for _, rung := range source.Rungs {
		for _, instruction := range rung.Instructions {
			where := Diagnostic{Rung: rung.Path, Line: instruction.Line}
			native := codec.FBRelativeInstruction{Mnemonic: instruction.Mnemonic}
			if instruction.Mnemonic == "TIMH" {
				native.Function = "015"
			}
			for _, operand := range instruction.Operands {
				binding, ok := bindings[operand.Name]
				if !ok || binding.TypeCode != operand.TypeCode {
					return nil, failure(where, "fb-binding", "unresolved relative reference", ErrFBAllocation)
				}
				native.Operands = append(native.Operands, binding.Reference)
			}
			words, err := codec.EncodeFBRelativeInstruction(profile, native)
			if err != nil {
				return nil, failure(where, "fb-relative", err.Error(), err)
			}
			if len(body.Spans) >= MaxInstructions || len(body.Words) > MaxInstructions-len(words) {
				return nil, failure(where, "fb-limit", "definition body limit", ErrSourceLimit)
			}
			body.Spans = append(body.Spans, Span{Rung: rung.Path, Line: instruction.Line, OffsetWords: len(body.Words), LengthWords: len(words)})
			body.Words = append(body.Words, words...)
		}
	}
	return &FBDefinitionBody{Definition: path, Allocation: allocation, Body: body}, nil
}
