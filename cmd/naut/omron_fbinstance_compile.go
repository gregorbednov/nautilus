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
	flags := flag.NewFlagSet("omron compile-fb-instance", flag.ContinueOnError)
	model := flags.String("model", "", "explicit qualified CPU model")
	instance := flags.String("instance", "", "fully qualified saved global instance path")
	timerMode := flags.String("timer-mode", "", "explicit timer mode (bcd)")
	encoding := flags.String("encoding", "auto", "CXT text encoding")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 || *model == "" || *instance == "" || *timerMode != "bcd" {
		return errors.New("compile-fb-instance requires -model MODEL -instance PATH -timer-mode bcd FILE")
	}
	data, err := readOmronFile(flags.Arg(0))
	if err != nil {
		return err
	}
	doc, err := cxp.ReadEncoding(data, *encoding)
	if err != nil {
		return err
	}
	result, err := compiler.CompileCXPFBInstance(codec.Profile{Model: *model, TimerCounterMode: codec.TimerModeBCD}, doc, *instance)
	if err != nil {
		return err
	}
	report := struct {
		SchemaVersion          int                  `json:"schemaVersion"`
		Format                 string               `json:"format"`
		SourceSHA256           string               `json:"sourceSHA256"`
		Model                  string               `json:"model"`
		TimerCounterMode       string               `json:"timerCounterMode"`
		SavedPlacementUsed     bool                 `json:"savedPlacementUsed"`
		StorageInitialized     bool                 `json:"storageInitialized"`
		GlobalPlacementAudited bool                 `json:"globalPlacementAudited"`
		Linked                 bool                 `json:"linked"`
		HardwareQualified      bool                 `json:"hardwareQualified"`
		Result                 *compiler.FBInstance `json:"result"`
	}{SchemaVersion: 1, Format: "cj1-fb-instance-record-containers", SourceSHA256: doc.SHA256, Model: *model, TimerCounterMode: *timerMode, SavedPlacementUsed: true, Result: result}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}
