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

func omronCompileTaskRecord(args []string) error {
	flags := flag.NewFlagSet("omron compile-task-record", flag.ContinueOnError)
	model := flags.String("model", "", "explicit qualified CPU model")
	program := flags.String("program", "", "fully qualified source program path")
	timer := flags.String("timer-mode", "", "explicit BCD timer mode")
	encoding := flags.String("encoding", "auto", "CXT text encoding")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 || *model == "" || *program == "" || *timer != "bcd" {
		return errors.New("compile-task-record requires -model MODEL -program PATH -timer-mode bcd FILE")
	}
	data, err := readOmronFile(flags.Arg(0))
	if err != nil {
		return err
	}
	doc, err := cxp.ReadEncoding(data, *encoding)
	if err != nil {
		return err
	}
	result, err := compiler.CompileCXPTaskRecord(codec.Profile{Model: *model, TimerCounterMode: codec.TimerModeBCD}, doc, *program)
	if err != nil {
		return err
	}
	report := struct {
		SchemaVersion              int                  `json:"schemaVersion"`
		Format                     string               `json:"format"`
		SourceSHA256               string               `json:"sourceSHA256"`
		Model                      string               `json:"model"`
		TimerCounterMode           string               `json:"timerCounterMode"`
		TaskRecordGenerated        bool                 `json:"taskRecordGenerated"`
		TaskConfigurationGenerated bool                 `json:"taskConfigurationGenerated"`
		StorageInitialized         bool                 `json:"storageInitialized"`
		Linked                     bool                 `json:"linked"`
		HardwareQualified          bool                 `json:"hardwareQualified"`
		Result                     *compiler.TaskRecord `json:"result"`
	}{SchemaVersion: 1, Format: "cj1-task-record-containers", SourceSHA256: doc.SHA256, Model: *model, TimerCounterMode: *timer, TaskRecordGenerated: true, Result: result}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}
