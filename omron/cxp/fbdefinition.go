package cxp

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var ErrFBDefinition = errors.New("unsupported or malformed CXP FB definition")

// FBReference is a typed symbolic reference, not an allocated relative address.
type FBReference struct {
	Name            string `json:"name"`
	TypeCode        int    `json:"typeCode"`
	TypeName        string `json:"typeName"`
	DeclarationLine int    `json:"declarationLine,omitempty"`
	Implicit        bool   `json:"implicit,omitempty"`
}

type FBInstruction struct {
	Mnemonic string        `json:"mnemonic"`
	Spelling string        `json:"spelling"`
	Operands []FBReference `json:"operands"`
	Line     int           `json:"line"`
	Column   int           `json:"column"`
}

type FBDefinitionRung struct {
	Path         string          `json:"path"`
	Comment      string          `json:"comment,omitempty"`
	Instructions []FBInstruction `json:"instructions"`
}

// FBDefinitionSource describes the observed linear CSV-cell ladder format.
// It does not allocate variables/timers or generate native relative operands.
type FBDefinitionSource struct {
	Path      string             `json:"path"`
	Name      string             `json:"name"`
	TaskID    int                `json:"taskID"`
	Version   string             `json:"version"`
	Variables []FBVariable       `json:"variables"`
	Rungs     []FBDefinitionRung `json:"rungs"`
}

var fbDefinitionPath = regexp.MustCompile(`^(Resource\[[0-9]+\])\.FunctionBlocks\.(FunctionBlockWrapper\[[0-9]+\])\.FunctionBlock$`)

func definitionError(path string, line int, message string) error {
	return fmt.Errorf("%w: %s line %d: %s", ErrFBDefinition, path, line, message)
}

// SourceForFBDefinition lowers only seven-column series networks. Contacts are
// ordered by X coordinates; neither cell storage order nor wire gaps may change
// evaluation order. Parallel paths, disabled rungs and other forms fail closed.
func (doc *Document) SourceForFBDefinition(path string) (*FBDefinitionSource, error) {
	result, err := doc.sourceForFBDefinition(path)
	if err != nil && !errors.Is(err, ErrFBDefinition) {
		return nil, fmt.Errorf("%w: %w", ErrFBDefinition, err)
	}
	return result, err
}

func (doc *Document) sourceForFBDefinition(path string) (*FBDefinitionSource, error) {
	wrapper, err := doc.definitionWrapper(path)
	if err != nil {
		return nil, err
	}
	if err := sourceValue(wrapper.Fields, "FBType", `"FBLadder"`, path); err != nil {
		return nil, err
	}
	block, err := sourceField(wrapper.Fields, "FunctionBlock", "object", path)
	if err != nil {
		return nil, err
	}
	for _, field := range wrapper.Fields {
		if field.Name != "FBType" && field.Name != "FunctionBlock" {
			return nil, definitionError(path, field.Line, "unsupported wrapper field "+field.Name)
		}
	}
	seenFields := map[string]bool{}
	for _, field := range block.Fields {
		if seenFields[field.Name] {
			return nil, definitionError(path, field.Line, "duplicate definition field "+field.Name)
		}
		seenFields[field.Name] = true
		switch field.Name {
		case "IEC1131Type", "CompileInline", "Name", "FileType", "Created", "Modified", "Protection", "FBDefVer", "OnlineReserveMemory", "POUHeader", "POUBody", "TaskID":
		default:
			return nil, definitionError(path, field.Line, "unsupported definition field "+field.Name)
		}
	}
	for _, pair := range [][2]string{{"IEC1131Type", "3"}, {"CompileInline", "0"}, {"FileType", "3"}} {
		if err := sourceValue(block.Fields, pair[0], pair[1], path); err != nil {
			return nil, err
		}
	}
	if err := checkFBReserve(block, path); err != nil {
		return nil, err
	}
	name, err := sourceField(block.Fields, "Name", "scalar", path)
	if err != nil {
		return nil, err
	}
	if name.Text() == "" {
		return nil, definitionError(path, name.Line, "empty definition name")
	}
	header, err := sourceField(block.Fields, "POUHeader", "object", path)
	if err != nil {
		return nil, err
	}
	format, _ := block.Get("FBDefVer")
	format20 := format.Text() == "2.0"
	variables, version, err := fbVariables(header, path+".POUHeader", "FBLadder", format20)
	if err != nil {
		return nil, err
	}
	task, err := sourceField(block.Fields, "TaskID", "scalar", path)
	if err != nil {
		return nil, err
	}
	id, err := fbNumber(task.Value, 65535)
	if err != nil {
		return nil, definitionError(path, task.Line, "invalid task ID")
	}
	refs := map[string]FBReference{"EN": {Name: "EN", TypeCode: 0, TypeName: "BOOL", Implicit: true}, "ENO": {Name: "ENO", TypeCode: 0, TypeName: "BOOL", Implicit: true}}
	for _, variable := range variables {
		if variable.RemainingColumns[0] != "" || variable.RemainingColumns[1] != "0" || variable.RemainingColumns[3] != "0" {
			return nil, definitionError(path, variable.Line, "unsupported variable metadata")
		}
		refs[variable.Name] = FBReference{variable.Name, variable.TypeCode, variable.TypeName, variable.Line, false}
	}
	body, err := sourceField(block.Fields, "POUBody", "object", path)
	if err != nil {
		return nil, err
	}
	if len(body.Fields) != 1 {
		return nil, definitionError(path, body.Line, "unsupported body fields")
	}
	data, err := sourceField(body.Fields, "FBProgramData", "object", path)
	if err != nil {
		return nil, err
	}
	rungs, err := sourceChildren(data, "FBRC", "FBR", path+".POUBody.FBProgramData")
	if err != nil {
		return nil, err
	}
	if len(rungs) == 0 {
		return nil, definitionError(path, data.Line, "empty definition")
	}
	result := &FBDefinitionSource{Path: path, Name: name.Text(), TaskID: id, Version: version, Variables: variables}
	for _, rung := range rungs {
		p := path + ".POUBody.FBProgramData." + rung.Name
		parsed, err := readFBSeriesRung(rung, p, refs, format20)
		if err != nil {
			return nil, err
		}
		result.Rungs = append(result.Rungs, *parsed)
	}
	return result, nil
}

