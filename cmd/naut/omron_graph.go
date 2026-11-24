package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/joyautomation/nautilus/lang/ld"
	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/cxp"
)

func omronGraph(args []string) error {
	flags := flag.NewFlagSet("omron graph", flag.ContinueOnError)
	resource := flags.String("resource", "Resource[0]", "CXP/CXT resource path")
	encoding := flags.String("encoding", "auto", "CXT text encoding")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("expected one CXP/CXT filename or -")
	}
	var data []byte
	var err error
	if flags.Arg(0) == "-" {
		data, err = io.ReadAll(io.LimitReader(os.Stdin, cxp.MaxSize+1))
	} else {
		data, err = readOmronFile(flags.Arg(0))
	}
	if err != nil {
		return err
	}
	doc, err := cxp.ReadEncoding(data, *encoding)
	if err != nil {
		return err
	}
	model, err := omronLadderModel(doc, *resource)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(model)
}

type omronLadderGraph struct {
	*ld.Model
	Omron omronWatchModel `json:"omron"`
}

func omronLadderModel(doc *cxp.Document, resource string) (*omronLadderGraph, error) {
	tasks, err := doc.SourceForTasks(resource)
	if err != nil {
		return nil, err
	}
	model := &omronLadderGraph{Model: &ld.Model{Rungs: []ld.Rung{}}}
	watch := newOmronWatchBindings(&model.Omron)
	for _, task := range tasks {
		program, err := doc.SourceForProgram(task.Path)
		if err != nil {
			return nil, err
		}
		scope := fmt.Sprintf("TK%d", task.Number-256)
		watch.readSymbols(doc, task.Path, scope)
		for _, section := range program.Sections {
			heading := fmt.Sprintf("TK%d · %s · %s", task.Number-256, task.Name, section.Name)
			model.Comments = append(model.Comments, ld.Comment{Line: section.Line, EndLine: section.Line, Text: heading})
			for i, source := range section.Rungs {
				name := fmt.Sprintf("TK%d · %s · R%d", task.Number-256, section.Name, i)
				rows, err := omronLadderRung(doc, program.Path, source)
				if err != nil {
					lines := []string{name + " — diagram unavailable: " + err.Error()}
					if source.Comment != "" {
						lines = append(lines, source.Comment)
					}
					for _, instruction := range source.Instructions {
						lines = append(lines, instruction.Text)
					}
					model.Comments = append(model.Comments, ld.Comment{Line: source.Line, EndLine: source.Line, Text: strings.Join(lines, "\n")})
					continue
				}
				for i := range rows {
					rows[i].Name = name
					rows[i].Scope = scope
					if len(rows) > 1 {
						rows[i].Name += fmt.Sprintf(".%d", i+1)
					}
					watch.readOperands(scope, rows[i].Elements)
					watch.readOperands(scope, rows[i].Coils)
				}
				model.Rungs = append(model.Rungs, rows...)
			}
		}
	}
	return model, nil
}

