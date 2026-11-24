package main

import (
	"encoding/json"
	"testing"

	"github.com/joyautomation/nautilus/omron/compiler"
)

func TestOmronCompileFBSectionsCLI(t *testing.T) {
	for _, spec := range []struct{ file, model string }{{"cj1m.cxp", "CJ1M CPU21"}, {"cj1h.cxp", "CJ1H-H CPU65"}} {
		file := "../../omron/cxp/testdata/" + spec.file
		args := []string{"-model", spec.model, "-resource", "Resource[0]", "-timer-mode", "bcd", "-number", "BLINK=0", file}
		data, err := captureOmronBodyCommand(t, omronCompileFBDefinitionSection, args)
		if err != nil {
			t.Fatal(err)
		}
		var report struct {
			Format, SourceSHA256                                                                                                                                  string
			DefinitionNumbersExplicit, NextEnableExplicit, InstanceDefinitionReferencesChecked, SavedPlacementUsed, StorageInitialized, Linked, HardwareQualified bool
			Result                                                                                                                                                compiler.FBDefinitionSection
		}
		if err := json.Unmarshal(data, &report); err != nil {
			t.Fatal(err)
		}
		if report.Format != "cj1-fb-definition-section-containers" || len(report.SourceSHA256) != 64 || !report.DefinitionNumbersExplicit || report.NextEnableExplicit || report.InstanceDefinitionReferencesChecked || report.SavedPlacementUsed || report.StorageInitialized || report.Linked || report.HardwareQualified || len(report.Result.Words) != 50 {
			t.Fatal("invalid definition section report", string(data))
		}
		args = append(args[:len(args)-1], "-next-enable", "0,0,0,0,0,0,0", file)
		data, err = captureOmronBodyCommand(t, omronCompileFBPrefix, args)
		if err != nil {
			t.Fatal(err)
		}
		var prefix struct {
			Format                                                                                                                                                string
			DefinitionNumbersExplicit, NextEnableExplicit, InstanceDefinitionReferencesChecked, SavedPlacementUsed, StorageInitialized, Linked, HardwareQualified bool
			Result                                                                                                                                                compiler.FBPrefix
		}
		if err := json.Unmarshal(data, &prefix); err != nil {
			t.Fatal(err)
		}
		if prefix.Format != "cj1-fb-prefix-containers" || !prefix.DefinitionNumbersExplicit || !prefix.NextEnableExplicit || !prefix.InstanceDefinitionReferencesChecked || !prefix.SavedPlacementUsed || prefix.StorageInitialized || prefix.Linked || prefix.HardwareQualified || len(prefix.Result.Words) != 166 || prefix.Result.InstanceOffsetWords != 50 {
			t.Fatal("invalid prefix report", string(data))
		}
	}
}

func TestOmronCompileFBSectionsRejectsInvalidControls(t *testing.T) {
	file := "../../omron/cxp/testdata/cj1m.cxp"
	base := []string{"-model", "CJ1M CPU21", "-resource", "Resource[0]", "-timer-mode", "bcd"}
	for _, number := range []string{"", "BLINK", "BLINK=-1", "BLINK=1024", "BLINK=01", "blink=0", "Extra=0"} {
		args := append(append([]string{}, base...), "-number", number, file)
		data, err := captureOmronBodyCommand(t, omronCompileFBDefinitionSection, args)
		if err == nil || len(data) != 0 {
			t.Fatal("invalid numbering emitted section", number, string(data), err)
		}
	}
	for _, options := range [][]string{
		{"-number", "BLINK=0", "-number", "BLINK=1", file},
		{"-number", "BLINK=0", "-number", "Extra=1", file},
	} {
		data, err := captureOmronBodyCommand(t, omronCompileFBDefinitionSection, append(append([]string{}, base...), options...))
		if err == nil || len(data) != 0 {
			t.Fatal("duplicate/extra mapping emitted section", string(data), err)
		}
	}
	for _, next := range []string{"", "0,0", "0,0,0,0,0,0,1"} {
		args := append(append([]string{}, base...), "-number", "BLINK=0", "-next-enable", next, file)
		data, err := captureOmronBodyCommand(t, omronCompileFBPrefix, args)
		if err == nil || len(data) != 0 {
			t.Fatal("unqualified prefix state emitted output", next, string(data), err)
		}
	}
}