func readFBSeriesRung(rung Field, path string, refs map[string]FBReference, format20 bool) (*FBDefinitionRung, error) {
	for _, pair := range [][2]string{{"Flags", `"1,0"`}, {"TotEl", "7"}} {
		if err := sourceValue(rung.Fields, pair[0], pair[1], path); err != nil {
			return nil, err
		}
	}
	comment, err := sourceField(rung.Fields, "Com", "scalar", path)
	if err != nil {
		return nil, err
	}
	var cells [7]Field
	var columns [7][]string
	indices := map[string]bool{}
	for _, field := range rung.Fields {
		if field.Name == "Com" || field.Name == "Flags" || field.Name == "TotEl" {
			continue
		}
		valid := false
		for i := 0; i < 7; i++ {
			valid = valid || field.Name == fmt.Sprintf("[%d]", i)
		}
		if !valid || field.Kind != "scalar" || indices[field.Name] {
			return nil, definitionError(path, field.Line, "unknown or duplicate graphical field")
		}
		indices[field.Name] = true
		c := strings.Split(field.Text(), ",")
		for i := range c {
			c[i] = strings.TrimSpace(c[i])
		}
		if format20 {
			if len(c) < 7 || c[4] != "0" {
				return nil, definitionError(path, field.Line, "unsupported FBDefVer 2.0 cell")
			}
			c = append(c[:4], c[5:]...)
		}
		if len(c) < 6 {
			return nil, definitionError(path, field.Line, "truncated CSV cell")
		}
		x, e := fbNumber(c[0], 6)
		if e != nil || cells[x].Name != "" || c[1] != "0" || c[2] != "1" {
			return nil, definitionError(path, field.Line, "unsupported cell coordinates/width")
		}
		cells[x], columns[x] = field, c
	}
	if len(indices) != 7 {
		return nil, definitionError(path, rung.Line, "cell count mismatch")
	}
	lookup := func(name string, line int, types ...int) (FBReference, error) {
		r, ok := refs[name]
		if !ok {
			return FBReference{}, definitionError(path, line, "unknown variable "+name)
		}
		for _, code := range types {
			if r.TypeCode == code {
				return r, nil
			}
		}
		return FBReference{}, definitionError(path, line, "unsupported variable role/type for "+name)
	}
	result := &FBDefinitionRung{Path: path, Comment: comment.Text()}
	wireSeen := false
	for x := 0; x < 6; x++ {
		c, field := columns[x], cells[x]
		if len(c) == 6 && c[3] == "1" && c[4] == "0" && c[5] == "3" {
			wireSeen = true
			continue
		}
		if wireSeen || len(c) != 8 || c[3] != "1" || c[4] != "1" || c[5] != "1" || (c[7] != "0" && c[7] != "8") {
			return nil, definitionError(path, field.Line, "unsupported contact or contact after wire gap")
		}
		r, err := lookup(c[6], field.Line, 0, 12, 13)
		if err != nil {
			return nil, err
		}
		mnemonic := "AND"
		if x == 0 {
			mnemonic = "LD"
		}
		if c[7] == "8" {
			mnemonic += "NOT"
		}
		result.Instructions = append(result.Instructions, FBInstruction{mnemonic, mnemonic, []FBReference{r}, field.Line, x})
	}
	if len(result.Instructions) == 0 {
		return nil, definitionError(path, rung.Line, "series network requires a leading condition")
	}
	c, field := columns[6], cells[6]
	if len(c) == 8 && c[3] == "1" && c[4] == "1" && c[5] == "2" && c[7] == "0" {
		r, err := lookup(c[6], field.Line, 0)
		if err != nil {
			return nil, err
		}
		result.Instructions = append(result.Instructions, FBInstruction{"OUT", "OUT", []FBReference{r}, field.Line, 6})
		return result, nil
	}
	if c[3] != "0" || c[5] != "5" || len(c) < 10 || c[7] != "0" {
		return nil, definitionError(path, field.Line, "unsupported terminal cell")
	}
	var mnemonic string
	var operands []FBReference
	switch c[6] {
	case "SET", "RSET":
		if len(c) != 10 || c[4] != "2" || c[9] != "0" {
			return nil, definitionError(path, field.Line, "unsupported set/reset cell")
		}
		r, err := lookup(c[8], field.Line, 0)
		if err != nil {
			return nil, err
		}
		operands = []FBReference{r}
		mnemonic = c[6]
	case "TIMH(015)":
		if len(c) != 12 || c[4] != "3" || c[9] != "0" || c[11] != "0" {
			return nil, definitionError(path, field.Line, "unsupported timer cell")
		}
		timer, err := lookup(c[8], field.Line, 13)
		if err != nil {
			return nil, err
		}
		preset, err := lookup(c[10], field.Line, 1, 2, 7)
		if err != nil {
			return nil, err
		}
		operands = []FBReference{timer, preset}
		mnemonic = "TIMH"
	default:
		return nil, definitionError(path, field.Line, "unsupported terminal instruction "+c[6])
	}
	result.Instructions = append(result.Instructions, FBInstruction{mnemonic, c[6], operands, field.Line, 6})
	return result, nil
}

