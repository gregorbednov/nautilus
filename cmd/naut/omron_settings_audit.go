package main

import (
	"encoding/json"
	"errors"
	"flag"
	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"os"
)

func omronAuditImageSettings(args []string) error {
	flags := flag.NewFlagSet("omron audit-image-settings", flag.ContinueOnError)
	model := flags.String("model", "", "explicit recipe CPU model")
	resource := flags.String("resource", "", "selected resource")
	settingsPath := flags.String("settings", "", "explicit settings JSON")
	encoding := flags.String("encoding", "auto", "project text encoding")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 || *model == "" || *resource == "" || *settingsPath == "" {
		return errors.New("expected -model, -resource, -settings and one source project; offline only")
	}
	source, err := readOmronFile(flags.Arg(0))
	if err != nil {
		return err
	}
	raw, err := omronAuditReadFile(*settingsPath, 1<<20)
	if err != nil {
		return err
	}
	settings, err := parseOmronImageSettings(raw)
	if err != nil {
		return err
	}
	report, err := compiler.AuditCXPImageSettings(codec.Profile{Model: *model, TimerCounterMode: codec.TimerModeBCD}, source, *encoding, *resource, settings)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err = enc.Encode(report); err != nil {
		return err
	}
	return errors.New("source settings agreement and initialization remain unqualified; inspect audit checks")
}
