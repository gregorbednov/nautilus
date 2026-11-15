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
	flags := flag.NewFlagSet("omron compile-fb-definition-body", flag.ContinueOnError)
	model := flags.String("model", "", "explicit qualified CPU model")
	definition := flags.String("definition", "", "fully qualified FB definition path")
	timerMode := flags.String("timer-mode", "", "explicit timer mode (bcd)")
	encoding := flags.String("encoding", "auto", "CXT text encoding")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 || *model == "" || *definition == "" || *timerMode != "bcd" {
		return errors.New("compile-fb-definition-body requires -model MODEL -definition PATH -timer-mode bcd FILE")
	}
	data, err := readOmronFile(flags.Arg(0))
	if err != nil {
		return err
	}
	doc, err := cxp.ReadEncoding(data, *encoding)
	if err != nil {
		return err
	}
	result, err := compiler.CompileCXPFBDefinitionBody(codec.Profile{Model: *model, TimerCounterMode: codec.TimerModeBCD}, doc, *definition)
	if err != nil {
		return err
	}
	report := struct {
		SchemaVersion          int                        `json:"schemaVersion"`
		Format                 string                     `json:"format"`
		SourceSHA256           string                     `json:"sourceSHA256"`
		Model                  string                     `json:"model"`
		TimerCounterMode       string                     `json:"timerCounterMode"`
		RelativeFrameAllocated bool                       `json:"relativeFrameAllocated"`
		InstancesPlaced        bool                       `json:"instancesPlaced"`
		Linked                 bool                       `json:"linked"`
		HardwareQualified      bool                       `json:"hardwareQualified"`
		Result                 *compiler.FBDefinitionBody `json:"result"`
	}{SchemaVersion: 1, Format: "cj1-fb-definition-body-containers", SourceSHA256: doc.SHA256, Model: *model, TimerCounterMode: *timerMode, RelativeFrameAllocated: true, Result: result}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}
