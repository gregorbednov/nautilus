package cxp

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const MaxSymbolRows = 65536
const MaxSymbolBytes = 8 << 20

var ErrSymbolTable = errors.New("unsupported or malformed CXP symbol table")

type SymbolError struct {
	Scope   string
	Line    int
	Message string
}

func (err *SymbolError) Error() string {
	return fmt.Sprintf("%s line %d: %s", err.Scope, err.Line, err.Message)
}
func (err *SymbolError) Unwrap() error { return ErrSymbolTable }

// Symbol retains the five declaration columns preceding its opaque comment.
// Empty names are address annotations, not named bindings. Auto/FB/array/unknown
// declarations are inventoried; parsing them does not allocate or qualify them.
type Symbol struct {
	Name     string `json:"name"`
	Address  string `json:"address"`
	DataType string `json:"dataType"`
	AuxType  string `json:"auxType,omitempty"`
	Flags    string `json:"flags,omitempty"`
	Scope    string `json:"scope"`
	Line     int    `json:"line"`
}

type SymbolTables struct {
	Resource string   `json:"resource"`
	Program  string   `json:"program"`
	Globals  []Symbol `json:"globals"`
	Locals   []Symbol `json:"locals"`
}

var programScopePattern = regexp.MustCompile(`^(Resource\[[0-9]+\])\.Programs\.(Program\[[0-9]+\])$`)
var commentMarkerPattern = regexp.MustCompile(`^\$\?St\$Bk\?_#\[[0-9]+\]$`)

func symbolFailure(scope string, line int, message string) error {
	return &SymbolError{scope, line, message}
}

func uniqueSymbolField(fields []Field, name, scope string, required bool) (Field, error) {
	var result Field
	count := 0
	for _, field := range fields {
		if field.Name == name {
			result = field
			count++
		}
	}
	if count > 1 || (required && count == 0) {
		return Field{}, symbolFailure(scope, result.Line, "missing or duplicate "+name)
	}
	return result, nil
}

// SymbolsForProgram reads only the resource globals and the specified native
// program's locals. Other programs and FB declaration formats are never merged.
func (doc *Document) SymbolsForProgram(programPath string) (*SymbolTables, error) {
	if doc == nil {
		return nil, symbolFailure(programPath, 0, "nil source document")
	}
	match := programScopePattern.FindStringSubmatch(programPath)
	if len(match) != 3 {
		return nil, symbolFailure(programPath, 0, "unsupported program scope")
	}
	resource, err := uniqueSymbolField(doc.Fields, match[1], match[1], true)
	if err != nil {
		return nil, err
	}
	programs, err := uniqueSymbolField(resource.Fields, "Programs", match[1], true)
	if err != nil {
		return nil, err
	}
	program, err := uniqueSymbolField(programs.Fields, match[2], programPath, true)
	if err != nil {
		return nil, err
	}
	if resource.Kind != "object" || programs.Kind != "object" || program.Kind != "object" {
		return nil, symbolFailure(programPath, program.Line, "program/resource is not an object")
	}
	total := 0
	globals, err := readSymbolScope(resource.Fields, "GlobalVariables", match[1], &total)
	if err != nil {
		return nil, err
	}
	locals, err := readSymbolScope(program.Fields, "LocalVariables", programPath, &total)
	if err != nil {
		return nil, err
	}
	if len(globals) > MaxSymbolRows-len(locals) {
		return nil, symbolFailure(programPath, 0, "symbol row limit")
	}
	return &SymbolTables{match[1], programPath, globals, locals}, nil
}

