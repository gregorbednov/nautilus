package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/cxp"
)

func omronCompileRungs(args []string) error {
	flags := flag.NewFlagSet("omron compile-rungs", flag.ContinueOnError)
	model := flags.String("model", "", "explicit codec model: CJ1M CPU21 or CJ1H-H CPU65")
	timerMode := flags.String("timer-mode", "", "explicit timer/counter mode: bcd or binary; default unknown")
	encoding := flags.String("encoding", "auto", "CXT text encoding")
	var paths []string
	flags.Func("rung", "select a rung path; repeat for a contiguous selection in one program", func(path string) error { paths = append(paths, path); return nil })
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 || *model == "" || len(paths) == 0 {
		return errors.New("compile-rungs requires -model MODEL, one or more -rung PATH flags, and one CXP/CXT filename")
	}
	wanted := map[string]bool{}
	for _, path := range paths {
		if path == "" || wanted[path] {
			return errors.New("empty or duplicate -rung selection")
		}
		wanted[path] = true
	}
	data, err := readOmronFile(flags.Arg(0))
	if err != nil {
		return err
	}
	doc, err := cxp.ReadEncoding(data, *encoding)
	if err != nil {
		return err
	}
	var selected []cxp.Rung
	previous := -1
	owner := ""
	for index, rung := range doc.Summary().Rungs {
		if !wanted[rung.Path] {
			continue
		}
		section := strings.Index(rung.Path, ".Sections.")
		if section < 0 || !strings.Contains(rung.Path[:section], ".Programs.Program[") {
			return fmt.Errorf("rung %q is not in a native program section", rung.Path)
		}
		program := rung.Path[:section]
		if owner != "" && owner != program {
			return errors.New("selected rungs must belong to one program")
		}
		if previous >= 0 && index != previous+1 {
			return errors.New("selected rungs must be contiguous; intervening rungs cannot be skipped")
		}
		owner, previous = program, index
		selected = append(selected, rung)
		delete(wanted, rung.Path)
	}
	if len(wanted) != 0 {
		for _, path := range paths {
			if wanted[path] {
				return fmt.Errorf("rung not found: %s", path)
			}
		}
	}
	profile := codec.Profile{Model: *model, TimerCounterMode: codec.TimerCounterMode(*timerMode)}
	body, err := compiler.CompileCXPProgramRungs(profile, doc, owner, selected)
	if err != nil {
		return err
	}
	// Nothing is emitted until every selected rung has been validated. This
	// report is intentionally distinct from a complete project/native image.
	report := struct {
		Schema              int            `json:"schemaVersion"`
		Format              string         `json:"format"`
		SourceSHA256        string         `json:"sourceSHA256"`
		Program             string         `json:"program"`
		Model               string         `json:"model"`
		TimerMode           string         `json:"timerCounterMode"`
		Body                *compiler.Body `json:"body"`
		CodeHex             string         `json:"codeHex"`
		FullProjectCompiled bool           `json:"fullProjectCompiled"`
		HardwareQualified   bool           `json:"hardwareQualified"`
	}{Schema: 1, Format: "cj1-selected-rung-containers", SourceSHA256: doc.SHA256, Program: owner, Model: *model, TimerMode: *timerMode, Body: body, CodeHex: hex.EncodeToString(body.Bytes())}
	output := json.NewEncoder(os.Stdout)
	output.SetIndent("", "  ")
	return output.Encode(report)
}
