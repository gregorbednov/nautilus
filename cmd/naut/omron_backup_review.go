package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"

	"github.com/joyautomation/nautilus/omron/commission"
	"github.com/joyautomation/nautilus/omron/um"
)

func readOmronCommissionSnapshot(path string) (commission.Snapshot, error) {
	var snapshot commission.Snapshot
	raw, err := omronAuditReadFile(path, 1<<20)
	if err != nil {
		return snapshot, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&snapshot); err != nil {
		return snapshot, err
	}
	var extra any
	if err = decoder.Decode(&extra); err != io.EOF {
		return snapshot, errors.New("snapshot must contain one JSON value")
	}
	return snapshot, nil
}
func omronAuditProgramBackup(args []string) error {
	flags := flag.NewFlagSet("omron audit-program-backup", flag.ContinueOnError)
	model := flags.String("model", "", "explicit compiler model")
	before := flags.String("before", "", "retained identity/status JSON before program reads")
	after := flags.String("after", "", "retained identity/status JSON after program reads")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 2 || *model == "" || *before == "" || *after == "" {
		return errors.New("expected -model, -before, -after and two UM files; offline only")
	}
	a, err := readOmronCommissionSnapshot(*before)
	if err != nil {
		return err
	}
	b, err := readOmronCommissionSnapshot(*after)
	if err != nil {
		return err
	}
	first, err := omronAuditReadFile(flags.Arg(0), um.MaxSize)
	if err != nil {
		return err
	}
	second, err := omronAuditReadFile(flags.Arg(1), um.MaxSize)
	if err != nil {
		return err
	}
	report, err := commission.ReviewProgramBackup(*model, a, b, first, second)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err = enc.Encode(report); err != nil {
		return err
	}
	if !report.ReadPairConsistent {
		return errors.New("retained program backup pair is inconsistent; inspect checks")
	}
	return nil
}
