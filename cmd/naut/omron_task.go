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
	return omronCompileTasks(args, false)
}

func omronCompileTaskBlock(args []string) error { return omronCompileTasks(args, true) }

func omronCompileTasks(args []string, block bool) error {
	command, scopeFlag := "compile-task-record", "program"
	if block {
		command, scopeFlag = "compile-task-block", "resource"
	}
	flags := flag.NewFlagSet("omron "+command, flag.ContinueOnError)
	model := flags.String("model", "", "explicit qualified CPU model")
	program := flags.String(scopeFlag, "", "fully qualified source scope")
	timer := flags.String("timer-mode", "", "explicit BCD timer mode")
	encoding := flags.String("encoding", "auto", "CXT text encoding")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 || *model == "" || *program == "" || *timer != "bcd" {
		return errors.New(command + " requires -model MODEL -" + scopeFlag + " PATH -timer-mode bcd FILE")
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
	format := "cj1-task-record-containers"
	if block {
		result, err = compiler.CompileCXPTaskBlock(profile, doc, *program)
		format = "cj1-task-block-containers"
	} else {
		result, err = compiler.CompileCXPTaskRecord(profile, doc, *program)
	}
	if err != nil {
		return err
	}
	report := struct {
		SchemaVersion              int    `json:"schemaVersion"`
		Format                     string `json:"format"`
		SourceSHA256               string `json:"sourceSHA256"`
		Model                      string `json:"model"`
		TimerCounterMode           string `json:"timerCounterMode"`
		TaskRecordGenerated        bool   `json:"taskRecordGenerated"`
		TaskRecordsGenerated       bool   `json:"taskRecordsGenerated"`
		TaskDirectoryGenerated     bool   `json:"taskDirectoryGenerated"`
		TaskConfigurationGenerated bool   `json:"taskConfigurationGenerated"`
		StorageInitialized         bool   `json:"storageInitialized"`
		Linked                     bool   `json:"linked"`
		HardwareQualified          bool   `json:"hardwareQualified"`
		Result                     any    `json:"result"`
	}{SchemaVersion: 1, Format: format, SourceSHA256: doc.SHA256, Model: *model, TimerCounterMode: *timer, TaskRecordGenerated: !block, TaskRecordsGenerated: block, TaskDirectoryGenerated: block, Result: result}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}
