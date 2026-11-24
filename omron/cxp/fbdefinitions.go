package cxp

import "strings"

// SourceForFBDefinitions reads all definitions in source order, requiring unique
// names and body TaskIDs. It does not assign N1 record numbers from those IDs.
func (doc *Document) SourceForFBDefinitions(resourcePath string) ([]*FBDefinitionSource, error) {
	if doc == nil || !fbResourcePath.MatchString(resourcePath) {
		return nil, definitionError(resourcePath, 0, "invalid resource scope")
	}
	resource, err := sourceField(doc.Fields, resourcePath, "object", resourcePath)
	if err != nil {
		return nil, err
	}
	blocks, err := sourceField(resource.Fields, "FunctionBlocks", "object", resourcePath)
	if err != nil {
		return nil, err
	}
	wrappers, err := sourceChildren(blocks, "FunctionBlockCount", "FunctionBlockWrapper", resourcePath)
	if err != nil {
		return nil, err
	}
	if len(wrappers) > 1024 {
		return nil, definitionError(resourcePath, blocks.Line, "definition number capacity exceeded")
	}
	result := make([]*FBDefinitionSource, 0, len(wrappers))
	names, bodies := map[string]bool{}, map[int]bool{}
	for _, wrapper := range wrappers {
		source, err := doc.SourceForFBDefinition(resourcePath + ".FunctionBlocks." + wrapper.Name + ".FunctionBlock")
		if err != nil {
			return nil, err
		}
		key := strings.ToUpper(source.Name)
		if names[key] || bodies[source.TaskID] {
			return nil, definitionError(source.Path, wrapper.Line, "ambiguous definition name or body TaskID")
		}
		names[key], bodies[source.TaskID] = true, true
		result = append(result, source)
	}
	return result, nil
}
