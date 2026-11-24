package cxp

import "regexp"

type TaskSource struct {
	Path           string `json:"path"`
	Name           string `json:"name"`
	Number         int    `json:"number"`
	OperationStart int    `json:"operationStart"`
}

var taskName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,23}$`)

// SourceForTask reads the observed unprotected task metadata. Only IDs256..287,
// OpStart0/1 and ASCII identifier names1..24 bytes are qualified. Other task
// classes and settings need separate configuration/prologue qualification.
func (doc *Document) SourceForTask(path string) (*TaskSource, error) {
	if _, err := doc.SourceForProgram(path); err != nil {
		return nil, err
	}
	match := programScopePattern.FindStringSubmatch(path)
	resource, err := sourceField(doc.Fields, match[1], "object", path)
	if err != nil {
		return nil, err
	}
	programs, err := sourceField(resource.Fields, "Programs", "object", path)
	if err != nil {
		return nil, err
	}
	program, err := sourceField(programs.Fields, match[2], "object", path)
	if err != nil {
		return nil, err
	}
	name, err := sourceField(program.Fields, "Name", "scalar", path)
	if err != nil {
		return nil, err
	}
	if !taskName.MatchString(name.Text()) {
		return nil, sourceError(path, name.Line, "task name requires an ASCII identifier of1..24 bytes")
	}
	id, err := sourceField(program.Fields, "TaskID", "scalar", path)
	if err != nil {
		return nil, err
	}
	number, err := fbNumber(id.Value, 287)
	if err != nil || number < 256 {
		return nil, sourceError(path, id.Line, "only task IDs256..287 are qualified")
	}
	start, err := sourceField(program.Fields, "OpStart", "scalar", path)
	if err != nil {
		return nil, err
	}
	startup, err := fbNumber(start.Value, 1)
	if err != nil {
		return nil, sourceError(path, start.Line, "startup must be canonical0 or1")
	}
	protection, err := sourceField(program.Fields, "Protection", "object", path)
	if err != nil {
		return nil, err
	}
	if len(protection.Fields) != 1 {
		return nil, sourceError(path, protection.Line, "unsupported task protection layout")
	}
	if err := sourceValue(protection.Fields, "Enabled", "0", path); err != nil {
		return nil, err
	}
	return &TaskSource{Path: path, Name: name.Text(), Number: number, OperationStart: startup}, nil
}