func readSymbolScope(fields []Field, name, scope string, total *int) ([]Symbol, error) {
	table, err := uniqueSymbolField(fields, name, scope, false)
	if err != nil {
		return nil, err
	}
	if table.Name == "" {
		return []Symbol{}, nil
	}
	if table.Kind != "object" {
		return nil, symbolFailure(scope, table.Line, "symbol scope is not an object")
	}
	list, err := uniqueSymbolField(table.Fields, "VariableList", scope, true)
	if err != nil {
		return nil, err
	}
	count, err := uniqueSymbolField(table.Fields, "VariableCount", scope, true)
	if err != nil {
		return nil, err
	}
	if list.Kind != "list" || count.Kind != "scalar" {
		return nil, symbolFailure(scope, list.Line, "unsupported symbol list/count representation")
	}
	want, err := strconv.Atoi(strings.TrimSpace(count.Value))
	if err != nil || want < 0 || want > MaxSymbolRows {
		return nil, symbolFailure(scope, count.Line, "invalid symbol count")
	}
	if len(list.Raw) > MaxSymbolBytes-*total {
		return nil, symbolFailure(scope, list.Line, "symbol byte limit")
	}
	*total += len(list.Raw)
	rows, err := parseSymbolRows(list, scope)
	if err != nil {
		return nil, err
	}
	if len(rows) != want {
		return nil, symbolFailure(scope, count.Line, "symbol count differs from parsed rows")
	}
	return rows, nil
}

func parseSymbolRows(list Field, scope string) ([]Symbol, error) {
	rows := []Symbol{}
	seen := map[string]bool{}
	lines := strings.Split(list.Raw, "\n")
	for index := 0; index < len(lines); index++ {
		line := strings.TrimSpace(lines[index])
		if line == "" {
			continue
		}
		number := list.Line + 2 + index
		columns, tail, err := symbolColumns(line)
		if err != nil {
			return nil, symbolFailure(scope, number, err.Error())
		}
		if len(rows) >= MaxSymbolRows {
			return nil, symbolFailure(scope, number, "symbol row limit")
		}
		if columns[0] != "" && seen[columns[0]] {
			return nil, symbolFailure(scope, number, "duplicate symbol "+columns[0])
		}
		seen[columns[0]] = true
		rows = append(rows, Symbol{columns[0], columns[1], columns[2], columns[3], columns[4], scope, number})
		// The sixth field is an opaque comment, not CSV: quotes/commas inside
		// inline comments are literal, and detached raw blocks may span lines.
		if strings.HasSuffix(strings.TrimSpace(tail), ";") {
			continue
		}
		if strings.TrimSpace(tail) != "" || index+1 >= len(lines) {
			return nil, symbolFailure(scope, number, "unterminated symbol comment")
		}
		index++
		marker := strings.TrimSpace(lines[index])
		if !commentMarkerPattern.MatchString(marker) {
			return nil, symbolFailure(scope, number, "unsupported multiline symbol comment")
		}
		closed := false
		for index++; index < len(lines); index++ {
			if strings.HasSuffix(strings.TrimSpace(lines[index]), marker+";") {
				closed = true
				break
			}
		}
		if !closed {
			return nil, symbolFailure(scope, number, "unterminated raw symbol comment")
		}
	}
	return rows, nil
}

// symbolColumns parses exactly five comma-separated CXT fields. It deliberately
// leaves the remaining comment untouched; encoding/csv would misread its quotes.
func symbolColumns(line string) ([5]string, string, error) {
	var columns [5]string
	for field := 0; field < 5; field++ {
		line = strings.TrimLeft(line, " \t")
		if strings.HasPrefix(line, `"`) {
			var value strings.Builder
			closed := false
			for index := 1; index < len(line); index++ {
				if line[index] != '"' {
					value.WriteByte(line[index])
					continue
				}
				if index+1 < len(line) && line[index+1] == '"' {
					value.WriteByte('"')
					index++
					continue
				}
				columns[field] = value.String()
				line = strings.TrimLeft(line[index+1:], " \t")
				if !strings.HasPrefix(line, ",") {
					return columns, "", errors.New("quoted symbol field lacks comma")
				}
				line = line[1:]
				closed = true
				break
			}
			if !closed {
				return columns, "", errors.New("unterminated quoted symbol field")
			}
		} else {
			value, rest, ok := strings.Cut(line, ",")
			if !ok || strings.Contains(value, `"`) {
				return columns, "", errors.New("malformed symbol declaration columns")
			}
			columns[field] = strings.TrimSpace(value)
			line = rest
		}
	}
	return columns, line, nil
}