func omronLadderRung(doc *cxp.Document, program string, source cxp.Rung) ([]ld.Rung, error) {
	rung := ld.Rung{Line: source.Line, EndLine: source.Line, Comment: source.Comment}
	if len(source.Graphic) > 0 {
		if len(source.Instructions) > 0 {
			return nil, errors.New("mixed graphic and mnemonic source")
		}
		call, err := doc.PlanFBCall(program, source.Path)
		if err != nil {
			return nil, err
		}
		var args []string
		for _, pin := range call.Inputs {
			args = append(args, pin.Declaration.Name+" := "+pin.Value.Source)
		}
		for _, pin := range call.Outputs {
			args = append(args, pin.Declaration.Name+" => "+pin.Value.Source)
		}
		rung.Elements = []ld.Element{
			{Kind: "contact", Ref: call.Enable.Source},
			{Kind: "fb", Inst: call.Instance, Type: call.Definition, Args: strings.Join(args, ", "), PowerIn: "EN"},
		}
		rung.EndLine = call.CallLine
		return []ld.Rung{rung}, nil
	}
	if len(source.Instructions) == 0 {
		return []ld.Rung{rung}, nil
	}
	var rows []ld.Rung
	var stack [][]ld.Element
	var relaySaves uint16
	afterOutput := false
	endLine := source.Line

	// A new LD after an output begins a new display row. OUT TR and LD TR
	// stay explicit so a restore never re-evaluates the contacts it saved.
	flush := func() {
		series := stack[0]
		last := len(series)
		for last > 0 && series[last-1].Kind == "coil" && series[last-1].Mode != "TR" {
			last--
		}
		rung.Elements, rung.Coils = series[:last], series[last:]
		rung.EndLine = endLine
		rows = append(rows, rung)
	}
	top := func() *[]ld.Element { return &stack[len(stack)-1] }
	for _, sourceInstruction := range source.Instructions {
		words := strings.Fields(sourceInstruction.Text)
		mnemonic := strings.ToUpper(words[0])
		fail := func() ([]ld.Rung, error) {
			return nil, fmt.Errorf("line %d: %s", sourceInstruction.Line, sourceInstruction.Text)
		}
		switch mnemonic {
		case "LD", "LD(000)", "LDNOT":
			if len(words) != 2 {
				return fail()
			}
			relay, relayErr := codec.ParseTemporaryRelay(words[1])
			if afterOutput {
				flush()
				rung = ld.Rung{Line: sourceInstruction.Line}
				stack = nil
				if relayErr != nil {
					relaySaves = 0
				}
			}
			if len(stack) == compiler.MaxLogicDepth || (relayErr == nil && (mnemonic == "LDNOT" || relaySaves&(1<<relay) == 0)) {
				return fail()
			}
			stack = append(stack, []ld.Element{{Kind: "contact", Ref: words[1], Neg: mnemonic == "LDNOT"}})
			afterOutput = false
		case "AND", "ANDNOT", "OR", "ORNOT":
			if len(stack) == 0 || len(words) != 2 {
				return fail()
			}
			contact := ld.Element{Kind: "contact", Ref: words[1], Neg: strings.HasSuffix(mnemonic, "NOT")}
			if strings.HasPrefix(mnemonic, "OR") {
				*top() = omronParallel(*top(), []ld.Element{contact})
			} else {
				*top() = append(*top(), contact)
			}
			afterOutput = false
		case "ANDLD", "ORLD":
			if len(stack) < 2 || len(words) != 1 {
				return fail()
			}
			right := *top()
			stack = stack[:len(stack)-1]
			if mnemonic == "ORLD" {
				*top() = omronParallel(*top(), right)
			} else {
				*top() = append(*top(), right...)
			}
			afterOutput = false
		case "OUT", "SET", "RSET":
			if len(stack) == 0 || len(words) != 2 {
				return fail()
			}
			mode := map[string]string{"SET": "S", "RSET": "R"}[mnemonic]
			if relay, err := codec.ParseTemporaryRelay(words[1]); mnemonic == "OUT" && err == nil {
				if relaySaves&(1<<relay) != 0 {
					return fail()
				}
				relaySaves |= 1 << relay
				mode = "TR"
			} else {
				if len(stack) != 1 {
					return fail()
				}
				afterOutput = true
			}
			*top() = append(*top(), ld.Element{Kind: "coil", Ref: words[1], Mode: mode})
		case "UP", "UP(521)", "DOWN", "DOWN(522)":
			if len(stack) == 0 || len(words) != 1 {
				return fail()
			}
			*top() = append(*top(), ld.Element{Kind: "fn", Fn: mnemonic, Text: sourceInstruction.Text})
			afterOutput = false
		case "TIM", "TIMH", "TIMH(015)", "MOV", "MOV(021)":
			if len(stack) != 1 || len(words) != 3 {
				return fail()
			}
			element := ld.Element{Kind: "fn", Fn: mnemonic, Text: sourceInstruction.Text}
			if strings.HasPrefix(mnemonic, "TIM") {
				element.Ref = "T" + words[1]
			}
			*top() = append(*top(), element)
			afterOutput = true
		case "END", "END(001)":
			if len(words) != 1 || len(source.Instructions) != 1 {
				return fail()
			}
			stack = [][]ld.Element{{{Kind: "fn", Fn: mnemonic, Text: sourceInstruction.Text}}}
			afterOutput = true
		default:
			return fail()
		}
		endLine = sourceInstruction.Line
	}
	if len(stack) != 1 || !afterOutput {
		return nil, errors.New("incomplete or unjoined execution condition")
	}
	flush()
	return rows, nil
}

func omronParallel(left, right []ld.Element) []ld.Element {
	if len(left) == 1 && left[0].Kind == "branch" {
		left[0].Legs = append(left[0].Legs, right)
		return left
	}
	return []ld.Element{{Kind: "branch", Legs: [][]ld.Element{left, right}}}
}
