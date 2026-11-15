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

func omronCompileFBInstance(args []string) error {
	return omronCompileFBInstanceCommand(args, false)
}

func omronCompileFBInstances(args []string) error {
	return omronCompileFBInstanceCommand(args, true)
}

func omronCompileFBInstanceCommand(args []string, all bool) error {
	command, scopeFlag := "compile-fb-instance", "instance"
	if all {
		command, scopeFlag = "compile-fb-instances", "resource"
	}
	flags := flag.NewFlagSet("omron "+command, flag.ContinueOnError)
	model := flags.String("model", "", "explicit qualified CPU model")
	instance := flags.String(scopeFlag, "", "fully qualified source scope")
	timerMode := flags.String("timer-mode", "", "explicit timer mode (bcd)")
	encoding := flags.String("encoding", "auto", "CXT text encoding")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 || *model == "" || *instance == "" || *timerMode != "bcd" {
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
	format := "cj1-fb-instance-record-containers"
	if all {
		result, err = compiler.CompileCXPFBInstances(profile, doc, *instance)
		format = "cj1-fb-instance-record-set"
	} else {
		result, err = compiler.CompileCXPFBInstance(profile, doc, *instance)
	}
	if err != nil {
		return err
	}
	report := struct {
		SchemaVersion           int    `json:"schemaVersion"`
		Format                  string `json:"format"`
		SourceSHA256            string `json:"sourceSHA256"`
		Model                   string `json:"model"`
		TimerCounterMode        string `json:"timerCounterMode"`
		SavedPlacementUsed      bool   `json:"savedPlacementUsed"`
		StorageInitialized      bool   `json:"storageInitialized"`
		GlobalPlacementAudited  bool   `json:"globalPlacementAudited"`
		InstanceIdentityAudited bool   `json:"instanceIdentityAudited"`
		InstanceOverlapAudited  bool   `json:"instanceOverlapAudited"`
		Linked                  bool   `json:"linked"`
		HardwareQualified       bool   `json:"hardwareQualified"`
		Result                  any    `json:"result"`
	}{SchemaVersion: 1, Format: format, SourceSHA256: doc.SHA256, Model: *model, TimerCounterMode: *timerMode, SavedPlacementUsed: true, InstanceIdentityAudited: all, InstanceOverlapAudited: all, Result: result}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}
