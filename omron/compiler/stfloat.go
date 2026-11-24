package compiler

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/joyautomation/nautilus/lang/st"
	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/internal/fbcode"
)

// STFloatDefinition is the CPU21 record for a word input and a REAL result.
// The two local REALs and the result occupy pairs of BR3 words.
type STFloatDefinition struct {
	Name        string   `json:"name"`
	Number      int      `json:"number"`
	BodyNumber  int      `json:"bodyNumber"`
	RecordWords []uint32 `json:"recordWords"`
}

type floatFrame struct {
	input     string
	realSlots map[string]uint32
	code      []uint32
}

func planFloatFrame(fb *st.FunctionBlockDecl) (*floatFrame, error) {
	frame := &floatFrame{realSlots: map[string]uint32{}}
	groups := map[string][]string{}
	seen := map[string]bool{"EN": true, "ENO": true}
	for _, block := range fb.VarBlocks {
		typ := "REAL"
		switch block.Kind {
		case "VAR_INPUT":
			typ = "INT"
		case "VAR_OUTPUT", "VAR":
		default:
			return nil, fmt.Errorf("unsupported block %s", block.Kind)
		}
		if block.Retain || block.Constant {
			return nil, fmt.Errorf("unsupported qualifiers for %s", block.Kind)
		}
		for _, v := range block.Variables {
			key := strings.ToUpper(v.Name)
			if seen[key] || v.Initial != nil || !strings.EqualFold(v.Datatype, typ) {
				return nil, fmt.Errorf("line %d: expected a distinct uninitialized %s in %s", v.Pos.Line, typ, block.Kind)
			}
			seen[key] = true
			groups[block.Kind] = append(groups[block.Kind], key)
		}
	}
	if len(groups["VAR_INPUT"]) != 1 || len(groups["VAR_OUTPUT"]) != 1 || len(groups["VAR"]) != 2 {
		return nil, fmt.Errorf("expected one INT input, one REAL output and two local REALs")
	}
	frame.input = groups["VAR_INPUT"][0]
	for i, name := range append(groups["VAR_OUTPUT"], groups["VAR"]...) {
		frame.realSlots[name] = uint32(2 + i*2)
	}
	return frame, nil
}

const floatTemporary = 0x6937fffe

var floatOps = map[string]uint32{"+": 0xb3410300, "-": 0xb3410302, "*": 0xb3410304, "/": 0xb3410306}

// Arithmetic uses one IR63 temporary. Preserve operand order, including a
// literal on the left of a nested expression.
func (f *floatFrame) operand(expr st.Expression) ([]uint32, error) {
	switch v := expr.(type) {
	case *st.NumberLit:
		value, err := strconv.ParseFloat(v.Value, 32)
		if err != nil {
			return nil, err
		}
		bits := math.Float32bits(float32(value))
		return []uint32{0x69400000 | uint32(uint16(bits)), 0x61400000 | bits>>16}, nil
	case *st.IdentExpr:
		if slot, ok := f.realSlots[strings.ToUpper(v.Name)]; ok {
			return []uint32{0x693c3000 | slot}, nil
		}
	case *st.CallExpr:
		if strings.EqualFold(v.Name, "INT_TO_REAL") && v.Callee == nil && len(v.Args) == 1 && len(v.NamedArgs)+len(v.OutputBindings) == 0 {
			if id, ok := v.Args[0].(*st.IdentExpr); ok && strings.EqualFold(id.Name, f.input) {
				f.code = append(f.code, 0xb341030a, 0x613c3001, 0x5d37fffe)
				return []uint32{floatTemporary}, nil
			}
		}
	case *st.BinaryExpr:
		op, ok := floatOps[v.Op]
		_, leftLit := v.Left.(*st.NumberLit)
		_, rightLit := v.Right.(*st.NumberLit)
		if !ok || leftLit == rightLit {
			return nil, fmt.Errorf("expected REAL arithmetic with one literal operand")
		}
		a, err := f.operand(v.Left)
		if err != nil {
			return nil, err
		}
		b, err := f.operand(v.Right)
		if err != nil {
			return nil, err
		}
		f.code = append(f.code, op)
		f.code = append(f.code, a...)
		f.code = append(f.code, b...)
		f.code = append(f.code, 0x5d37fffe)
		return []uint32{floatTemporary}, nil
	}
	return nil, fmt.Errorf("unsupported REAL operand")
}

// CompileSTFloatDefinition lowers the observed scalar ST frame directly from
// editable ST. Record IDs and body numbers are independent linker inputs.
func CompileSTFloatDefinition(profile codec.Profile, source string, number, bodyNumber int) (*STFloatDefinition, error) {
	if profile != (codec.Profile{Model: codec.CJ1MCPU21, TimerCounterMode: codec.TimerModeBCD}) {
		return nil, codec.ErrUnqualifiedProfile
	}
	if number < 0 || number > 1023 {
		return nil, fmt.Errorf("definition number outside 0..1023")
	}
	if len(source) > 65536 {
		return nil, ErrSourceLimit
	}
	entry, exit, err := fbcode.DefinitionEnvelope(8, bodyNumber)
	if err != nil {
		return nil, err
	}
	if !utf8.ValidString(source) {
		return nil, fmt.Errorf("ST source must be UTF-8")
	}
	fb, err := st.ParseFunctionBlock(strings.TrimPrefix(source, "\uFEFF"))
	if err != nil {
		return nil, err
	}
	if !fbVariableName.MatchString(fb.Name) || len(fb.Name) > 64 {
		return nil, fmt.Errorf("invalid definition name %q", fb.Name)
	}
	f, err := planFloatFrame(fb)
	if err != nil {
		return nil, err
	}
	f.code = append(entry, 0x9b7f0201, 0x69460003, 0x61460000, 0x6907f000, 0x5d07f000)
	for _, stmt := range fb.Statements {
		a, ok := stmt.(*st.AssignStmt)
		if !ok {
			return nil, fmt.Errorf("expected REAL assignment in %s", fb.Name)
		}
		id, ok := a.TargetExpr.(*st.IdentExpr)
		if !ok {
			return nil, fmt.Errorf("line %d: expected scalar destination", a.Pos.Line)
		}
		slot, ok := f.realSlots[strings.ToUpper(id.Name)]
		if !ok {
			return nil, fmt.Errorf("line %d: unknown REAL destination %s", a.Pos.Line, id.Name)
		}
		value, err := f.operand(a.Value)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", a.Pos.Line, err)
		}
		if len(value) != 1 || value[0] != floatTemporary {
			return nil, fmt.Errorf("line %d: expected conversion or arithmetic expression", a.Pos.Line)
		}
		f.code = append(f.code, 0xb3410141, floatTemporary, 0x5d3c3000|slot)
		if len(f.code) > MaxInstructions-32 {
			return nil, ErrSourceLimit
		}
	}
	f.code = append(f.code, 0xb0000814, 0x9b7f0201, 0x6946fffd, 0x6146ffff, 0x6907f000, 0x5d07f000)
	f.code = append(f.code, exit...)
	header := make([]uint32, 23)
	header[0] = 0xc00 | uint32(number)
	header[2] = uint32(len(header) + len(f.code))
	name := bytes.Repeat([]byte{' '}, 64)
	copy(name, fb.Name)
	for i := 0; i < 16; i++ {
		header[3+i] = binary.BigEndian.Uint32(name[4*i:])
	}
	return &STFloatDefinition{fb.Name, number, bodyNumber, append(header, f.code...)}, nil
}
