// Package compiler validates selected native mnemonic rungs and emits their
// instruction containers. It does not allocate symbols/FBs or assemble tasks,
// complete project images or PLC transfer payloads.
package compiler

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/il"
)

const (
	MaxInstructions = 65536
	MaxSourceBytes  = 8 << 20
	MaxLogicDepth   = 8
)

var ErrStructure = errors.New("invalid mnemonic program structure")
var ErrSourceLimit = errors.New("mnemonic source limit exceeded")
var ErrUnsupportedRung = errors.New("unqualified graphical rung")

type Line struct {
	Number int
	Text   string
}

// Rung is an explicit network boundary; conditions and TR definitions cannot
// cross it. Path and line numbers are retained in diagnostics and word spans.
type Rung struct {
	Path  string
	Lines []Line
}

type Diagnostic struct {
	Rung    string `json:"rung"`
	Line    int    `json:"line"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Error struct {
	Diagnostic
	Cause error
}

func (err *Error) Error() string {
	return fmt.Sprintf("%s line %d: %s: %s", err.Rung, err.Line, err.Code, err.Message)
}
func (err *Error) Unwrap() error { return err.Cause }

type Span struct {
	Rung        string `json:"rung"`
	Line        int    `json:"line"`
	OffsetWords int    `json:"offsetWords"`
	LengthWords int    `json:"lengthWords"`
}

type Body struct {
	Words         []uint32     `json:"words"`
	Spans         []Span       `json:"spans"`
	Warnings      []Diagnostic `json:"warnings"`
	MaxLogicDepth int          `json:"maxLogicDepth"`
	HasEnd        bool         `json:"hasEnd"`
	Bindings      []BindingUse `json:"bindings,omitempty"`
}

// Bytes returns big-endian containers, matching observed N1 code serialization.
// This is a bare instruction body, not an N1 image or wire payload.
func (body *Body) Bytes() []byte {
	data := make([]byte, 4*len(body.Words))
	for i, word := range body.Words {
		binary.BigEndian.PutUint32(data[i*4:], word)
	}
	return data
}

func failure(where Diagnostic, code, message string, cause error) error {
	where.Code, where.Message = code, message
	return &Error{where, cause}
}

// CompileRungs returns no body on any error. It never inserts END, drops an
// instruction or reorders a rung. END is optional for a selected body fragment;
// if present it must be terminal. Interlocks must close within this selection.
func CompileRungs(profile codec.Profile, rungs []Rung) (*Body, error) {
	return compileRungs(profile, rungs, nil)
}

type resolveInstruction func(il.Instruction, Diagnostic) (il.Instruction, []BindingUse, error)

func compileRungs(profile codec.Profile, rungs []Rung, resolve resolveInstruction) (*Body, error) {
	if _, err := codec.EncodeInstruction(profile, il.Instruction{Mnemonic: "NOP"}); err != nil {
		return nil, failure(Diagnostic{}, "profile", err.Error(), err)
	}
	if len(rungs) > MaxInstructions {
		return nil, failure(Diagnostic{}, "source-limit", "too many rungs", ErrSourceLimit)
	}
	count, total := 0, 0
	for _, rung := range rungs {
		if len(rung.Path) > MaxSourceBytes-total {
			return nil, failure(Diagnostic{}, "source-limit", "source metadata too large", ErrSourceLimit)
		}
		total += len(rung.Path)
		if len(rung.Lines) > MaxInstructions-count {
			return nil, failure(Diagnostic{Rung: rung.Path}, "source-limit", "too many instruction lines", ErrSourceLimit)
		}
		count += len(rung.Lines)
		for _, line := range rung.Lines {
			if len(line.Text) > MaxSourceBytes-total {
				return nil, failure(Diagnostic{Rung: rung.Path, Line: line.Number}, "source-limit", "source text too large", ErrSourceLimit)
			}
			total += len(line.Text)
		}
	}
	body := &Body{Words: []uint32{}, Spans: []Span{}, Warnings: []Diagnostic{}}
	interlocks := 0
	var firstIL, last Diagnostic
	for rungIndex, rung := range rungs {
		path := rung.Path
		if path == "" {
			path = fmt.Sprintf("rung[%d]", rungIndex)
		}
		flow := rungFlow{}
		for lineIndex, line := range rung.Lines {
			number := line.Number
			if number == 0 {
				number = lineIndex + 1
			}
			where := Diagnostic{Rung: path, Line: number}
			if number < 0 {
				return nil, failure(where, "source-location", "negative line number", ErrStructure)
			}
			if strings.TrimSpace(line.Text) == "" {
				continue
			}
			last = where
			if body.HasEnd {
				return nil, failure(where, "after-end", "instruction follows END", ErrStructure)
			}
			instruction, err := il.Parse(line.Text)
			if err != nil {
				return nil, failure(where, "syntax", err.Error(), err)
			}
			// The lexical model preserves spelling; flow rules, like the codec,
			// match mnemonics case-insensitively. Operand/symbol case is retained.
			instruction.Mnemonic = strings.ToUpper(instruction.Mnemonic)
			if resolve != nil {
				var uses []BindingUse
				instruction, uses, err = resolve(instruction, where)
				if err != nil {
					return nil, failure(where, "symbol", err.Error(), err)
				}
				body.Bindings = append(body.Bindings, uses...)
			}
			words, err := codec.EncodeInstruction(profile, instruction)
			if err != nil {
				return nil, failure(where, "instruction", fmt.Sprintf("%s: %v", line.Text, err), err)
			}
			switch instruction.Mnemonic {
			case "IL":
				if err := flow.output(where, 1); err != nil {
					return nil, err
				}
				if interlocks == 0 {
					firstIL = where
				}
				interlocks++
			case "ILC":
				if err := flow.finish(where); err != nil {
					return nil, err
				}
				if interlocks == 0 {
					return nil, failure(where, "il-unmatched", "ILC has no preceding IL", ErrStructure)
				}
				if interlocks > 1 {
					body.Warnings = append(body.Warnings, Diagnostic{Rung: path, Line: number, Code: "shared-ilc", Message: "multiple IL instructions share one ILC; CX program checking may report unmatched pairs"})
				}
				interlocks = 0
				flow = rungFlow{}
			case "END":
				if err := flow.finish(where); err != nil {
					return nil, err
				}
				if interlocks != 0 {
					return nil, failure(firstIL, "il-unclosed", "IL region has no ILC before END", ErrStructure)
				}
				body.HasEnd = true
				flow = rungFlow{}
			default:
				if err := flow.apply(where, instruction); err != nil {
					return nil, err
				}
			}
			if flow.depth > body.MaxLogicDepth {
				body.MaxLogicDepth = flow.depth
			}
			body.Spans = append(body.Spans, Span{path, number, len(body.Words), len(words)})
			body.Words = append(body.Words, words...)
		}
		if err := flow.finish(last); err != nil {
			return nil, err
		}
	}
	if interlocks != 0 {
		return nil, failure(firstIL, "il-unclosed", "IL region has no ILC in selected body", ErrStructure)
	}
	if len(body.Words) == 0 {
		return nil, failure(last, "empty-body", "no instructions to compile", ErrStructure)
	}
	return body, nil
}

type rungFlow struct {
	depth       int
	afterOutput bool
	relays      uint16
}

func (flow *rungFlow) output(where Diagnostic, inputs int) error {
	if flow.depth != inputs {
		return failure(where, "output-condition", fmt.Sprintf("output requires %d complete execution condition(s), have %d", inputs, flow.depth), ErrStructure)
	}
	flow.afterOutput = true
	return nil
}

func (flow *rungFlow) finish(where Diagnostic) error {
	if flow.depth > 1 {
		return failure(where, "logic-unjoined", "uncombined LD blocks at network boundary", ErrStructure)
	}
	if flow.depth == 1 && !flow.afterOutput {
		return failure(where, "missing-output", "execution condition has no output", ErrStructure)
	}
	return nil
}

func (flow *rungFlow) apply(where Diagnostic, instruction il.Instruction) error {
	switch instruction.Mnemonic {
	case "NOP":
		return nil
	case "LD", "LDNOT":
		if strings.HasPrefix(strings.ToUpper(instruction.Operands[0]), "TR") {
			relay, err := codec.ParseTemporaryRelay(instruction.Operands[0])
			if err != nil {
				return failure(where, "instruction", err.Error(), err)
			}
			if flow.relays&(1<<relay) == 0 {
				return failure(where, "tr-undefined", "temporary relay has no definition in this block", ErrStructure)
			}
			if flow.afterOutput {
				flow.depth = 0
			}
			flow.depth++
			if flow.depth > MaxLogicDepth {
				return failure(where, "logic-overflow", "more than eight pending execution conditions", ErrStructure)
			}
			flow.afterOutput = false
			return nil
		}
		if flow.afterOutput {
			*flow = rungFlow{}
		}
		flow.depth++
		if flow.depth > MaxLogicDepth {
			return failure(where, "logic-overflow", "more than eight pending execution conditions", ErrStructure)
		}
		flow.afterOutput = false
	case "AND", "ANDNOT", "OR", "ORNOT", "NOT", "UP", "DOWN":
		if flow.depth == 0 {
			return failure(where, "logic-underflow", "input instruction has no LD condition", ErrStructure)
		}
		flow.afterOutput = false
	case "ANDLD", "ORLD":
		if flow.depth < 2 {
			return failure(where, "logic-underflow", "block combination requires two execution conditions", ErrStructure)
		}
		flow.depth--
		flow.afterOutput = false
	case "OUT":
		if strings.HasPrefix(strings.ToUpper(instruction.Operands[0]), "TR") {
			if flow.depth == 0 {
				return failure(where, "tr-context", "TR save requires an execution condition", ErrStructure)
			}
			relay, err := codec.ParseTemporaryRelay(instruction.Operands[0])
			if err != nil {
				return failure(where, "instruction", err.Error(), err)
			}
			mask := uint16(1) << relay
			if flow.relays&mask != 0 {
				return failure(where, "tr-redefined", "temporary relay is assigned twice in the same block", ErrStructure)
			}
			flow.relays |= mask
			return nil
		}
		return flow.output(where, 1)
	case "OUTNOT", "SET", "RSET", "MOV", "TIM", "TIMH", "TIMX", "TIMHX":
		return flow.output(where, 1)
	case "CNT", "CNTX":
		if err := flow.output(where, 2); err != nil {
			return err
		}
		// Counter continuation from either input is not qualified here. A new
		// instruction block must establish its own execution condition.
		*flow = rungFlow{}
	default:
		return failure(where, "flow-unqualified", "instruction has no qualified execution-condition rule", ErrStructure)
	}
	return nil
}
