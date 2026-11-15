package cxp

import "strings"

// SourceForTasks inventories every program in a resource and checks unique
// task IDs and case-folded names. It retains source order; native record order
// is chosen separately by the task block assembler.
func (doc *Document) SourceForTasks(resourcePath string) ([]*TaskSource, error) {
	if doc == nil || !fbResourcePath.MatchString(resourcePath) {
		return nil, sourceError(resourcePath, 0, "invalid resource scope")
	}
	resource, err := sourceField(doc.Fields, resourcePath, "object", resourcePath)
	if err != nil {
		return nil, err
	}
	programs, err := sourceField(resource.Fields, "Programs", "object", resourcePath)
	if err != nil {
		return nil, err
	}
	rows, err := sourceChildren(programs, "ProgramCount", "Program", resourcePath)
	if err != nil {
		return nil, err
	}
	if len(rows) > 32 {
		return nil, sourceError(resourcePath, programs.Line, "task count exceeds qualified ID range256..287")
	}
	result := make([]*TaskSource, 0, len(rows))
	ids, names := map[int]bool{}, map[string]bool{}
	for _, row := range rows {
		source, err := doc.SourceForTask(resourcePath + ".Programs." + row.Name)
		if err != nil {
			return nil, err
		}
		name := strings.ToUpper(source.Name)
		if ids[source.Number] || names[name] {
			return nil, sourceError(source.Path, row.Line, "duplicate task ID or case-ambiguous name")
		}
		ids[source.Number], names[name] = true, true
		result = append(result, source)
	}
	return result, nil
}
