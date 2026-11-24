package compiler

import (
	"fmt"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/internal/fbcode"
)

// Only the single indirect BOOL inversion is measured. Its instruction
// prefixes include the logic context of this particular two-instruction rung.
func compileInOutBOOLBody(doc *cxp.Document, path string) (*FBDefinitionBody, error) {
	source, err := doc.SourceForFBDefinition(path)
	if err != nil {
		return nil, err
	}
	if len(source.Variables) != 1 || len(source.Rungs) != 1 {
		return nil, fmt.Errorf("%w: expected one In Out BOOL and one rung", ErrFBAllocation)
	}
	variable := source.Variables[0]
	rung := source.Rungs[0]
	if variable.Usage != 4 || variable.TypeCode != 0 || !fbVariableName.MatchString(variable.Name) ||
		len(rung.Instructions) != 2 || rung.Instructions[0].Mnemonic != "LDNOT" || rung.Instructions[1].Mnemonic != "OUT" {
		return nil, fmt.Errorf("%w: expected LDNOT and OUT on the same In Out BOOL", ErrFBAllocation)
	}
	for _, ins := range rung.Instructions {
		if len(ins.Operands) != 1 || ins.Operands[0].Name != variable.Name {
			return nil, fmt.Errorf("%w: %s line %d requires the declared In Out operand", ErrFBAllocation, rung.Path, ins.Line)
		}
	}
	// EN and ENO use two bits of the first word. The following two words
	// hold the caller-supplied address; the bit itself lives in caller memory.
	frame, err := PlanOfflineFBAllocation(nil)
	if err != nil {
		return nil, err
	}
	bank := &frame.Memory[3]
	offset := bank.Words
	frame.Variables = append(frame.Variables, FBAllocatedVariable{Name: variable.Name, UsageCode: variable.Usage,
		TypeCode: variable.TypeCode, DeclarationLine: variable.Line,
		Reference: codec.FBRelativeOperand{BaseRegister: 3, Offset: offset, Unit: "word"}})
	bank.SizeBits += 32
	bank.OtherWords += 2
	bank.Words += 2

	body := &Body{MaxLogicDepth: 1, Warnings: []Diagnostic{}}
	body.Words = fbcode.InOutBOOLInversion()
	body.Spans = []Span{
		{Rung: rung.Path, Line: rung.Instructions[0].Line, OffsetWords: 6, LengthWords: 2},
		{Rung: rung.Path, Line: rung.Instructions[1].Line, OffsetWords: 8, LengthWords: 2},
	}
	body.Generated = []GeneratedSpan{
		{Kind: "fb-inout-address-setup", Section: rung.Path, OffsetWords: 0, LengthWords: 6},
		{Kind: "fb-inout-address-restore", Section: rung.Path, OffsetWords: 10, LengthWords: 3},
	}
	return &FBDefinitionBody{Definition: path, Allocation: frame, Body: body}, nil
}
