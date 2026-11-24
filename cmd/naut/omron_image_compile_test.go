package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/um"
)

const imageSettingsFile = "../../omron/compiler/testdata/n1-example-settings.json"

func TestOmronCompileSourceInvocationImageMatchesNativeGuards(t *testing.T) {
	for _, model := range []string{"CJ1M CPU21", "CJ1H-H CPU65"} {
		output := filepath.Join(t.TempDir(), "source-invocation.n1")
		args := []string{"-model", model, "-resource", "Resource[0]", "-timer-mode", "bcd", "-settings", imageSettingsFile, "-o", output, "../../omron/compiler/testdata/n1-source-invocation.cxt"}
		data, err := captureOmronBodyCommand(t, omronCompileImage, args)
		if err != nil {
			t.Fatal(err)
		}
		var report struct {
			Linked, HardwareQualified, StorageInitialized, SourceSettingsVerified bool
			Image                                                                 um.N1Report
		}
		if err = json.Unmarshal(data, &report); err != nil || !report.Linked || report.HardwareQualified || report.Image.HardwareQualified || report.StorageInitialized || report.SourceSettingsVerified {
			t.Fatal("scope", string(data), err)
		}
		raw, err := os.ReadFile(output)
		if err != nil || len(raw) != 3180 || fmt.Sprintf("%x", sha256.Sum256(raw)) != "0345bfbcfcedbc88cd90759cf30e7948ecad94d805cb1e5e040acd101dece502" {
			t.Fatal("CLI source invocation differs from native", err)
		}
	}
}

func TestOmronCompileBinaryImageCLIMatchesNative(t *testing.T) {
	for _, model := range []string{"CJ1M CPU21", "CJ1H-H CPU65"} {
		for _, spec := range []struct{ source, hash string }{
			{"n1-finalization-probe.cxt", "87ed301503b374ed2025377ade99897b5d6877e6e5f68d410258b03d3ff2a87a"},
			{"n1-finalization-boundary-probe.cxt", "c96592f79747caa284dfe1dbb2bd3db260bb5a448df8a8dd93572fe891ba8ecb"},
		} {
			output := filepath.Join(t.TempDir(), "binary.n1")
			args := []string{"-model", model, "-resource", "Resource[0]", "-timer-mode", "binary", "-settings", "../../omron/compiler/testdata/n1-finalization-probe-settings.json", "-o", output, "../../omron/compiler/testdata/" + spec.source}
			data, err := captureOmronBodyCommand(t, omronCompileImage, args)
			if err != nil {
				t.Fatal(err)
			}
			var report struct {
				TimerCounterMode  string
				HardwareQualified bool
				Image             um.N1Report
			}
			if err = json.Unmarshal(data, &report); err != nil || report.TimerCounterMode != "binary" || report.HardwareQualified || report.Image.HardwareQualified {
				t.Fatal("incorrect binary scope", string(data), err)
			}
			got, err := os.ReadFile(output)
			if err != nil || fmt.Sprintf("%x", sha256.Sum256(got)) != spec.hash {
				t.Fatal("binary CLI differs from native3/4", err)
			}
			// Actual FB-bearing source must still fail before publishing a file.
			args[7] = imageSettingsFile
			args[9] = filepath.Join(t.TempDir(), "rejected.n1")
			args[10] = "../../omron/cxp/testdata/cj1m.cxp"
			if data, err = captureOmronBodyCommand(t, omronCompileImage, args); err == nil || len(data) != 0 {
				t.Fatal("binary FB accepted")
			}
			if _, err = os.Stat(args[9]); !os.IsNotExist(err) {
				t.Fatal("failed binary build published an output", err)
			}
		}
	}
}

func TestOmronCompileImageCLIProducesExactNativeFiles(t *testing.T) {
	for _, spec := range []struct{ source, native, model, first string }{{"cj1m.cxp", "cj1m-input-bit.n1", "CJ1M CPU21", "LD W0.15\nOUT W0.01"}, {"cj1h.cxp", "cj1h-baseline.n1", "CJ1H-H CPU65", "LD W0.00\nOUT W0.01"}} {
		input, err := os.ReadFile("../../omron/cxp/testdata/" + spec.source)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := cxp.Read(input)
		if err != nil {
			t.Fatal(err)
		}
		if err = doc.SetRung("Resource[0].Programs.Program[0].Sections.Sec[0].ProgramData.R[0]", spec.first); err != nil {
			t.Fatal(err)
		}
		source, err := doc.MarshalCXT()
		if err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		file := filepath.Join(dir, "source.cxt")
		output := filepath.Join(dir, "project.n1")
		if err = os.WriteFile(file, source, 0600); err != nil {
			t.Fatal(err)
		}
		args := []string{"-model", spec.model, "-resource", "Resource[0]", "-timer-mode", "bcd", "-settings", imageSettingsFile, "-o", output, file}
		data, err := captureOmronBodyCommand(t, omronCompileImage, args)
		if err != nil {
			t.Fatal(err)
		}
		var report struct {
			Format, Model, TimerCounterMode, SourceSHA256, SettingsSHA256, Output                                                                                                                                       string
			GlobalHeaderGenerated, TaskConfigurationGenerated, InternalReferencesChecked, InstanceMemoryContainmentChecked, Linked, SourceSettingsVerified, StorageInitialized, GlobalStorageAudited, HardwareQualified bool
			Image                                                                                                                                                                                                       um.N1Report
		}
		if err = json.Unmarshal(data, &report); err != nil {
			t.Fatal(err)
		}
		settings, _ := os.ReadFile(imageSettingsFile)
		if report.Format != "cj1-n1-image" || report.Model != spec.model || report.TimerCounterMode != "bcd" || report.SourceSHA256 != fmt.Sprintf("%x", sha256.Sum256(source)) || report.SettingsSHA256 != fmt.Sprintf("%x", sha256.Sum256(settings)) || report.Output != output || !report.GlobalHeaderGenerated || !report.TaskConfigurationGenerated || !report.InternalReferencesChecked || !report.InstanceMemoryContainmentChecked || !report.Linked || report.SourceSettingsVerified || report.StorageInitialized || report.GlobalStorageAudited || report.HardwareQualified {
			t.Fatal("incorrect build report", string(data))
		}
		got, err := os.ReadFile(output)
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile("../../omron/um/testdata/" + spec.native)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) || report.Image.SHA256 != fmt.Sprintf("%x", sha256.Sum256(want)) {
			t.Fatal("CLI N1 differs from native")
		}
		if data, err = captureOmronBodyCommand(t, omronCompileImage, args); err == nil || len(data) != 0 {
			t.Fatal("existing output was overwritten")
		}
		after, _ := os.ReadFile(output)
		if !bytes.Equal(after, want) {
			t.Fatal("existing output changed")
		}
	}
}

