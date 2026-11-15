package cxp

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var ErrFBCall = errors.New("unsupported or malformed CXP FB call")

// FBVariable preserves CXP's declaration codes. TypeCode is NOT the native
// nDATA_TYPE bitmask. TypeName identifies PH_TYPE, not native-code support.
type FBVariable struct {
	Name     string `json:"name"`
	Usage    int    `json:"usageCode"`
	TypeCode int    `json:"typeCode"`
	TypeName string `json:"typeName,omitempty"`
	// Columns3..6 are retained without assigning unverified field semantics.
	RemainingColumns [4]string `json:"remainingColumns"`
	Comment          string    `json:"comment"`
	Line             int       `json:"line"`
}

type FBCallOperand struct {
	Source  string  `json:"source"`
	Operand string  `json:"operand"`
	Line    int     `json:"line"`
	Symbol  *Symbol `json:"symbol,omitempty"`
}

type FBCallPin struct {
	Declaration FBVariable    `json:"declaration"`
	Value       FBCallOperand `json:"value"`
	Row         int           `json:"row"`
}

// FBCallPlan is source metadata for the observed single-call scalar layout.
// It does not compile a graph, map declaration types, allocate variables or
// validate saved allocation addresses. Definition/instance numbers are source
// references, not permission to emit a complete executable FB.
type FBCallPlan struct {
	Program        string        `json:"program"`
	Rung           string        `json:"rung"`
	CallLine       int           `json:"callLine"`
	Instance       string        `json:"instance"`
	InstanceNumber int           `json:"instanceNumber"`
	Definition     string        `json:"definition"`
	DefinitionPath string        `json:"definitionPath"`
	TaskID         int           `json:"definitionTaskID"`
	Version        string        `json:"version"`
	Enable         FBCallOperand `json:"enable"`
	Inputs         []FBCallPin   `json:"inputs"`
	Outputs        []FBCallPin   `json:"outputs"`
	Variables      []FBVariable  `json:"variables"`
}

func fbError(path string, line int, message string) error {
	return fmt.Errorf("%w: %s line %d: %s", ErrFBCall, path, line, message)
}

func fbNumber(text string, max int) (int, error) {
	n, err := strconv.Atoi(text)
	if err != nil || n < 0 || n > max || strconv.Itoa(n) != text {
		return 0, ErrFBCall
	}
	return n, nil
}

func fbVariables(header Field, path string) ([]FBVariable, string, error) {
	list, err := sourceField(header.Fields, "PouVarList", "list", path)
	if err != nil {
		return nil, "", err
	}
	count, err := sourceField(header.Fields, "PouVarCount", "scalar", path)
	if err != nil {
		return nil, "", err
	}
	n, err := fbNumber(count.Value, 65536)
	if err != nil || len(list.Raw) > MaxSymbolBytes {
		return nil, "", fbError(path, count.Line, "invalid declaration count/size")
	}
	version, err := sourceField(header.Fields, "InstanceID", "scalar", path)
	if err != nil || version.Text() == "" {
		return nil, "", fbError(path, header.Line, "missing declaration version")
	}
	var result []FBVariable
	seen := map[string]bool{}
	for i, row := range strings.Split(list.Raw, "\n") {
		row = strings.TrimSpace(row)
		if row == "" {
			continue
		}
		columns := strings.SplitN(row, ",", 8)
		line := list.Line + 2 + i
		if len(columns) != 8 || !strings.HasSuffix(columns[7], ";") || len(result) >= n {
			return nil, "", fbError(path, line, "unsupported declaration row")
		}
		for j := 0; j < 7; j++ {
			columns[j] = strings.TrimSpace(columns[j])
			if strings.ContainsAny(columns[j], "\"\x11") {
				return nil, "", fbError(path, line, "unsupported quoted declaration column")
			}
		}
		usage, e1 := fbNumber(columns[0], 2)
		typeCode, e2 := fbNumber(columns[2], 65535)
		key := strings.ToUpper(columns[1])
		if e1 != nil || e2 != nil || key == "" || seen[key] || key == "EN" || key == "ENO" {
			return nil, "", fbError(path, line, "invalid or ambiguous declaration")
		}
		seen[key] = true
		result = append(result, FBVariable{Name: columns[1], Usage: usage, TypeCode: typeCode, TypeName: FBTypeName(typeCode), RemainingColumns: [4]string{columns[3], columns[4], columns[5], columns[6]}, Comment: strings.TrimSuffix(columns[7], ";"), Line: line})
	}
	if len(result) != n {
		return nil, "", fbError(path, count.Line, "declaration count mismatch")
	}
	return result, version.Text(), nil
}