func (doc *Document) definitionWrapper(path string) (*Field, error) {
	match := fbDefinitionPath.FindStringSubmatch(path)
	if doc == nil || len(match) != 3 {
		return nil, definitionError(path, 0, "invalid definition scope")
	}
	resource, err := sourceField(doc.Fields, match[1], "object", path)
	if err != nil {
		return nil, err
	}
	blocks, err := sourceField(resource.Fields, "FunctionBlocks", "object", path)
	if err != nil {
		return nil, err
	}
	wrappers, err := sourceChildren(blocks, "FunctionBlockCount", "FunctionBlockWrapper", path)
	if err != nil {
		return nil, err
	}
	var wrapper Field
	for _, candidate := range wrappers {
		if candidate.Name == match[2] {
			wrapper = candidate
		}
	}
	if wrapper.Name == "" {
		return nil, definitionError(path, blocks.Line, "definition not declared")
	}
	return &wrapper, nil
}

func checkFBReserve(block Field, path string) error {
	reserve, err := sourceField(block.Fields, "OnlineReserveMemory", "object", path)
	if err != nil {
		return err
	}
	if len(reserve.Fields) != 6 {
		return definitionError(path, reserve.Line, "unsupported online reserve layout")
	}
	if err := sourceValue(reserve.Fields, "NumRegisters", "5", path); err != nil {
		return err
	}
	for base := 0; base < 5; base++ {
		if err := sourceValue(reserve.Fields, fmt.Sprintf("BR[%d]", base), `"0,0"`, path); err != nil {
			return err
		}
	}
	return nil
}
