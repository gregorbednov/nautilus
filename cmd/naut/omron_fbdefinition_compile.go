package main

import (
	"encoding/json"
	"errors"
	"flag"
	"os"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/cxp"
)

func omronCompileFBDefinitionBody(args []string) error {
	return omronCompileFBDefinition(args, false)
}

func omronCompileFBDefinitionRecord(args []string) error {
	return omronCompileFBDefinition(args, true)
}

func omronCompileFBDefinition(args []string, record bool) error {
	command := "compile-fb-definition-body"
	if record {
		command = "compile-fb-definition"
	}
	flags := flag.NewFlagSet("omron "+command, flag.ContinueOnError)
	number := -1
	if record {
		flags.IntVar(&number, "number", -1, "explicit definition number (0..1023)")
	}
	model := flags.String("model", "", "explicit qualified CPU model")
	definition := flags.String("definition", "", "fully qualified FB definition path")
	timerMode := flags.String("timer-mode", "", "explicit timer mode (bcd)")
	encoding := flags.String("encoding", "auto", "CXT text encoding")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 || *model == "" || *definition == "" || *timerMode != "bcd" || (record && (number < 0 || number > 1023)) {
		requirements := command + " requires -model MODEL -definition PATH -timer-mode bcd"
		if record {
			requirements += " -number 0..1023"
		}
		return errors.New(requirements + " FILE")
	}
	data, err := readOmronFile(flags.Arg(0))
	if err != nil {
		return err
	}
	doc, err := cxp.ReadEncoding(data, *encoding)
	if err != nil {
		return err
	}
	profile := codec.Profile{Model: *model, TimerCounterMode: codec.TimerModeBCD}
	var result any
	format := "cj1-fb-definition-body-containers"
	if record {
		result, err = compiler.CompileCXPFBDefinition(profile, doc, *definition, number)
		format = "cj1-fb-definition-record-containers"
	} else {
		result, err = compiler.CompileCXPFBDefinitionBody(profile, doc, *definition)
	}
	if err != nil {
		return err
	}
	report := struct {
		SchemaVersion             int    `json:"schemaVersion"`
		Format                    string `json:"format"`
		SourceSHA256              string `json:"sourceSHA256"`
		Model                     string `json:"model"`
		TimerCounterMode          string `json:"timerCounterMode"`
		RelativeFrameAllocated    bool   `json:"relativeFrameAllocated"`
		DefinitionRecordGenerated bool   `json:"definitionRecordGenerated,omitempty"`
		InstancesPlaced           bool   `json:"instancesPlaced"`
		Linked                    bool   `json:"linked"`
		HardwareQualified         bool   `json:"hardwareQualified"`
		Result                    any    `json:"result"`
	}{SchemaVersion: 1, Format: format, SourceSHA256: doc.SHA256, Model: *model, TimerCounterMode: *timerMode, RelativeFrameAllocated: true, DefinitionRecordGenerated: record, Result: result}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}
