package main

import (
	"encoding/json"
	"errors"
	"flag"
	"os"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/commission"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/um"
)

func omronPreflightProgramWrite(args []string) error {
	flags := flag.NewFlagSet("omron preflight-program-write", flag.ContinueOnError)
	model := flags.String("model", "", "declared compiler model; not inferred from N1")
	snapshotPath := flags.String("snapshot", "", "retained status command JSON containing cpu/status raw data")
	backupPath := flags.String("backup", "", "first retained program-only UM backup")
	backupCheckPath := flags.String("backup-check", "", "second retained program file for comparison")
	sourcePath := flags.String("source", "", "optional source project to recompile locally; requires settings, resource and timer-mode")
	settingsPath := flags.String("settings", "", "explicit recipe settings JSON")
	resource := flags.String("resource", "", "explicit resource path for recipe reproduction")
	timer := flags.String("timer-mode", "", "explicit BCD timer mode for recipe reproduction")
	encoding := flags.String("encoding", "auto", "source encoding for recipe reproduction")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 || *model == "" || *snapshotPath == "" || *backupPath == "" || *backupCheckPath == "" {
		return errors.New("expected -model, -snapshot, -backup, -backup-check and one raw N1 candidate; offline only")
	}
	recipe := *sourcePath != "" || *settingsPath != "" || *resource != "" || *timer != ""
	explicitEncoding := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "encoding" {
			explicitEncoding = true
		}
	})
	if recipe && (*sourcePath == "" || *settingsPath == "" || *resource == "" || *timer != "bcd") || !recipe && explicitEncoding {
		return errors.New("recipe reproduction requires -source, -settings, -resource and -timer-mode bcd; encoding requires a recipe")
	}
	image, err := omronAuditReadFile(flags.Arg(0), um.MaxSize)
	if err != nil {
		return err
	}
	snapshot, err := readOmronCommissionSnapshot(*snapshotPath)
	if err != nil {
		return err
	}
	backup, err := omronAuditReadFile(*backupPath, um.MaxSize)
	if err != nil {
		return err
	}
	backupCheck, err := omronAuditReadFile(*backupCheckPath, um.MaxSize)
	if err != nil {
		return err
	}
	var build *compiler.VerifiedImageBuild
	if recipe {
		source, err := readOmronFile(*sourcePath)
		if err != nil {
			return err
		}
		rawSettings, err := omronAuditReadFile(*settingsPath, 1<<20)
		if err != nil {
			return err
		}
		settings, err := parseOmronImageSettings(rawSettings)
		if err != nil {
			return err
		}
		build, err = compiler.VerifyCXPImageBuild(codec.Profile{Model: *model, TimerCounterMode: codec.TimerModeBCD}, source, *encoding, *resource, settings, image)
		if err != nil {
			return err
		}
	}
	report, err := commission.AssessProgramWriteWithBuild(*model, image, snapshot, backup, backupCheck, build)
	if err != nil {
		return err
	}
	output := json.NewEncoder(os.Stdout)
	output.SetIndent("", "  ")
	if err = output.Encode(report); err != nil {
		return err
	}
	if !report.ReadyForWrite {
		return errors.New("offline preflight remains blocked; inspect reported conflicts and unverified prerequisites")
	}
	return nil
}
