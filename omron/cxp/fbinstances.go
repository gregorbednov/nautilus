package cxp

import (
	"regexp"
	"strings"
)

var fbResourcePath = regexp.MustCompile(`^Resource\[[0-9]+\]$`)

// SourceForFBInstances inventories every saved global instance in source order
// and checks the bijection with FUNCTION BLOCK symbols. Local FB instances and
// local names shadowing these globals are not supported. It does not audit
// scalar memory bindings or allocate absolute memory.
func (doc *Document) SourceForFBInstances(resourcePath string) ([]*FBInstanceSource, error) {
	if doc == nil || !fbResourcePath.MatchString(resourcePath) {
		return nil, fbError(resourcePath, 0, "invalid resource scope")
	}
	resource, err := sourceField(doc.Fields, resourcePath, "object", resourcePath)
	if err != nil {
		return nil, err
	}
	global, err := sourceField(resource.Fields, "GlobalVariables", "object", resourcePath)
	if err != nil {
		return nil, err
	}
	instances, err := sourceField(global.Fields, "POUInstances", "object", resourcePath)
	if err != nil {
		return nil, err
	}
	rows, err := sourceChildren(instances, "NumInstances", "Instance", resourcePath)
	if err != nil {
		return nil, err
	}
	if len(rows) > 2048 {
		return nil, fbError(resourcePath, instances.Line, "instance identity capacity exceeded")
	}
	blocks, err := sourceField(resource.Fields, "FunctionBlocks", "object", resourcePath)
	if err != nil {
		return nil, err
	}
	wrappers, err := sourceChildren(blocks, "FunctionBlockCount", "FunctionBlockWrapper", resourcePath)
	if err != nil {
		return nil, err
	}
	definitions := map[string]bool{}
	for _, wrapper := range wrappers {
		block, err := sourceField(wrapper.Fields, "FunctionBlock", "object", resourcePath)
		if err != nil {
			return nil, err
		}
		name, err := sourceField(block.Fields, "Name", "scalar", resourcePath)
		if err != nil {
			return nil, err
		}
		key := strings.ToUpper(name.Text())
		if key == "" || definitions[key] {
			return nil, fbError(resourcePath, name.Line, "ambiguous definition name")
		}
		definitions[key] = true
	}
	total := 0
	symbols, err := readSymbolScope(resource.Fields, "GlobalVariables", resourcePath, &total)
	if err != nil {
		return nil, err
	}
	byName := map[string]Symbol{}
	for _, symbol := range symbols {
		if symbol.Name == "" {
			continue
		}
		key := strings.ToUpper(symbol.Name)
		if _, exists := byName[key]; exists {
			return nil, fbError(resourcePath, symbol.Line, "case-ambiguous global symbol")
		}
		byName[key] = symbol
	}
	result := make([]*FBInstanceSource, 0, len(rows))
	matched := map[string]bool{}
	for _, row := range rows {
		instance, err := doc.SourceForFBInstance(resourcePath + ".GlobalVariables.POUInstances." + row.Name)
		if err != nil {
			return nil, err
		}
		key := strings.ToUpper(instance.Name)
		symbol, exists := byName[key]
		address := strings.TrimSpace(symbol.Address)
		if !exists || symbol.Name != instance.Name || symbol.DataType != "FUNCTION BLOCK" || symbol.Flags != "A" || symbol.AuxType != instance.Definition {
			return nil, fbError(instance.Path, row.Line, "saved instance disagrees with global symbol declaration")
		}
		if address != "[Auto]" {
			def, err := doc.SourceForFBDefinition(instance.DefinitionPath)
			if symbol.Address != "" || err != nil {
				return nil, fbError(instance.Path, row.Line, "unsupported FB instance address")
			}
			inout := len(def.Variables) == 1 && def.Variables[0].Usage == fbFileInOut && def.Variables[0].TypeCode == 0
			if !inout && !boolInputOutputPair(def.Variables) {
				return nil, fbError(instance.Path, row.Line, "unsupported FB instance address")
			}
		}
		matched[key] = true
		result = append(result, instance)
	}
	for _, symbol := range symbols {
		if strings.EqualFold(symbol.DataType, "FUNCTION BLOCK") && !matched[strings.ToUpper(symbol.Name)] {
			return nil, fbError(resourcePath, symbol.Line, "global FB symbol lacks a saved instance")
		}
	}
	programs, err := sourceField(resource.Fields, "Programs", "object", resourcePath)
	if err != nil {
		return nil, err
	}
	children, err := sourceChildren(programs, "ProgramCount", "Program", resourcePath)
	if err != nil {
		return nil, err
	}
	for _, program := range children {
		path := resourcePath + ".Programs." + program.Name
		locals, err := readSymbolScope(program.Fields, "LocalVariables", path, &total)
		if err != nil {
			return nil, err
		}
		for _, symbol := range locals {
			if strings.EqualFold(symbol.DataType, "FUNCTION BLOCK") || matched[strings.ToUpper(symbol.Name)] {
				return nil, fbError(path, symbol.Line, "local FB instances or shadowed global instances are not qualified")
			}
		}
		local, err := sourceField(program.Fields, "LocalVariables", "object", path)
		if err != nil {
			return nil, err
		}
		saved, err := sourceField(local.Fields, "POUInstances", "object", path)
		if err != nil {
			return nil, err
		}
		localRows, err := sourceChildren(saved, "NumInstances", "Instance", path)
		if err != nil {
			return nil, err
		}
		if len(localRows) != 0 {
			return nil, fbError(path, saved.Line, "local instance records are not qualified")
		}
	}
	return result, nil
}
