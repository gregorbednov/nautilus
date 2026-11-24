package cxp

import "strings"

// PouVarList uses file codes. POUHeadDoc's COM API separately uses 3 for In Out.
const (
	fbFileExternal = 3
	fbFileInOut    = 4
)

// FBDeclarationFinding identifies a source-level requirement for the CJ1M
// CPU21 and CJ1H-H CPU65. A version requirement alone does not qualify code generation.
type FBDeclarationFinding struct {
	BlockName             string `json:"blockName,omitempty"`
	Path                  string `json:"path"`
	Line                  int    `json:"line"`
	Variable              string `json:"variable"`
	Usage                 int    `json:"usageCode"`
	UsageName             string `json:"usage"`
	TypeCode              int    `json:"typeCode"`
	TypeName              string `json:"typeName,omitempty"`
	MinimumCJ1UnitVersion string `json:"minimumCJ1UnitVersion,omitempty"`
	Issue                 string `json:"issue,omitempty"`
}

func parseFBDeclaration(row, path string, line int) (FBVariable, error) {
	columns := strings.SplitN(row, ",", 8)
	if len(columns) != 8 || !strings.HasSuffix(columns[7], ";") {
		return FBVariable{}, fbError(path, line, "unsupported declaration row")
	}
	for i := 0; i < 7; i++ {
		columns[i] = strings.TrimSpace(columns[i])
		if strings.ContainsAny(columns[i], "\"\x11") {
			return FBVariable{}, fbError(path, line, "unsupported quoted declaration column")
		}
	}
	usage, errUsage := fbNumber(columns[0], fbFileInOut)
	typeCode, errType := fbNumber(columns[2], 65535)
	name := columns[1]
	if errUsage != nil || errType != nil || name == "" || strings.EqualFold(name, "EN") || strings.EqualFold(name, "ENO") {
		return FBVariable{}, fbError(path, line, "invalid or ambiguous declaration")
	}
	return FBVariable{Name: name, Usage: usage, TypeCode: typeCode, TypeName: FBTypeName(typeCode),
		RemainingColumns: [4]string{columns[3], columns[4], columns[5], columns[6]},
		Comment:          strings.TrimSuffix(columns[7], ";"), Line: line}, nil
}

func (v FBVariable) cj1DeclarationConstraint(language string) (version, issue string) {
	if v.Usage == fbFileExternal {
		// External symbol resolution has no measured declaration constraint here.
		return "", ""
	}
	if v.TypeCode == 15 {
		if v.Usage == 1 || v.Usage == 2 {
			return "", "STRING requires an Internal or In Out variable in a Structured Text function block"
		}
		if language == "FBLadder" {
			return "", "STRING requires a Structured Text function block"
		}
		return "4.0", ""
	}
	if v.Usage == fbFileInOut {
		return "4.0", ""
	}
	return "", ""
}

func inspectFBDeclarations(path string, wrapper Field) []FBDeclarationFinding {
	language, _ := wrapper.Get("FBType")
	if language.Text() != "FBLadder" && language.Text() != "FBST" {
		return nil
	}
	block, _ := wrapper.Get("FunctionBlock")
	name, _ := block.Get("Name")
	header, _ := block.Get("POUHeader")
	list, _ := header.Get("PouVarList")
	if list.Kind != "list" || len(list.Raw) > MaxSymbolBytes {
		return nil
	}
	declPath := path + ".FunctionBlock.POUHeader.PouVarList"
	var findings []FBDeclarationFinding
	for i, row := range strings.Split(list.Raw, "\n") {
		v, err := parseFBDeclaration(strings.TrimSpace(row), declPath, list.Line+2+i)
		if err != nil {
			continue
		}
		version, issue := v.cj1DeclarationConstraint(language.Text())
		if version != "" || issue != "" {
			usages := [...]string{"Internal", "Input", "Output", "External", "In Out"}
			findings = append(findings, FBDeclarationFinding{BlockName: name.Text(), Path: declPath, Line: v.Line, Variable: v.Name,
				Usage: v.Usage, UsageName: usages[v.Usage], TypeCode: v.TypeCode, TypeName: v.TypeName,
				MinimumCJ1UnitVersion: version, Issue: issue})
		}
	}
	return findings
}
