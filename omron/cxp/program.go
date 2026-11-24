package cxp

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var ErrProgramSource = errors.New("unsupported or malformed CXP program source")

type ProgramSourceError struct {
	Path    string
	Line    int
	Message string
}

func (e *ProgramSourceError) Error() string {
	return fmt.Sprintf("%s line %d: %s", e.Path, e.Line, e.Message)
}
func (e *ProgramSourceError) Unwrap() error { return ErrProgramSource }

// ProgramSource retains every declared section and rung, including empty ones.
// It describes source order, not native task scheduling or binary scaffolding.
type ProgramSource struct {
	Path     string
	Sections []ProgramSection
}
type ProgramSection struct {
	Path  string
	Name  string
	Line  int
	Rungs []Rung
}

func sourceError(path string, line int, message string) error {
	return &ProgramSourceError{path, line, message}
}

func sourceField(fields []Field, name, kind, path string) (Field, error) {
	var result Field
	count := 0
	for _, f := range fields {
		if f.Name == name {
			result = f
			count++
		}
	}
	if count != 1 || result.Kind != kind {
		return Field{}, sourceError(path, result.Line, "missing, duplicate or unsupported "+name)
	}
	return result, nil
}

func sourceValue(fields []Field, name, value, path string) error {
	f, err := sourceField(fields, name, "scalar", path)
	if err != nil {
		return err
	}
	if f.Value != value {
		return sourceError(path, f.Line, "unqualified "+name+" value")
	}
	return nil
}

func sourceChildren(parent Field, countName, prefix, path string) ([]Field, error) {
	// CX-Programmer can omit the counts for an empty FB list and a single task.
	switch countName {
	case "FunctionBlockCount":
		if len(parent.Fields) == 0 {
			return []Field{}, nil
		}
	case "ProgramCount":
		if len(parent.Fields) == 1 {
			child := parent.Fields[0]
			if child.Name == "Program[0]" && child.Kind == "object" {
				return []Field{child}, nil
			}
		}
	}
	f, err := sourceField(parent.Fields, countName, "scalar", path)
	if err != nil {
		return nil, err
	}
	n, err := strconv.Atoi(f.Value)
	if err != nil || n < 0 || n > 65536 || strconv.Itoa(n) != f.Value {
		return nil, sourceError(path, f.Line, "invalid "+countName)
	}
	children := make([]Field, 0, len(parent.Fields))
	for _, child := range parent.Fields {
		if child.Name == countName {
			continue
		}
		if child.Name != fmt.Sprintf("%s[%d]", prefix, len(children)) || child.Kind != "object" {
			return nil, sourceError(path, child.Line, "unexpected or out-of-order child "+child.Name)
		}
		children = append(children, child)
	}
	if len(children) != n {
		return nil, sourceError(path, f.Line, "declared child count mismatch")
	}
	return children, nil
}

