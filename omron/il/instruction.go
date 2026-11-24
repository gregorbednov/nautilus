// Package il models Omron mnemonic source independently of CPU encoding.
// Parsing is lexical only: it does not establish instruction availability,
// operand types, function-number correctness, or executable semantics.
package il

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Instruction retains source spellings. Function is the optional decimal
// function label (including leading zeroes), never an executable opcode.
// Modifier retains the leading @ or % spelling without assigning semantics.
// Operands remain opaque until a CPU-qualified instruction schema is available.
type Instruction struct {
	Source   string   `json:"source"`
	Mnemonic string   `json:"mnemonic"`
	Modifier string   `json:"modifier,omitempty"`
	Function string   `json:"function,omitempty"`
	Operands []string `json:"operands"`
}

// SyntaxError reports a one-based UTF-8 byte column within the source line.
type SyntaxError struct {
	Column  int    `json:"column"`
	Message string `json:"message"`
}

func (e *SyntaxError) Error() string { return fmt.Sprintf("column %d: %s", e.Column, e.Message) }

// Parse parses one source instruction, retaining quoted operands and symbol
// expressions verbatim. It deliberately has no mnemonic allowlist: recognising
// a spelling must not be confused with qualifying it for a particular CJ1 CPU.
func Parse(source string) (Instruction, error) {
	result := Instruction{Source: source, Operands: []string{}}
	fail := func(i int, message string) (Instruction, error) {
		return result, &SyntaxError{Column: i + 1, Message: message}
	}
	if !utf8.ValidString(source) {
		return fail(0, "invalid UTF-8")
	}
	for i, r := range source {
		if unicode.IsControl(r) && r != '\t' {
			return fail(i, "control character in instruction")
		}
	}
	start := 0
	for start < len(source) && (source[start] == ' ' || source[start] == '\t') {
		start++
	}
	end := start
	for end < len(source) && source[end] != ' ' && source[end] != '\t' {
		end++
	}
	if start == end {
		return fail(start, "missing mnemonic")
	}
	token := source[start:end]
	offset := start
	if token[0] == '@' || token[0] == '%' {
		result.Modifier = token[:1]
		token = token[1:]
		offset++
	}
	if i := strings.IndexByte(token, '('); i >= 0 {
		if !strings.HasSuffix(token, ")") {
			return fail(offset+i, "unterminated function label")
		}
		result.Function = token[i+1 : len(token)-1]
		if result.Function == "" {
			return fail(offset+i+1, "empty function label")
		}
		for j, c := range result.Function {
			if c < '0' || c > '9' {
				return fail(offset+i+1+j, "function label must be decimal digits")
			}
		}
		token = token[:i]
	}
	if token == "" {
		return fail(offset, "missing mnemonic")
	}
	for j, c := range token {
		if c > 127 || !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || strings.ContainsRune("_+-*/<>=!.$", c)) {
			return fail(offset+j, "invalid mnemonic character")
		}
	}
	result.Mnemonic = token
	for i := end; i < len(source); {
		for i < len(source) && (source[i] == ' ' || source[i] == '\t') {
			i++
		}
		if i == len(source) {
			break
		}
		begin := i
		quoted := false
		quoteStart := i
		for i < len(source) {
			if source[i] == '"' {
				if quoted && i+1 < len(source) && source[i+1] == '"' {
					i += 2
					continue
				}
				quoted = !quoted
				if quoted {
					quoteStart = i
				}
			} else if !quoted && (source[i] == ' ' || source[i] == '\t') {
				break
			}
			i++
		}
		if quoted {
			return fail(quoteStart, "unterminated quoted operand")
		}
		result.Operands = append(result.Operands, source[begin:i])
	}
	return result, nil
}
