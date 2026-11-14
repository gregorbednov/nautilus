package main

import (
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/cxp"
)

func captureOmronCompilation(t *testing.T, args []string) ([]byte, error) {
	t.Helper()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stdout
	os.Stdout = write
	defer func() { os.Stdout = previous; read.Close(); write.Close() }()
	err = omronCompileRungs(args)
	write.Close()
	os.Stdout = previous
	data, readErr := io.ReadAll(read)
	if readErr != nil {
		t.Fatal(readErr)
	}
	return data, err
}

func TestOmronCompileRungsCLI(t *testing.T) {
	data, err := os.ReadFile("../../omron/cxp/testdata/cj1m.cxp")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := cxp.Read(data)
	if err != nil {
		t.Fatal(err)
	}
	const path = "Resource[0].Programs.Program[0].Sections.Sec[0].ProgramData.R[0]"
	if err := doc.SetRung(path, "LD W0.00\nOUT W0.01"); err != nil {
		t.Fatal(err)
	}
	cxt, err := doc.MarshalCXT()
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "project.cxt")
	if err := os.WriteFile(file, cxt, 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"-model", "CJ1M CPU21", "-rung", path, file}
	output, err := captureOmronCompilation(t, args)
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Format              string
		SourceSHA256        string
		Program             string
		Body                compiler.Body
		CodeHex             string
		FullProjectCompiled bool
		HardwareQualified   bool
	}
	if err := json.Unmarshal(output, &report); err != nil {
		t.Fatal(err)
	}
	bytes, err := hex.DecodeString(report.CodeHex)
	if err != nil || report.Format != "cj1-selected-rung-containers" || len(report.SourceSHA256) != 64 || report.Program != "Resource[0].Programs.Program[0]" || report.CodeHex != "e205e000fa05e001" || len(bytes) != 8 || report.Body.HasEnd || report.FullProjectCompiled || report.HardwareQualified {
		t.Fatalf("wrong selected-body report: %s", output)
	}
	for _, bad := range [][]string{
		{}, {file}, {"-model", "CJ1M CPU21", file},
		{"-model", "CJ1M CPU22", "-rung", path, file},
		{"-model", "CJ1M CPU21", "-timer-mode", "automatic", "-rung", path, file},
		{"-model", "CJ1M CPU21", "-rung", path, "-rung", path, file},
		{"-model", "CJ1M CPU21", "-rung", "missing", file},
	} {
		if output, err := captureOmronCompilation(t, bad); err == nil || len(output) != 0 {
			t.Fatalf("emitted output for invalid selection %v: %s/%v", bad, output, err)
		}
	}
	var program []cxp.Rung
	for _, rung := range doc.Summary().Rungs {
		if strings.HasPrefix(rung.Path, "Resource[0].Programs.Program[0].Sections.") {
			program = append(program, rung)
		}
	}
	if len(program) < 3 {
		t.Fatal("missing selection control fixture")
	}
	if output, err := captureOmronCompilation(t, []string{"-model", "CJ1M CPU21", "-rung", program[0].Path, "-rung", program[2].Path, file}); err == nil || len(output) != 0 {
		t.Fatal("silently skipped intervening rung", err)
	}
	if err := doc.SetRung(program[1].Path, "LD W0.02\nOUT W0.03"); err != nil {
		t.Fatal(err)
	}
	cxt, err = doc.MarshalCXT()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, cxt, 0600); err != nil {
		t.Fatal(err)
	}
	// Flag order cannot silently reorder project execution.
	output, err = captureOmronCompilation(t, []string{"-model", "CJ1M CPU21", "-rung", program[1].Path, "-rung", program[0].Path, file})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(output, &report); err != nil {
		t.Fatal(err)
	}
	if report.CodeHex != "e205e000fa05e001e205e002fa05e003" {
		t.Fatal("rungs reordered by flag order", report.CodeHex)
	}
	graphical := ""
	otherProgram := ""
	for _, network := range doc.Summary().Rungs {
		if len(network.Graphic) > 0 && strings.HasPrefix(network.Path, "Resource[0].Programs.Program[0].Sections.") {
			graphical = network.Path
		}
		if strings.HasPrefix(network.Path, "Resource[0].Programs.Program[1].Sections.") {
			otherProgram = network.Path
		}
	}
	if graphical == "" || otherProgram == "" {
		t.Fatal("missing graphical/multiple-program selection fixtures")
	}
	if output, err := captureOmronCompilation(t, []string{"-model", "CJ1M CPU21", "-rung", graphical, file}); err == nil || len(output) != 0 || !strings.Contains(err.Error(), "graphical-rung") {
		t.Fatal("graphical rung emitted partial code", err)
	}
	if output, err := captureOmronCompilation(t, []string{"-model", "CJ1M CPU21", "-rung", path, "-rung", otherProgram, file}); err == nil || len(output) != 0 || !strings.Contains(err.Error(), "one program") {
		t.Fatal("mixed program selection emitted code", err)
	}
	if err := doc.SetRung(path, "LD W0.00\nLD TR0\nOUT W0.01"); err != nil {
		t.Fatal(err)
	}
	cxt, err = doc.MarshalCXT()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, cxt, 0600); err != nil {
		t.Fatal(err)
	}
	if output, err := captureOmronCompilation(t, args); err == nil || len(output) != 0 || !strings.Contains(err.Error(), "tr-undefined") {
		t.Fatal("invalid flow emitted code", err)
	}
}
