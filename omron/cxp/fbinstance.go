package cxp

import (
	"fmt"
	"regexp"
	"strings"
)

type FBInstanceBase struct {
	Address string `json:"address"`
	Size    int    `json:"size"`
}
type FBInstanceSource struct {
	Path           string            `json:"path"`
	Name           string            `json:"name"`
	Number         int               `json:"number"`
	Definition     string            `json:"definition"`
	DefinitionPath string            `json:"definitionPath"`
	Bases          [5]FBInstanceBase `json:"bases"`
}

var fbInstancePath = regexp.MustCompile(`^(Resource\[[0-9]+\])\.GlobalVariables\.POUInstances\.(Instance\[[0-9]+\])$`)

// SourceForFBInstance reads one already allocated global instance. It neither
// chooses absolute addresses nor proves that other instances do not overlap it.
func (doc *Document) SourceForFBInstance(path string) (*FBInstanceSource, error) {
	match := fbInstancePath.FindStringSubmatch(path)
	if doc == nil || len(match) != 3 {
		return nil, fbError(path, 0, "invalid global instance scope")
	}
	resource, err := sourceField(doc.Fields, match[1], "object", path)
	if err != nil {
		return nil, err
	}
	global, err := sourceField(resource.Fields, "GlobalVariables", "object", path)
	if err != nil {
		return nil, err
	}
	instances, err := sourceField(global.Fields, "POUInstances", "object", path)
	if err != nil {
		return nil, err
	}
	rows, err := sourceChildren(instances, "NumInstances", "Instance", path)
	if err != nil {
		return nil, err
	}
	var selected Field
	names, numbers := map[string]bool{}, map[int]bool{}
	for _, row := range rows {
		name, e := sourceField(row.Fields, "Name", "scalar", path)
		if e != nil {
			return nil, e
		}
		number, e := sourceField(row.Fields, "InstanceNum", "scalar", path)
		if e != nil {
			return nil, e
		}
		id, e := fbNumber(number.Value, 2047)
		if e != nil || names[strings.ToUpper(name.Text())] || numbers[id] {
			return nil, fbError(path, row.Line, "duplicate/invalid instance identity")
		}
		names[strings.ToUpper(name.Text())] = true
		numbers[id] = true
		if row.Name == match[2] {
			selected = row
		}
	}
	if selected.Name == "" {
		return nil, fbError(path, 0, "instance not declared")
	}
	for _, field := range selected.Fields {
		switch field.Name {
		case "InstanceType", "Name", "POUType", "InstanceNum", "OnlineEditDiffExpand", "BR0", "BR1", "BR2", "BR3", "BR4", "MemoryAllocated":
		default:
			return nil, fbError(path, field.Line, "unsupported instance field "+field.Name)
		}
	}
	for _, pair := range [][2]string{{"InstanceType", `"FB"`}, {"OnlineEditDiffExpand", "0"}, {"MemoryAllocated", "true"}} {
		if err := sourceValue(selected.Fields, pair[0], pair[1], path); err != nil {
			return nil, err
		}
	}
	name, err := sourceField(selected.Fields, "Name", "scalar", path)
	if err != nil {
		return nil, err
	}
	definition, err := sourceField(selected.Fields, "POUType", "scalar", path)
	if err != nil {
		return nil, err
	}
	number, err := sourceField(selected.Fields, "InstanceNum", "scalar", path)
	if err != nil {
		return nil, err
	}
	id, _ := fbNumber(number.Value, 2047)
	result := &FBInstanceSource{Path: path, Name: name.Text(), Number: id, Definition: definition.Text()}
	for base := 0; base < 5; base++ {
		field, e := sourceField(selected.Fields, fmt.Sprintf("BR%d", base), "scalar", path)
		if e != nil {
			return nil, e
		}
		parts := strings.Split(field.Text(), ",")
		if len(parts) != 2 {
			return nil, fbError(path, field.Line, "invalid base binding")
		}
		size, e := fbNumber(strings.TrimSpace(parts[1]), 2048)
		if e != nil {
			return nil, fbError(path, field.Line, "invalid base size")
		}
		result.Bases[base] = FBInstanceBase{Address: strings.TrimSpace(parts[0]), Size: size}
	}
	blocks, err := sourceField(resource.Fields, "FunctionBlocks", "object", path)
	if err != nil {
		return nil, err
	}
	wrappers, err := sourceChildren(blocks, "FunctionBlockCount", "FunctionBlockWrapper", path)
	if err != nil {
		return nil, err
	}
	for _, wrapper := range wrappers {
		block, e := sourceField(wrapper.Fields, "FunctionBlock", "object", path)
		if e != nil {
			return nil, e
		}
		n, e := sourceField(block.Fields, "Name", "scalar", path)
		if e != nil {
			return nil, e
		}
		if n.Text() == result.Definition {
			if result.DefinitionPath != "" {
				return nil, fbError(path, n.Line, "ambiguous definition")
			}
			result.DefinitionPath = match[1] + ".FunctionBlocks." + wrapper.Name + ".FunctionBlock"
		}
	}
	if result.Name == "" || result.DefinitionPath == "" {
		return nil, fbError(path, selected.Line, "unresolved instance definition/name")
	}
	return result, nil
}
