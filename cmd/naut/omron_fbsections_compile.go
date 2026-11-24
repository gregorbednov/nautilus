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

func omronCompileFBDefinitionSection(args []string) error {
	return omronCompileFBSections(args, false, false)
}
func omronCompileFBPrefix(args []string) error     { return omronCompileFBSections(args, true, false) }
func omronCompileResourceBody(args []string) error { return omronCompileFBSections(args, true, true) }

func omronCompileFBSections(args []string, prefix, resourceBody bool) error {
	command := "compile-fb-definition-section"
	if prefix {
		command = "compile-fb-prefix"
	}
	if resourceBody {
		command = "compile-resource-body"
	}
	flags := flag.NewFlagSet("omron "+command, flag.ContinueOnError)
	model := flags.String("model", "", "explicit qualified CPU model")
	resource := flags.String("resource", "", "fully qualified resource path")
	timer := flags.String("timer-mode", "", "explicit BCD timer mode")
	encoding := flags.String("encoding", "auto", "CXT text encoding")
	numbers := map[string]int{}
	flags.Func("number", "repeat NAME=NUMBER; explicit definition record IDs0..1023", func(value string) error {
		name, text, ok := strings.Cut(value, "=")
		n, err := strconv.Atoi(text)
		_, duplicate := numbers[name]
		if !ok || name == "" || err != nil || n < 0 || n > 1023 || strconv.Itoa(n) != text || duplicate {
			return errors.New("number requires unique NAME=0..1023")
		}
		numbers[name] = n
		return nil
	})
	var nextText string
	if prefix {
		flags.StringVar(&nextText, "next-enable", "", "explicit zero next-enable state")
	}
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 || *model == "" || *resource == "" || *timer != "bcd" || len(numbers) == 0 {
		return errors.New(command + " requires -model MODEL -resource PATH -timer-mode bcd -number NAME=N FILE")
	}
	var next [7]uint32
	if prefix {
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
	profile := codec.Profile{Model: *model, TimerCounterMode: codec.TimerModeBCD}
	var result any
	format := "cj1-fb-definition-section-containers"
	if resourceBody {
		result, err = compiler.CompileCXPResourceBody(profile, doc, *resource, numbers, next)
		format = "cj1-resource-body-containers"
	} else if prefix {
		result, err = compiler.CompileCXPFBPrefix(profile, doc, *resource, numbers, next)
		format = "cj1-fb-prefix-containers"
	} else {
		result, err = compiler.CompileCXPFBDefinitionSection(profile, doc, *resource, numbers)
	}
	if err != nil {
		return err
	}
	report := struct {
		SchemaVersion                       int    `json:"schemaVersion"`
		Format                              string `json:"format"`
		SourceSHA256                        string `json:"sourceSHA256"`
		Model                               string `json:"model"`
		TimerCounterMode                    string `json:"timerCounterMode"`
		DefinitionNumbersExplicit           bool   `json:"definitionNumbersExplicit"`
		NextEnableExplicit                  bool   `json:"nextEnableExplicit"`
		InstanceDefinitionReferencesChecked bool   `json:"instanceDefinitionReferencesChecked"`
		InternalReferencesChecked           bool   `json:"internalReferencesChecked"`
		TaskDirectoryGenerated              bool   `json:"taskDirectoryGenerated"`
		GlobalHeaderGenerated               bool   `json:"globalHeaderGenerated"`
		TaskConfigurationGenerated          bool   `json:"taskConfigurationGenerated"`
		SavedPlacementUsed                  bool   `json:"savedPlacementUsed"`
		StorageInitialized                  bool   `json:"storageInitialized"`
		Linked                              bool   `json:"linked"`
		HardwareQualified                   bool   `json:"hardwareQualified"`
		Result                              any    `json:"result"`
	}{SchemaVersion: 1, Format: format, SourceSHA256: doc.SHA256, Model: *model, TimerCounterMode: *timer, DefinitionNumbersExplicit: true, NextEnableExplicit: prefix, InstanceDefinitionReferencesChecked: prefix, InternalReferencesChecked: resourceBody, TaskDirectoryGenerated: resourceBody,TaskConfigurationGenerated:resourceBody, SavedPlacementUsed: prefix, Result: result}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}