func TestOmronCompileImageRejectsAmbiguousSettingsWithoutOutput(t *testing.T) {
	valid, err := os.ReadFile(imageSettingsFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{
		strings.Replace(string(valid), `"schemaVersion": 1`, `"schemaVersion": 2`, 1),
		strings.Replace(string(valid), `"copyProhibited": false,`, "", 1),
		strings.Replace(string(valid), `"copyProhibited": false`, `"copyProhibited": null`, 1),
		strings.Replace(string(valid), `"copyProhibited": false`, `"copyProhibited": false, "copyProhibited": true`, 1),
		strings.Replace(string(valid), `"copyProhibited": false`, `"copyProhibited": false, "COPYPROHIBITED": true`, 1),
		strings.Replace(string(valid), `"size": 0`, `"ignored": 0`, 1),
		strings.Replace(string(valid), `"BLINK": 0`, `"BLINK": null`, 1),
		strings.Replace(string(valid), `"address": "AC0"`, `"address": "D0"`, 1),
		strings.Replace(string(valid), `"size": 1024`, `"size": 1`, 1),
		string(valid) + " {}",
		strings.Repeat("[", 18) + "0" + strings.Repeat("]", 18),
	} {
		dir := t.TempDir()
		settings := filepath.Join(dir, "settings.json")
		output := filepath.Join(dir, "out.n1")
		if err = os.WriteFile(settings, []byte(bad), 0600); err != nil {
			t.Fatal(err)
		}
		data, err := captureOmronBodyCommand(t, omronCompileImage, []string{"-model", "CJ1M CPU21", "-resource", "Resource[0]", "-timer-mode", "bcd", "-settings", settings, "-o", output, "../../omron/cxp/testdata/cj1m.cxp"})
		if err == nil || len(data) != 0 {
			t.Fatal("ambiguous settings accepted", bad)
		}
		if _, err = os.Stat(output); !os.IsNotExist(err) {
			t.Fatal("failed compilation left an output", err)
		}
	}
}

func TestOmronCompileImageWithoutFB(t *testing.T) {
	settings, err := os.ReadFile(imageSettingsFile)
	if err != nil {
		t.Fatal(err)
	}
	settings = bytes.Replace(settings, []byte(`"BLINK": 0`), nil, 1)
	dir := t.TempDir()
	settingsPath, output := filepath.Join(dir, "settings.json"), filepath.Join(dir, "no-fb.n1")
	if err = os.WriteFile(settingsPath, settings, 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"-model", "CJ1H-H CPU65", "-resource", "Resource[0]", "-timer-mode", "bcd", "-settings", settingsPath, "-o", output, "../../omron/compiler/testdata/n1-no-fb.cxt"}
	data, err := captureOmronBodyCommand(t, omronCompileImage, args)
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Linked, HardwareQualified bool
		Image                     um.N1Report
	}
	if err = json.Unmarshal(data, &report); err != nil || !report.Linked || report.HardwareQualified || report.Image.ByteLength != 2624 {
		t.Fatal("incorrect no-FB build report", string(data), err)
	}
	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../omron/compiler/testdata/n1-no-fb-cj1h-h.n1")
	if err != nil || !bytes.Equal(got, want) {
		t.Fatal("no-FB CLI image differs from native", err)
	}
	// An explicit empty map is valid only when the source has no definitions.
	args[len(args)-1] = "../../omron/cxp/testdata/cj1h.cxp"
	args[9] = filepath.Join(dir, "must-not-exist.n1")
	if data, err = captureOmronBodyCommand(t, omronCompileImage, args); err == nil || len(data) != 0 {
		t.Fatal("empty map bypassed definition numbering")
	}
	if _, err = os.Stat(args[9]); !os.IsNotExist(err) {
		t.Fatal("failed compilation published a file", err)
	}
}
