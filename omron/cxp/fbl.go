package cxp

import (
	"bytes"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// BuildFBL creates the offline SRCPRGRM.FBL source archive for the observed
// single-definition, allocated ladder layout. When modified is empty it
// preserves the date in the CXP. An explicit CX timestamp is useful for
// reproducing a capture, since CX updates Modified during transfer.
func (doc *Document) BuildFBL(modified string) ([]byte, error) {
	return doc.buildFBL(modified, 3)
}

// BuildProgramsOnlyFBL builds the source archive with the list numbering
// observed in the CPU65H Programs-only transfer.
func (doc *Document) BuildProgramsOnlyFBL(modified string) ([]byte, error) {
	return doc.buildFBL(modified, 1)
}

func (doc *Document) buildFBL(modified string, firstList int) ([]byte, error) {
	const resourcePath = "Resource[0]"
	if doc == nil {
		return nil, fmt.Errorf("FBL: missing source document")
	}
	if err := sourceValue(doc.Fields, "CXProgVer", `"1.8"`, resourcePath); err != nil {
		return nil, err
	}
	if err := sourceValue(doc.Fields, "ResourceCount", "1", resourcePath); err != nil {
		return nil, err
	}
	definitions, err := doc.SourceForFBDefinitions(resourcePath)
	if err != nil {
		return nil, err
	}
	instances, err := doc.SourceForFBInstances(resourcePath)
	if err != nil {
		return nil, err
	}
	if len(definitions) != 1 || len(instances) == 0 || len(instances) > 2 {
		return nil, fmt.Errorf("FBL requires one ladder definition and one or two allocated instances")
	}
	resource, err := sourceField(doc.Fields, resourcePath, "object", resourcePath)
	if err != nil {
		return nil, err
	}
	name, err := sourceField(resource.Fields, "Name", "scalar", resourcePath)
	if err != nil {
		return nil, err
	}
	memory, err := sourceField(resource.Fields, "FBMemory", "object", resourcePath)
	if err != nil {
		return nil, err
	}
	global, err := sourceField(resource.Fields, "GlobalVariables", "object", resourcePath)
	if err != nil {
		return nil, err
	}
	saved, err := sourceField(global.Fields, "POUInstances", "object", resourcePath)
	if err != nil {
		return nil, err
	}
	symbols, err := fblSymbols(global, len(instances), firstList)
	if err != nil {
		return nil, err
	}
	blocks, err := sourceField(resource.Fields, "FunctionBlocks", "object", resourcePath)
	if err != nil {
		return nil, err
	}
	blocks.Fields = slices.Clone(blocks.Fields)
	for i, wrapper := range blocks.Fields {
		if wrapper.Kind != "object" {
			continue
		}
		wrapper.Fields, err = fblDefinition(wrapper.Fields, modified, definitions[0].Path, firstList+1)
		if err != nil {
			return nil, err
		}
		blocks.Fields[i] = wrapper
	}
	project := Document{Encoding: "windows-1251", Fields: []Field{
		{Name: resourcePath, Kind: "object", Fields: []Field{name, memory, symbols, saved, blocks}},
	}}
	const header = "CXProgVer:=\"1.8\";\r\nName:=\"FB source program\";\nResourceCount:=1;\nResource[0]:=\nBEGIN\n"
	const end = "END;\n"
	text, err := project.marshalSource(MaxSize-len(header)-len(end), true)
	if err != nil {
		return nil, err
	}
	// CX uses LF for the source-file wrapper and CRLF for the resource body.
	text = bytes.TrimPrefix(text, []byte("Resource[0]:=\r\nBEGIN\r\n"))
	text = bytes.TrimSuffix(text, []byte("END;\r\n"))
	text = append(append([]byte(header), text...), end...)
	return packFBSource(text)
}

func fblSymbols(global Field, instances, listNumber int) (Field, error) {
	list, err := sourceField(global.Fields, "VariableList", "list", global.Name)
	if err != nil {
		return Field{}, err
	}
	rows, err := parseSymbolRows(list, global.Name)
	if err != nil {
		return Field{}, err
	}
	lines := strings.Split(list.Raw, "\n")
	var retained []string
	for _, row := range rows {
		if !strings.EqualFold(row.DataType, "FUNCTION BLOCK") {
			continue
		}
		line := lines[row.Line-list.Line-2]
		if !strings.HasSuffix(strings.TrimSpace(line), ";") {
			return Field{}, fmt.Errorf("FBL does not support multiline FB symbol comments")
		}
		retained = append(retained, "   "+strings.TrimSpace(line))
	}
	if len(retained) != instances {
		return Field{}, fmt.Errorf("FBL sources and allocated instances disagree")
	}
	list.Delimiter, list.Raw = fmt.Sprintf("BEGIN_LIST_$#[%d]", listNumber), strings.Join(retained, "\n")
	global.Fields = []Field{list, {Name: "VariableCount", Kind: "scalar", Value: strconv.Itoa(len(retained))}}
	return global, nil
}

func fblDefinition(fields []Field, modified, path string, listNumber int) ([]Field, error) {
	updated := slices.Clone(fields)
	for i, field := range updated {
		if field.Name != "FunctionBlock" {
			continue
		}
		field.Fields = slices.Clone(field.Fields)
		if _, err := sourceField(field.Fields, "Modified", "scalar", path); err != nil {
			return nil, err
		}
		for j, child := range field.Fields {
			switch child.Name {
			case "Modified":
				if modified != "" {
					stamp, err := time.Parse("2 1 2006 15 4 5", modified)
					if err != nil {
						return nil, fmt.Errorf("invalid FBL Modified timestamp: %w", err)
					}
					child.Value = fmt.Sprintf(`"%d %d %d %d %d %d"`, stamp.Day(), stamp.Month(), stamp.Year(), stamp.Hour(), stamp.Minute(), stamp.Second())
				}
			case "POUHeader":
				child.Fields = fblHeader(child.Fields, listNumber)
			case "POUBody":
				child.Fields = slices.Clone(child.Fields)
				data := &child.Fields[0] // SourceForFBDefinitions validated the sole FBProgramData field.
				var err error
				data.Fields, err = fblRungs(data.Fields, path)
				if err != nil {
					return nil, err
				}
			}
			field.Fields[j] = child
		}
		updated[i] = field
	}
	return updated, nil
}

func fblHeader(fields []Field, listNumber int) []Field {
	updated := slices.Clone(fields)
	for i, field := range updated {
		if field.Name != "PouVarList" {
			continue
		}
		field.Delimiter = fmt.Sprintf("BEGIN_LIST_$#[%d]", listNumber)
		lines := strings.Split(field.Raw, "\n")
		for n, line := range lines {
			lines[n] = "      " + strings.TrimSpace(line)
		}
		field.Raw = strings.Join(lines, "\n")
		updated[i] = field
	}
	return updated
}

func fblRungs(fields []Field, path string) ([]Field, error) {
	updated := slices.Clone(fields)
	for i, rung := range updated {
		if rung.Name == "FBRC" {
			continue
		}
		rung.Fields = slices.Clone(rung.Fields)
		var cells []int
		for n, cell := range rung.Fields {
			if !strings.HasPrefix(cell.Name, "[") {
				continue
			}
			if cell.Name != fmt.Sprintf("[%d]", len(cells)) {
				return nil, definitionError(path, cell.Line, "unexpected FBL cell order")
			}
			cells = append(cells, n)
		}
		// The CX source file emits the same cells in reverse storage order.
		for a, b := 0, len(cells)-1; a < b; a, b = a+1, b-1 {
			left, right := &rung.Fields[cells[a]], &rung.Fields[cells[b]]
			left.Value, right.Value = right.Value, left.Value
		}
		updated[i] = rung
	}
	return updated, nil
}