// SourceForProgram validates the observed mnemonic-section layout rather than
// selecting the lossy inspection Summary. Unknown code-bearing fields fail.
// Graphics remain explicit so the compiler cannot silently omit an FB call.
func (doc *Document) SourceForProgram(path string) (*ProgramSource, error) {
	if doc == nil {
		return nil, sourceError(path, 0, "nil source document")
	}
	match := programScopePattern.FindStringSubmatch(path)
	if len(match) != 3 {
		return nil, sourceError(path, 0, "unsupported program scope")
	}
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
	if err = sourceValue(program.Fields, "FileType", "2", path); err != nil {
		return nil, err
	}
	if err = sourceValue(program.Fields, "CompileInline", "0", path); err != nil {
		return nil, err
	}
	for _, f := range program.Fields {
		switch f.Name {
		case "CompileInline", "Name", "FileType", "Created", "Modified", "Protection", "LocalVariables", "TaskID", "OpStart", "Sections":
		default:
			return nil, sourceError(path, f.Line, "unqualified program field "+f.Name)
		}
	}
	sections, err := sourceField(program.Fields, "Sections", "object", path)
	if err != nil {
		return nil, err
	}
	children, err := sourceChildren(sections, "SC", "Sec", path+".Sections")
	if err != nil {
		return nil, err
	}
	result := &ProgramSource{Path: path, Sections: []ProgramSection{}}
	total := 0
	textBytes, instructions := 0, 0
	for _, sec := range children {
		sectionPath := path + ".Sections." + sec.Name
		for _, pair := range [][2]string{{"IEC1131Type", "11"}, {"CompileInline", "0"}, {"SecType", "0"}} {
			if err = sourceValue(sec.Fields, pair[0], pair[1], sectionPath); err != nil {
				return nil, err
			}
		}
		for _, f := range sec.Fields {
			switch f.Name {
			case "IEC1131Type", "CompileInline", "SecType", "SecName", "ProgramData":
			default:
				return nil, sourceError(sectionPath, f.Line, "unqualified section field "+f.Name)
			}
		}
		name, err := sourceField(sec.Fields, "SecName", "scalar", sectionPath)
		if err != nil {
			return nil, err
		}
		data, err := sourceField(sec.Fields, "ProgramData", "object", sectionPath)
		if err != nil {
			return nil, err
		}
		rungs, err := sourceChildren(data, "RC", "R", sectionPath+".ProgramData")
		if err != nil {
			return nil, err
		}
		total += len(rungs)
		if total > 65536 {
			return nil, sourceError(sectionPath, data.Line, "program rung limit")
		}
		section := ProgramSection{Path: sectionPath, Name: name.Text(), Line: sec.Line, Rungs: []Rung{}}
		for _, r := range rungs {
			rungPath := sectionPath + ".ProgramData." + r.Name
			rung := Rung{Path: rungPath, Line: r.Line, Instructions: []Instruction{}}
			hasGraphic := false
			for _, f := range r.Fields {
				if strings.HasPrefix(f.Name, "[") {
					hasGraphic = true
				}
			}
			flags, flagsErr := sourceField(r.Fields, "Flags", "scalar", rungPath)
			if flagsErr != nil {
				return nil, flagsErr
			}
			if flags.Value != `"1,0"` && !(hasGraphic && flags.Value == `"0,0"`) {
				return nil, sourceError(rungPath, flags.Line, "unqualified Flags value")
			}
			slCount := 0
			for _, f := range r.Fields {
				switch f.Name {
				case "SL":
					slCount++
					if slCount > 1 || (f.Kind != "raw" && f.Kind != "scalar") {
						return nil, sourceError(rungPath, f.Line, "duplicate or unsupported SL")
					}
					source := f.Text()
					line := f.Line
					if f.Kind == "raw" {
						source = f.Raw
						line += 2
					}
					if len(source) > (8<<20)-textBytes {
						return nil, sourceError(rungPath, f.Line, "program source byte limit")
					}
					textBytes += len(source)
					for i, text := range strings.Split(source, "\n") {
						if strings.TrimSpace(text) != "" {
							instructions++
							if instructions > 65536 {
								return nil, sourceError(rungPath, line+i, "program instruction limit")
							}
							rung.Instructions = append(rung.Instructions, Instruction{Line: line + i, Text: strings.TrimSpace(text)})
						}
					}
				case "Com":
					rung.Comment = f.Text()
				case "Flags", "FBversion", "AtchCmts":
				default:
					if hasGraphic && (f.Name == "HasRCmts" || f.Name == "TotEl" || strings.HasPrefix(f.Name, "AtCom[") || strings.HasPrefix(f.Name, "UCmts[")) {
						continue
					}
					if !strings.HasPrefix(f.Name, "[") {
						return nil, sourceError(rungPath, f.Line, "unqualified rung field "+f.Name)
					}
					rung.Graphic = append(rung.Graphic, f)
				}
			}
			if slCount == 0 && len(rung.Graphic) == 0 {
				return nil, sourceError(rungPath, r.Line, "rung has no source representation")
			}
			section.Rungs = append(section.Rungs, rung)
		}
		result.Sections = append(result.Sections, section)
	}
	return result, nil
}
