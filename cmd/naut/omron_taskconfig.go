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

func omronCompileTaskConfiguration(args []string) error {
	flags := flag.NewFlagSet("omron compile-task-configuration", flag.ContinueOnError)
	model := flags.String("model", "", "explicit qualified CPU model")
	resource := flags.String("resource", "", "fully qualified resource scope")
	timer := flags.String("timer-mode", "", "explicit BCD timer mode")
	encoding := flags.String("encoding", "auto", "CXT text encoding")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 || *model == "" || *resource == "" || *timer != "bcd" {
		return errors.New("compile-task-configuration requires -model MODEL -resource PATH -timer-mode bcd FILE")
	}
	data, err := readOmronFile(flags.Arg(0))
	if err != nil {
		return err
	}
	doc, err := cxp.ReadEncoding(data, *encoding)
	if err != nil {
		return err
	}
	result, err := compiler.CompileCXPTaskConfiguration(codec.Profile{Model: *model, TimerCounterMode: codec.TimerModeBCD}, doc, *resource)
	if err != nil {
		return err
	}
	report := struct {
		SchemaVersion              int                         `json:"schemaVersion"`
		Format                     string                      `json:"format"`
		SourceSHA256               string                      `json:"sourceSHA256"`
		Model                      string                      `json:"model"`
		TimerCounterMode           string                      `json:"timerCounterMode"`
		TaskConfigurationGenerated bool                        `json:"taskConfigurationGenerated"`
		GlobalHeaderGenerated      bool                        `json:"globalHeaderGenerated"`
		Linked                     bool                        `json:"linked"`
		HardwareQualified          bool                        `json:"hardwareQualified"`
		Result                     *compiler.TaskConfiguration `json:"result"`
	}{SchemaVersion: 1, Format: "cj1-task-configuration", SourceSHA256: doc.SHA256, Model: *model, TimerCounterMode: *timer, TaskConfigurationGenerated: true, Result: result}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}
