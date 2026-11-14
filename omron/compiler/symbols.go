package compiler

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/il"
)

var ErrUnknownSymbol = errors.New("symbol not defined in selected scope")
var ErrSymbolType = errors.New("symbol type is not qualified for this operand")
var ErrUnqualifiedSymbol = errors.New("symbol binding is not qualified")

// BindingUse audits an exact operand substitution. Declaration and use locations
// remain distinct; local overrides cannot silently pick a global address.
type BindingUse struct {
	Rung            string `json:"rung"`
	Line            int    `json:"line"`
	Operand         int    `json:"operand"`
	Name            string `json:"name"`
	Address         string `json:"address"`
	DataType        string `json:"dataType"`
	Scope           string `json:"scope"`
	DeclarationLine int    `json:"declarationLine"`
}

var nativeOperandPattern = regexp.MustCompile(`(?i)^(?:[0-9]+(?:\.[0-9]*)?|(?:TR|W|H|A|D|T|C)[0-9]+(?:\.[0-9]*)?|[#&*@].*)$`)

type symbolResolver struct{ globals, locals map[string]cxp.Symbol }

func newSymbolResolver(tables *cxp.SymbolTables) (*symbolResolver, error) {
	resolver := &symbolResolver{map[string]cxp.Symbol{}, map[string]cxp.Symbol{}}
	folded := map[string]string{}
	for _, scope := range []struct {
		symbols []cxp.Symbol
		target  map[string]cxp.Symbol
	}{{tables.Globals, resolver.globals}, {tables.Locals, resolver.locals}} {
		for _, symbol := range scope.symbols {
			if symbol.Name == "" {
				continue
			}
			if nativeOperandPattern.MatchString(symbol.Name) {
				return nil, fmt.Errorf("%w: symbol %q collides with native operand syntax", ErrUnqualifiedSymbol, symbol.Name)
			}
			key := strings.ToUpper(symbol.Name)
			if previous, exists := folded[key]; exists && previous != symbol.Name {
				return nil, fmt.Errorf("%w: case-variant names %q and %q are not qualified", ErrUnqualifiedSymbol, previous, symbol.Name)
			}
			folded[key] = symbol.Name
			if _, exists := scope.target[symbol.Name]; exists {
				return nil, fmt.Errorf("%w: duplicate symbol %q", ErrUnqualifiedSymbol, symbol.Name)
			}
			scope.target[symbol.Name] = symbol
		}
	}
	return resolver, nil
}

func operandKind(name string, index int) string {
	switch name {
	case "LD", "LDNOT", "AND", "ANDNOT", "OR", "ORNOT", "OUT", "OUTNOT", "SET", "RSET":
		return "bit"
	case "MOV":
		return "word"
	case "TIM", "TIMH", "TIMX", "TIMHX", "CNT", "CNTX":
		if index == 1 {
			return "word"
		}
		return "number"
	}
	return ""
}

func (resolver *symbolResolver) resolve(instruction il.Instruction, where Diagnostic) (il.Instruction, []BindingUse, error) {
	resolved := instruction
	resolved.Operands = append([]string{}, instruction.Operands...)
	var uses []BindingUse
	for index, operand := range instruction.Operands {
		kind := operandKind(instruction.Mnemonic, index)
		if kind == "" || nativeOperandPattern.MatchString(operand) {
			continue
		}
		name := operand
		if len(name) >= 2 && name[0] == '"' && name[len(name)-1] == '"' {
			name = strings.ReplaceAll(name[1:len(name)-1], `""`, `"`)
		}
		symbol, found := resolver.locals[name]
		if !found {
			symbol, found = resolver.globals[name]
		}
		if !found {
			return il.Instruction{}, nil, fmt.Errorf("%w: %q", ErrUnknownSymbol, name)
		}
		address, err := boundOperand(symbol, kind)
		if err != nil {
			return il.Instruction{}, nil, err
		}
		resolved.Operands[index] = address
		uses = append(uses, BindingUse{where.Rung, where.Line, index, name, address, symbol.DataType, symbol.Scope, symbol.Line})
	}
	return resolved, uses, nil
}

func boundOperand(symbol cxp.Symbol, kind string) (string, error) {
	if symbol.Flags != "" || symbol.AuxType != "" {
		return "", fmt.Errorf("%w: %s has flags or auxiliary type metadata", ErrUnqualifiedSymbol, symbol.Name)
	}
	address := strings.TrimSpace(symbol.Address)
	switch kind {
	case "bit":
		if strings.ToUpper(symbol.DataType) != "BOOL" {
			return "", fmt.Errorf("%w: %s requires BOOL", ErrSymbolType, symbol.Name)
		}
		parsed, err := codec.ParseBitAddress(address)
		if err != nil {
			return "", fmt.Errorf("%w: %s has no qualified direct bit address", ErrUnqualifiedSymbol, symbol.Name)
		}
		return parsed.String(), nil
	case "word":
		switch strings.ToUpper(symbol.DataType) {
		case "CHANNEL", "WORD", "INT", "UINT", "UINT_BCD":
		default:
			return "", fmt.Errorf("%w: %s requires a qualified 16-bit word type", ErrSymbolType, symbol.Name)
		}
		parsed, err := codec.ParseWordAddress(address)
		if err != nil {
			return "", fmt.Errorf("%w: %s has no qualified direct word address", ErrUnqualifiedSymbol, symbol.Name)
		}
		return parsed.String(), nil
	default:
		return "", fmt.Errorf("%w: %s cannot supply a timer/counter slot", ErrSymbolType, symbol.Name)
	}
}
