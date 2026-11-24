package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/cxp"
)

//go:embed templates/cj1/program.cxt
var omronStarterSource []byte

//go:embed templates/cj1/settings.json
var omronStarterSettings []byte

type omronProject struct {
	SchemaVersion int    `json:"schemaVersion"`
	Model         string `json:"model"`
	TimerMode     string `json:"timerCounterMode"`
	Resource      string `json:"resource"`
	CheckVersion  string `json:"programCheckUnitVersion,omitempty"`
	SourceFormat  string `json:"sourceFormat,omitempty"`
}

func (p omronProject) validate() error {
	if p.SourceFormat != "" && (p.SourceFormat != "st-il" || p.Model != codec.CJ1MCPU21 || p.TimerMode != "bcd" || p.Resource != "Resource[0]" || p.CheckVersion == "4.0") {
		return errors.New("st-il projects require CJ1M CPU21, BCD, Resource[0] and version 3.0 or unspecified")
	}
	if p.CheckVersion != "" && p.CheckVersion != "3.0" && p.CheckVersion != "4.0" {
		return errors.New("programCheckUnitVersion must be 3.0 or 4.0")
	}
	if p.SchemaVersion != 1 || p.Resource == "" {
		return errors.New("CJ1 project requires schemaVersion 1 and a resource")
	}
	if p.Model != codec.CJ1MCPU21 && p.Model != codec.CJ1HCPU65 {
		return fmt.Errorf("unsupported CJ1 model %q", p.Model)
	}
	if p.TimerMode != "bcd" && p.TimerMode != "binary" {
		return errors.New("select timer mode bcd or binary")
	}
	return nil
}

func omronNewProject(args []string) error {
	p := omronProject{SchemaVersion: 1}
	f := flag.NewFlagSet("omron new-project", flag.ContinueOnError)
	f.StringVar(&p.Model, "model", "", "CJ1 CPU model")
	f.StringVar(&p.CheckVersion, "check-unit-version", "", "CJ1 source compatibility check (3.0 or 4.0)")
	f.StringVar(&p.TimerMode, "timer-mode", "", "bcd or binary")
	f.StringVar(&p.Resource, "resource", "Resource[0]", "resource to compile")
	source := f.String("source", "", "existing CXP or CXT project")
	settings := f.String("settings", "", "explicit image settings for imported source")
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 1 || (*source == "") != (*settings == "") {
		return errors.New("new-project needs a new directory; supply both -source and -settings when importing")
	}
	if err := p.validate(); err != nil {
		return err
	}
	program, options := omronStarterSource, omronStarterSettings
	if *source != "" {
		raw, err := readOmronFile(*source)
		if err != nil {
			return err
		}
		doc, err := cxp.Read(raw)
		if err != nil {
			return err
		}
		// The workspace is edited as UTF-8 even when the CXP used a Windows code page.
		doc.Encoding = "utf-8"
		program, err = doc.MarshalCXT()
		if err != nil {
			return err
		}
		options, err = readOmronFile(*settings)
		if err != nil {
			return err
		}
		if _, err := parseOmronImageSettings(options); err != nil {
			return err
		}
	}
	manifest, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	dir := f.Arg(0)
	if err := os.Mkdir(dir, 0755); err != nil {
		return err
	}
	// Publish the manifest last so a build cannot see an incomplete project.
	var published []string
	for _, item := range []struct {
		name string
		data []byte
	}{
		{"program.cxt", program}, {"settings.json", options}, {"cj1.json", append(manifest, '\n')},
	} {
		path := filepath.Join(dir, item.name)
		if err := writeNewOmronFile(path, item.data); err != nil {
			for _, owned := range published {
				_ = os.Remove(owned)
			}
			_ = os.Remove(dir)
			return err
		}
		published = append(published, path)
	}
	return nil
}

func omronBuildProject(args []string) error {
	if len(args) != 1 {
		return errors.New("build-project requires a project directory")
	}
	dir := args[0]
	p, err := readOmronProject(dir)
	if err != nil {
		return err
	}
	if p.SourceFormat == "st-il" {
		return buildSTILProject(dir, p)
	}
	return compileOmronProjectImage(p.Model, p.TimerMode, p.Resource, "auto", filepath.Join(dir, "settings.json"), filepath.Join(dir, "program.cxt"), "", filepath.Join(dir, "build"), p.CheckVersion)
}

func readOmronProject(dir string) (omronProject, error) {
	var p omronProject
	raw, err := readOmronFile(filepath.Join(dir, "cj1.json"))
	if err != nil {
		return p, err
	}
	if err := checkOmronImageJSON(json.NewDecoder(bytes.NewReader(raw)), 0); err != nil {
		return p, err
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return p, err
	}
	fields := []string{"schemaVersion", "model", "timerCounterMode", "resource"}
	if p.SourceFormat != "" {
		fields = append(fields, "sourceFormat")
	}
	if p.CheckVersion != "" {
		fields = append(fields, "programCheckUnitVersion")
	}
	if _, err := requiredOmronSettings(raw, fields...); err != nil {
		return p, err
	}
	return p, p.validate()
}
