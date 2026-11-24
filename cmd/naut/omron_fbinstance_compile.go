package main

import (
	"encoding/json"
	"errors"
	"flag"
	"os"
	"strconv"
	"strings"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/cxp"
)

func omronCompileFBInstance(args []string) error {
	return omronCompileFBInstanceCommand(args, false, false)
}

func omronCompileFBInstances(args []string) error {
	return omronCompileFBInstanceCommand(args, true, false)
}

func omronCompileFBInstanceSection(args []string) error {
	return omronCompileFBInstanceCommand(args, true, true)
}

func omronCompileFBInstanceCommand(args []string, all, section bool) error {
	command, scopeFlag := "compile-fb-instance", "instance"
	if all {
		command, scopeFlag = "compile-fb-instances", "resource"
	}
	if section {
		command = "compile-fb-instance-section"
	}
	flags := flag.NewFlagSet("omron "+command, flag.ContinueOnError)
	var unit string
	if !all {
		flags.StringVar(&unit, "unit-version", "", "explicit target version for a version-qualified instance")
	}
	var nextText string
	if section {
		flags.StringVar(&nextText, "next-enable", "", "explicit seven decimal values; only all-zero state is qualified")
	}
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
	var next [7]uint32
	if section {
		var err error
		next, err = parseOmronNextEnable(nextText)
		if err != nil {
			return err
		}
	}
	data, err := readOmronFile(flags.Arg(0))
	if err != nil {
		return err
	}
	doc, err := cxp.ReadEncoding(data, *encoding)
	if err != nil {
		return err
	}
	profile := codec.Profile{Model: *model, UnitVersion: unit, TimerCounterMode: codec.TimerModeBCD}
	var result any
	format := "cj1-fb-instance-record-containers"
	if section {
		result, err = compiler.CompileCXPFBInstanceSection(profile, doc, *instance, next)
		format = "cj1-fb-instance-section-containers"
	} else if all {
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
		UnitVersion             string `json:"unitVersion,omitempty"`
		SavedPlacementUsed      bool   `json:"savedPlacementUsed"`
		StorageInitialized      bool   `json:"storageInitialized"`
		GlobalPlacementAudited  bool   `json:"globalPlacementAudited"`
		InstanceIdentityAudited bool   `json:"instanceIdentityAudited"`
		InstanceOverlapAudited  bool   `json:"instanceOverlapAudited"`
		NextEnableExplicit      bool   `json:"nextEnableExplicit"`
		SectionGenerated        bool   `json:"sectionGenerated"`
		Linked                  bool   `json:"linked"`
		HardwareQualified       bool   `json:"hardwareQualified"`
		Result                  any    `json:"result"`
	}{SchemaVersion: 1, Format: format, SourceSHA256: doc.SHA256, Model: *model, TimerCounterMode: *timerMode, UnitVersion: unit, SavedPlacementUsed: true, InstanceIdentityAudited: all, InstanceOverlapAudited: all, NextEnableExplicit: section, SectionGenerated: section, Result: result}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func parseOmronNextEnable(text string) ([7]uint32, error) {
	var next [7]uint32
	parts := strings.Split(text, ",")
	if len(parts) != 7 {
		return next, errors.New("section requires explicit -next-enable 0,0,0,0,0,0,0")
	}
	for i, part := range parts {
		value, err := strconv.ParseUint(part, 10, 32)
		if err != nil || strconv.FormatUint(value, 10) != part {
			return [7]uint32{}, errors.New("next-enable values must be canonical unsigned decimal integers")
		}
		next[i] = uint32(value)
	}
	return next, nil
}
