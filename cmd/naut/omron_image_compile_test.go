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
