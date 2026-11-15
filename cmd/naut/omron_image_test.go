package main

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/joyautomation/nautilus/omron/um"
)

func TestOmronImageCLI(t *testing.T) {
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stdout
	os.Stdout = write
	defer func() { os.Stdout = previous }()
	err = omronImage([]string{"-format", "n1", "../../omron/um/testdata/cj1h-baseline.n1"})
	write.Close()
	os.Stdout = previous
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(read)
	read.Close()
	if err != nil {
		t.Fatal(err)
	}
	var report um.N1Report
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if report.Format != "cx-programmer-n1" || len(report.Tasks) != 2 || report.Tasks[0].NameASCII != "MotorDrive_LIFT_1" || report.HardwareQualified {
		t.Fatalf("unexpected report: %s", data)
	}
	for _, args := range [][]string{nil, {"-format", "n2", "missing"}, {"../../omron/cxp/testdata/cj1h.cxp"}} {
		if err := omronImage(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestOmronImageFunctionBlocksCLI(t *testing.T) {
	data, err := captureOmronBodyCommand(t, omronImage, []string{"-function-blocks", "../../omron/um/testdata/cj1h-baseline.n1"})
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		um.N1Report
		FunctionBlocks um.N1FunctionBlocks
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if len(report.FunctionBlocks.Definitions) != 1 || report.FunctionBlocks.Definitions[0].NameASCII != "BLINK" || len(report.FunctionBlocks.Instances) != 2 || report.HardwareQualified || report.FunctionBlocks.HardwareQualified {
		t.Fatal("invalid FB inspection report")
	}
}
