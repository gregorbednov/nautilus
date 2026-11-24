package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/joyautomation/nautilus/omron/cxp"
)

func TestOmronProjectConversionRequirement(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "conversions")
	if err := omronNewProject([]string{"-model", "CJ1M CPU21", "-timer-mode", "bcd", dir}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "program.cxt")
	source := strings.Replace(string(omronStarterSource), `SL:="END"`,
		"SL:=\"LD W0.00\n@num4(604) D100 D200\nEND\"", 1)
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	raw, err := captureOmronBodyCommand(t, func(args []string) error { return omronSource("inspect", args) }, []string{path})
	if err != nil {
		t.Fatal(err)
	}
	var report cxp.Summary
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Rungs) != 1 || len(report.Rungs[0].Instructions) != 3 {
		t.Fatalf("unexpected source summary: %s", raw)
	}
	lines := report.Rungs[0].Instructions
	if lines[0].MinimumCJ1UnitVersion != "" || lines[1].MinimumCJ1UnitVersion != "4.0" ||
		lines[2].MinimumCJ1UnitVersion != "" || lines[1].Line != lines[0].Line+1 {
		t.Fatalf("missing or misplaced requirement: %+v", lines)
	}
	if err := omronBuildProject([]string{dir}); err == nil || !strings.Contains(err.Error(), "NUM4 requires CJ1 unit version 4.0") {
		t.Fatalf("expected conversion diagnostic, got %v", err)
	}
}

func TestOmronProjectCheckVersion(t *testing.T) {
	const fixtures = "../../omron/compiler/testdata/"
	for _, tc := range []struct{ source, settings, version, sha string }{
		{"cj1h-bool-control-unit30.cxp", "n1-bool-control-settings.json", "3.0", "1f29fa66b64fc843cbd27b89ad3e73de9bd9d44e65c851dbc94c225f691622d0"},
		{"cj1h-inout-v4.cxp", "n1-inout-v4-settings.json", "3.0", ""},
		{"cj1h-inout-v4.cxp", "n1-inout-v4-settings.json", "4.0", "8a5cba1a0b147ec3dce7507311b75b9de4833857239d3596f5e15679156683d7"},
	} {
		dir := filepath.Join(t.TempDir(), "project")
		err := omronNewProject([]string{"-model", "CJ1H-H CPU65", "-timer-mode", "bcd", "-check-unit-version", tc.version,
			"-source", fixtures + tc.source, "-settings", fixtures + tc.settings, dir})
		if err != nil {
			t.Fatal(err)
		}
		_, err = captureOmronBodyCommand(t, omronBuildProject, []string{dir})
		if tc.sha == "" {
			if err == nil || !strings.Contains(err.Error(), "LD_Probe.SharedBit") {
				t.Fatal("missing target diagnostic", err)
			}
			if _, err := os.Stat(filepath.Join(dir, "build")); !os.IsNotExist(err) {
				t.Fatal("failed build published output", err)
			}
			journal := filepath.Join(dir, "journal")
			err = omronDownload([]string{"-project", "-endpoint", "127.0.0.1:1", "-execute",
				"-target", filepath.Join(dir, "missing-status.json"), "-journal", journal, dir})
			if err == nil || !strings.Contains(err.Error(), "LD_Probe.SharedBit") {
				t.Fatal("download skipped the project version check", err)
			}
			if _, err := os.Stat(journal); !os.IsNotExist(err) {
				t.Fatal("invalid download prepared a session", err)
			}
		} else if err != nil {
			t.Fatal(err)
		} else if _, err := os.Stat(filepath.Join(dir, "build", tc.sha+".n1")); err != nil {
			t.Fatal(err)
		}
	}
}

func TestOmronProjectBuildAndImport(t *testing.T) {
	for _, model := range []string{"CJ1M CPU21", "CJ1H-H CPU65"} {
		for _, tc := range []struct{ mode, input, hash string }{
			{"bcd", "", ""},
			{"binary", "", ""},
			{"bcd", "../../omron/compiler/testdata/n1-source-invocation.cxt", "0345bfbcfcedbc88cd90759cf30e7948ecad94d805cb1e5e040acd101dece502"},
			{"bcd", "../../omron/cxp/testdata/cj1m.cxp", ""},
		} {
			dir := filepath.Join(t.TempDir(), "plant 1")
			args := []string{"-model", model, "-timer-mode", tc.mode}
			if tc.input != "" {
				args = append(args, "-source", tc.input, "-settings", imageSettingsFile)
			}
			args = append(args, "--", dir)
			if err := omronNewProject(args); err != nil {
				t.Fatal(err)
			}
			if filepath.Ext(tc.input) == ".cxp" {
				source, err := os.ReadFile(filepath.Join(dir, "program.cxt"))
				if err != nil || !utf8.Valid(source) || !bytes.Contains(source, []byte("Лифт")) {
					t.Fatal("import did not preserve UTF-8 project comments", err)
				}
			}
			if err := omronNewProject(args); err == nil {
				t.Fatal("existing directory accepted")
			}
			var report struct {
				Output            string
				Image             struct{ SHA256 string }
				HardwareQualified bool
			}
			build := func() []byte {
				t.Helper()
				raw, err := captureOmronBodyCommand(t, omronBuildProject, []string{dir})
				if err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(raw, &report); err != nil {
					t.Fatal(err)
				}
				if report.HardwareQualified || filepath.Base(report.Output) != report.Image.SHA256+".n1" {
					t.Fatal("unexpected build report")
				}
				result, err := os.ReadFile(report.Output)
				if err != nil {
					t.Fatal(err)
				}
				return result
			}
			got := build()
			if !bytes.Equal(got, build()) {
				t.Fatal("repeat build differs")
			}
			if tc.hash != "" && report.Image.SHA256 != tc.hash {
				t.Fatal("import differs from native")
			}
			if tc.input == "" && tc.mode == "bcd" {
				want, err := os.ReadFile("../../omron/compiler/testdata/n1-no-fb-cj1m.n1")
				if err != nil || !bytes.Equal(got, want) {
					t.Fatal("starter differs from native", err)
				}
			}
			if tc.input != "" {
				out := filepath.Join(t.TempDir(), "direct.n1")
				_, err := captureOmronBodyCommand(t, omronCompileImage, []string{"-model", model, "-timer-mode", tc.mode, "-resource", "Resource[0]", "-settings", imageSettingsFile, "-o", out, tc.input})
				if err != nil {
					t.Fatal(err)
				}
				want, err := os.ReadFile(out)
				if err != nil || !bytes.Equal(got, want) {
					t.Fatal("import changed executable source", err)
				}
			}
			if err := os.WriteFile(report.Output, []byte("damaged"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := captureOmronBodyCommand(t, omronBuildProject, []string{dir}); err == nil {
				t.Fatal("accepted damaged cached output")
			}
		}
	}
}
