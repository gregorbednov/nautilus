package cxp

import (
	"fmt"
	"regexp"
	"strings"
)

// STFloatSource holds a CPU21 ST definition reconstructed from its saved
// declaration rows and original STBodyText. TaskID is the saved body number.
type STFloatSource struct {
	Text   string
	TaskID int
}

var stIdentifier = regexp.MustCompile("^[A-Za-z][A-Za-z0-9_]*$")

// STFloatSourceForDefinition returns nil for ladder wrappers, which have a
// separate compiler. Only the observed FBST 1.8 file layout is supported.
func (doc *Document) STFloatSourceForDefinition(path string) (*STFloatSource, error) {
	wrapper, err := doc.definitionWrapper(path)
	if err != nil {
		return nil, err
	}
	lang, err := sourceField(wrapper.Fields, "FBType", "scalar", path)
	if err != nil {
		return nil, err
	}
	if lang.Text() == "FBLadder" {
		return nil, nil
	}
	if lang.Text() != "FBST" || len(wrapper.Fields) != 2 {
		return nil, definitionError(path, lang.Line, "unsupported wrapper")
	}
	b, err := sourceField(wrapper.Fields, "FunctionBlock", "object", path)
	if err != nil {
		return nil, err
	}
	for _, f := range b.Fields {
		switch f.Name {
		case "IEC1131Type", "CompileInline", "Name", "FileType", "Created", "Modified", "Protection", "FBDefVer", "OnlineReserveMemory", "POUHeader", "STBodyText", "TaskID":
		default:
			return nil, definitionError(path, f.Line, "unsupported ST field "+f.Name)
		}
	}
	for _, p := range [][2]string{{"IEC1131Type", "4"}, {"FileType", "4"}, {"CompileInline", "0"}, {"FBDefVer", `"1.8"`}} {
		if err := sourceValue(b.Fields, p[0], p[1], path); err != nil {
			return nil, err
		}
	}
	if err := checkFBReserve(b, path); err != nil {
		return nil, err
	}
	protection, err := sourceField(b.Fields, "Protection", "object", path)
	if err != nil {
		return nil, err
	}
	if err := sourceValue(protection.Fields, "Enabled", "0", path); err != nil {
		return nil, err
	}
	name, err := sourceField(b.Fields, "Name", "scalar", path)
	if err != nil {
		return nil, err
	}
	if !stIdentifier.MatchString(name.Text()) {
		return nil, definitionError(path, name.Line, "invalid ST block name")
	}
	task, err := sourceField(b.Fields, "TaskID", "scalar", path)
	if err != nil {
		return nil, err
	}
	id, err := fbNumber(task.Value, 65535)
	if err != nil {
		return nil, definitionError(path, task.Line, "invalid ST TaskID")
	}
	header, err := sourceField(b.Fields, "POUHeader", "object", path)
	if err != nil {
		return nil, err
	}
	vars, _, err := fbVariables(header, path+".POUHeader", "FBST", false)
	if err != nil {
		return nil, err
	}
	body, err := sourceField(b.Fields, "STBodyText", "raw", path)
	if err != nil {
		return nil, err
	}
	var src strings.Builder
	fmt.Fprintf(&src, "FUNCTION_BLOCK %s\n", name.Text())
	for _, v := range vars {
		group, typ, initial := "VAR", "REAL", "0.0"
		switch v.Usage {
		case 0, 2:
			if v.TypeCode != 10 {
				return nil, definitionError(path, v.Line, "expected REAL")
			}
			if v.Usage == 2 {
				group = "VAR_OUTPUT"
			}
		case 1:
			if v.TypeCode != 1 {
				return nil, definitionError(path, v.Line, "expected INT")
			}
			group, typ, initial = "VAR_INPUT", "INT", "0"
		default:
			return nil, definitionError(path, v.Line, "unsupported ST declaration")
		}
		if !stIdentifier.MatchString(v.Name) || v.RemainingColumns != [4]string{"", "0", initial, "0"} {
			return nil, definitionError(path, v.Line, "unsupported ST variable metadata")
		}
		fmt.Fprintf(&src, "%s\n%s : %s;\nEND_VAR\n", group, v.Name, typ)
	}
	src.WriteString(body.Raw)
	src.WriteString("\nEND_FUNCTION_BLOCK\n")
	return &STFloatSource{Text: src.String(), TaskID: id}, nil
}
