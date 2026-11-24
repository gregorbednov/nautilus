package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/commission"
	"github.com/joyautomation/nautilus/omron/fins"
)

type parameterCaptureArea struct {
	name string
	code fins.ParameterArea
}

var capturedParameterAreas = []parameterCaptureArea{
	{"plc-setup", fins.ParameterPLCSetup}, {"io-table", fins.ParameterIOTable},
	{"routing", fins.ParameterRoutingTable}, {"cpu-bus", fins.ParameterCPUBusSetup},
}

type parameterCaptureIdentity struct{ Endpoint, RawSHA256 string }

type parameterDifference struct {
	Area                string `json:"area"`
	Bytes               int    `json:"bytes"`
	BaselineSHA256      string `json:"baselineSHA256"`
	ObservedSHA256      string `json:"observedSHA256"`
	ChangedBytes        int    `json:"changedBytes"`
	FirstDifferenceByte int    `json:"firstDifferenceByte"`
}

// Validate the capture against the retained bytes rather than trusting the
// original capture tool's comparison flags. Stable 0501 data and an endpoint
// label identify a comparable pair of archives, not a unique physical CPU.
func readParameterPair(dir, target, model string, area parameterCaptureArea) ([]byte, parameterCaptureIdentity, error) {
	var identity parameterCaptureIdentity
	dir = filepath.Join(dir, target+"-"+area.name)
	raw, err := omronAuditReadFile(filepath.Join(dir, "result.json"), 1<<20)
	if err != nil {
		return nil, identity, err
	}
	var meta struct {
		Before, After                          commission.Snapshot
		Target, Area, Code, Endpoint           string
		ExpectedBytes, FirstBytes, SecondBytes int
		FirstSHA256, SecondSHA256              string
	}
	if err = json.Unmarshal(raw, &meta); err != nil {
		return nil, identity, err
	}
	size := area.code.ByteLength()
	if meta.Area != area.name || meta.Target != target || meta.Code != fmt.Sprintf("%04x", area.code) || meta.Endpoint == "" ||
		meta.ExpectedBytes != size || meta.FirstBytes != size || meta.SecondBytes != size {
		return nil, identity, errors.New("area, target, endpoint or size disagrees with capture metadata")
	}
	cpu, err := fins.DecodeCPUUnitData(meta.Before.CPU.Raw)
	if err != nil {
		return nil, identity, err
	}
	if cpu.Model != model || !bytes.Equal(meta.Before.CPU.Raw, meta.After.CPU.Raw) ||
		meta.Before.CPU.Model != model || meta.After.CPU.Model != model ||
		meta.Before.CPU.Version != cpu.InternalSystemVersion || meta.After.CPU.Version != cpu.InternalSystemVersion {
		return nil, identity, errors.New("CPU identity mismatch")
	}
	first, err := omronAuditReadFile(filepath.Join(dir, "copy-1.bin"), int64(size))
	if err != nil {
		return nil, identity, err
	}
	second, err := omronAuditReadFile(filepath.Join(dir, "copy-2.bin"), int64(size))
	if err != nil {
		return nil, identity, err
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(first))
	if len(first) != size || !bytes.Equal(first, second) || hash != meta.FirstSHA256 || hash != meta.SecondSHA256 {
		return nil, identity, errors.New("the retained pair, lengths or recorded hashes disagree")
	}
	return first, parameterCaptureIdentity{meta.Endpoint, cpu.PayloadSHA256}, nil
}

func omronCompareParameterBackups(args []string) error {
	flags := flag.NewFlagSet("compare-parameter-backups", flag.ContinueOnError)
	model := flags.String("model", "", "CPU model in both parameter archives")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 2 {
		return errors.New("expected baseline and observed parameter archive directories")
	}
	var target, native string
	switch *model {
	case codec.CJ1MCPU21:
		target, native = "cj1m", "CJ1M_CPU21"
	case codec.CJ1HCPU65:
		target, native = "cj1h", "CJ1H_CPU65H"
	default:
		return errors.New("expected CJ1M CPU21 or CJ1H-H CPU65")
	}
	report := struct {
		Model            string                `json:"model"`
		Endpoint         string                `json:"recordedEndpoint"`
		Areas            []parameterDifference `json:"areas"`
		AllAreasEqual    bool                  `json:"allAreasEqual"`
		RestoreQualified bool                  `json:"restoreQualified"`
	}{Model: *model, AllAreasEqual: true}
	var expectedIdentity parameterCaptureIdentity
	for _, area := range capturedParameterAreas {
		var pair [2][]byte
		for index, dir := range flags.Args() {
			data, id, err := readParameterPair(dir, target, native, area)
			if err != nil {
				return fmt.Errorf("%s (%s): %w", dir, area.name, err)
			}
			if expectedIdentity.Endpoint == "" {
				expectedIdentity = id
			}
			if id != expectedIdentity {
				return fmt.Errorf("%s (%s): different recorded endpoint or CPU identity", dir, area.name)
			}
			pair[index] = data
		}
		row := parameterDifference{Area: area.name, Bytes: len(pair[0]), BaselineSHA256: fmt.Sprintf("%x", sha256.Sum256(pair[0])), ObservedSHA256: fmt.Sprintf("%x", sha256.Sum256(pair[1])), FirstDifferenceByte: -1}
		for i, b := range pair[0] {
			if b != pair[1][i] {
				if row.ChangedBytes == 0 {
					row.FirstDifferenceByte = i
				}
				row.ChangedBytes++
			}
		}
		report.Areas = append(report.Areas, row)
		report.AllAreasEqual = report.AllAreasEqual && row.ChangedBytes == 0
	}
	report.Endpoint = expectedIdentity.Endpoint
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		return err
	}
	if !report.AllAreasEqual {
		return errors.New("parameter area bytes differ")
	}
	return nil
}