var fbRungPath = regexp.MustCompile(`^(Resource\[[0-9]+\]\.Programs\.Program\[[0-9]+\])\.Sections\.Sec\[[0-9]+\]\.ProgramData\.R\[[0-9]+\]$`)
var fbDirectOperand = regexp.MustCompile(`(?i)^(?:#[0-9a-f]+|&[0-9]+|[0-9]+(?:\.[0-9]+)?|[WHADTC][0-9]+(?:\.[0-9]+)?)$`)

// PlanFBCall reads the measured seven-cell call layout (one LD enable, three
// scalar inputs, one scalar output and one wire). Input/output rows are matched
// to declaration order, NOT to serialized element order. Other layouts fail.
// Unknown type codes are retained; successful planning does not qualify them.
func (doc *Document) PlanFBCall(program, rungPath string) (*FBCallPlan, error) {
	plan, err := doc.planFBCall(program, rungPath)
	if err != nil && !errors.Is(err, ErrFBCall) {
		return nil, fmt.Errorf("%w: %w", ErrFBCall, err)
	}
	return plan, err
}

func (doc *Document) planFBCall(program, rungPath string) (*FBCallPlan, error) {
	if doc == nil || !fbRungPath.MatchString(rungPath) || fbRungPath.FindStringSubmatch(rungPath)[1] != program {
		return nil, fbError(rungPath, 0, "invalid program/rung scope")
	}
	source, err := doc.SourceForProgram(program)
	if err != nil {
		return nil, err
	}
	var rung *Rung
	for _, section := range source.Sections {
		for _, candidate := range section.Rungs {
			if candidate.Path == rungPath {
				copy := candidate
				rung = &copy
			}
		}
	}
	if rung == nil || len(rung.Instructions) != 0 || len(rung.Graphic) != 7 {
		return nil, fbError(rungPath, 0, "expected the seven-cell graphical call layout")
	}
	// Read the original object too: version/count/flags must not be lost through
	// the general inventory, nor may a duplicate cell number be accepted.
	fields := doc.Fields
	var object Field
	for _, part := range strings.Split(rungPath, ".") {
		object, err = sourceField(fields, part, "object", rungPath)
		if err != nil {
			return nil, err
		}
		fields = object.Fields
	}
	for _, pair := range [][2]string{{"TotEl", "7"}, {"Flags", `"0,0"`}} {
		if err := sourceValue(fields, pair[0], pair[1], rungPath); err != nil {
			return nil, err
		}
	}
	version, err := sourceField(fields, "FBversion", "scalar", rungPath)
	if err != nil || version.Text() == "" {
		return nil, fbError(rungPath, object.Line, "missing call version")
	}
	seenCells := map[string]bool{}
	type pinCell struct {
		row   int
		field Field
		value string
	}
	var inputs, outputs []pinCell
	var enable Field
	var enableSource, instanceName string
	callLine := 0
	wires := 0
	for _, cell := range rung.Graphic {
		if cell.Kind != "scalar" || seenCells[cell.Name] {
			return nil, fbError(rungPath, cell.Line, "duplicate or non-scalar cell")
		}
		seenCells[cell.Name] = true
		validName := false
		for i := 0; i < 7; i++ {
			validName = validName || cell.Name == fmt.Sprintf("[%d]", i)
		}
		if !validName {
			return nil, fbError(rungPath, cell.Line, "invalid cell index")
		}
		c := strings.Split(cell.Text(), "\x11")
		if len(c) == 3 && strings.Join(c, ",") == "1,1,3" {
			wires++
			continue
		}
		if len(c) != 8 || c[3] != "1" || c[4] != "1" || c[6] == "" {
			return nil, fbError(rungPath, cell.Line, "unsupported cell shape")
		}
		switch c[5] {
		case "LD":
			if enableSource != "" || c[0] != "0" || c[1] != "1" || c[2] != "1" || c[7] != "1" {
				return nil, fbError(rungPath, cell.Line, "unsupported enable")
			}
			enable, enableSource = cell, c[6]
		case "FBCALL":
			if instanceName != "" || c[0] != "2" || c[1] != "0" || c[2] != "7" || c[7] != "9" {
				return nil, fbError(rungPath, cell.Line, "unsupported call")
			}
			instanceName = c[6]
			callLine = cell.Line
		case "FB_INPUT", "FB_OUTPUT":
			row, e := fbNumber(c[1], 4)
			if e != nil || row < 2 || c[2] != "8" {
				return nil, fbError(rungPath, cell.Line, "unsupported pin row")
			}
			if c[5] == "FB_INPUT" && c[0] == "1" && c[7] == "10" {
				inputs = append(inputs, pinCell{row, cell, c[6]})
			} else if c[5] == "FB_OUTPUT" && c[0] == "4" && c[7] == "11" {
				outputs = append(outputs, pinCell{row, cell, c[6]})
			} else {
				return nil, fbError(rungPath, cell.Line, "unsupported pin side")
			}
		default:
			return nil, fbError(rungPath, cell.Line, "unsupported cell operation")
		}
	}
	if enableSource == "" || instanceName == "" || wires != 1 || len(inputs) != 3 || len(outputs) != 1 {
		return nil, fbError(rungPath, object.Line, "incomplete call")
	}
	sort.Slice(inputs, func(i, j int) bool { return inputs[i].row < inputs[j].row })
	for i, p := range inputs {
		if p.row != i+2 {
			return nil, fbError(rungPath, p.field.Line, "missing or duplicate input row")
		}
	}
	if outputs[0].row != 2 {
		return nil, fbError(rungPath, outputs[0].field.Line, "unsupported output row")
	}
	tables, err := doc.SymbolsForProgram(program)
	if err != nil {
		return nil, err
	}
	folded := map[string]string{}
	for _, scope := range [][]Symbol{tables.Globals, tables.Locals} {
		for _, symbol := range scope {
			if symbol.Name == "" {
				continue
			}
			key := strings.ToUpper(symbol.Name)
			if previous, ok := folded[key]; ok && previous != symbol.Name {
				return nil, fbError(program, symbol.Line, "case-ambiguous symbol names")
			}
			if fbDirectOperand.MatchString(symbol.Name) {
				return nil, fbError(program, symbol.Line, "symbol collides with native operand syntax")
			}
			folded[key] = symbol.Name
		}
	}
	lookup := func(name string, line int) (*Symbol, error) {
		for _, scope := range [][]Symbol{tables.Locals, tables.Globals} {
			for _, symbol := range scope {
				if symbol.Name == name {
					copy := symbol
					return &copy, nil
				}
			}
		}
		return nil, fbError(rungPath, line, "unknown symbol "+name)
	}
	bind := func(value string, field Field) (FBCallOperand, error) {
		if fbDirectOperand.MatchString(value) {
			return FBCallOperand{Source: value, Operand: value, Line: field.Line}, nil
		}
		symbol, e := lookup(value, field.Line)
		if e != nil {
			return FBCallOperand{}, e
		}
		if symbol.Flags != "" || symbol.AuxType != "" || strings.TrimSpace(symbol.Address) == "" {
			return FBCallOperand{}, fbError(rungPath, field.Line, "unqualified scalar binding")
		}
		return FBCallOperand{value, symbol.Address, field.Line, symbol}, nil
	}
	instanceSymbol, err := lookup(instanceName, object.Line)
	if err != nil {
		return nil, err
	}
	if instanceSymbol.DataType != "FUNCTION BLOCK" || instanceSymbol.Flags != "A" || strings.TrimSpace(instanceSymbol.Address) != "[Auto]" || instanceSymbol.AuxType == "" {
		return nil, fbError(rungPath, instanceSymbol.Line, "unqualified instance declaration")
	}
	resourceName := programScopePattern.FindStringSubmatch(program)[1]
	if instanceSymbol.Scope != resourceName {
		return nil, fbError(program, instanceSymbol.Line, "local FB instance records are not qualified")
	}
	resource, err := sourceField(doc.Fields, resourceName, "object", program)
	if err != nil {
		return nil, err
	}
	global, err := sourceField(resource.Fields, "GlobalVariables", "object", program)
	if err != nil {
		return nil, err
	}
	instances, err := sourceField(global.Fields, "POUInstances", "object", program)
	if err != nil {
		return nil, err
	}
	instanceRows, err := sourceChildren(instances, "NumInstances", "Instance", program)
	if err != nil {
		return nil, err
	}
	instanceNumber := -1
	seenNumbers := map[int]bool{}
	for _, row := range instanceRows {
		name, e1 := sourceField(row.Fields, "Name", "scalar", program)
		typeName, e2 := sourceField(row.Fields, "POUType", "scalar", program)
		number, e3 := sourceField(row.Fields, "InstanceNum", "scalar", program)
		n, e4 := fbNumber(number.Value, 65535)
		if e1 != nil || e2 != nil || e3 != nil || e4 != nil || seenNumbers[n] {
			return nil, fbError(program, row.Line, "invalid instance record")
		}
		seenNumbers[n] = true
		if name.Text() == instanceName {
			if instanceNumber >= 0 || typeName.Text() != instanceSymbol.AuxType {
				return nil, fbError(program, row.Line, "ambiguous instance type")
			}
			if e := sourceValue(row.Fields, "InstanceType", `"FB"`, program); e != nil {
				return nil, e
			}
			instanceNumber = n
		}
	}
	if instanceNumber < 0 {
		return nil, fbError(rungPath, 0, "missing saved instance")
	}
	blocks, err := sourceField(resource.Fields, "FunctionBlocks", "object", program)
	if err != nil {
		return nil, err
	}
	wrappers, err := sourceChildren(blocks, "FunctionBlockCount", "FunctionBlockWrapper", program)
	if err != nil {
		return nil, err
	}
	plan := &FBCallPlan{Program: program, Rung: rungPath, CallLine: callLine, Instance: instanceName, InstanceNumber: instanceNumber, Definition: instanceSymbol.AuxType, Version: version.Text()}
	for _, wrapper := range wrappers {
		block, e := sourceField(wrapper.Fields, "FunctionBlock", "object", program)
		if e != nil {
			return nil, e
		}
		name, e := sourceField(block.Fields, "Name", "scalar", program)
		if e != nil {
			return nil, e
		}
		if name.Text() != plan.Definition {
			continue
		}
		if plan.DefinitionPath != "" {
			return nil, fbError(program, name.Line, "duplicate definition")
		}
		path := resourceName + ".FunctionBlocks." + wrapper.Name + ".FunctionBlock"
		for _, pair := range [][2]string{{"FileType", "3"}, {"CompileInline", "0"}, {"IEC1131Type", "3"}} {
			if e := sourceValue(block.Fields, pair[0], pair[1], path); e != nil {
				return nil, e
			}
		}
		if e := sourceValue(wrapper.Fields, "FBType", `"FBLadder"`, path); e != nil {
			return nil, e
		}
		header, e := sourceField(block.Fields, "POUHeader", "object", path)
		if e != nil {
			return nil, e
		}
		variables, v, e := fbVariables(header, path)
		if e != nil {
			return nil, e
		}
		if v != plan.Version {
			return nil, fbError(path, header.Line, "stale call version")
		}
		task, e := sourceField(block.Fields, "TaskID", "scalar", path)
		if e != nil {
			return nil, e
		}
		id, e := fbNumber(task.Value, 65535)
		if e != nil {
			return nil, fbError(path, task.Line, "invalid definition task ID")
		}
		plan.DefinitionPath, plan.TaskID, plan.Variables = path, id, variables
	}
	if plan.DefinitionPath == "" {
		return nil, fbError(rungPath, 0, "missing definition")
	}
	plan.Enable, err = bind(enableSource, enable)
	if err != nil {
		return nil, err
	}
	for _, variable := range plan.Variables {
		if variable.Usage == 0 {
			continue
		}
		if variable.RemainingColumns[0] != "" || variable.RemainingColumns[1] != "0" || variable.RemainingColumns[3] != "0" {
			return nil, fbError(plan.DefinitionPath, variable.Line, "unsupported parameter metadata")
		}
		list, target := inputs, &plan.Inputs
		if variable.Usage == 2 {
			list, target = outputs, &plan.Outputs
		}
		if len(*target) >= len(list) {
			return nil, fbError(rungPath, variable.Line, "declaration/pin count mismatch")
		}
		p := list[len(*target)]
		value, e := bind(p.value, p.field)
		if e != nil {
			return nil, e
		}
		*target = append(*target, FBCallPin{variable, value, p.row})
	}
	if len(plan.Inputs) != len(inputs) || len(plan.Outputs) != len(outputs) {
		return nil, fbError(rungPath, 0, "declaration/pin count mismatch")
	}
	return plan, nil
}
