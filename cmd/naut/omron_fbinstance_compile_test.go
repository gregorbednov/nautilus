package main

import (
	"encoding/json"
	"github.com/joyautomation/nautilus/omron/compiler"
	"testing"
)

func TestOmronCompileFBInstanceCLI(t *testing.T) {
	for _, spec := range []struct{ file, model string }{{"cj1m.cxp", "CJ1M CPU21"}, {"cj1h.cxp", "CJ1H-H CPU65"}} {
		args := []string{"-model", spec.model, "-instance", "Resource[0].GlobalVariables.POUInstances.Instance[1]", "-timer-mode", "bcd", "../../omron/cxp/testdata/" + spec.file}
		data, err := captureOmronBodyCommand(t, omronCompileFBInstance, args)
		if err != nil {
			t.Fatal(err)
		}
		var report struct {
			Format, SourceSHA256                                                                      string
			SavedPlacementUsed, StorageInitialized, GlobalPlacementAudited, Linked, HardwareQualified bool
			Result                                                                                    compiler.FBInstance
		}
		if err := json.Unmarshal(data, &report); err != nil {
			t.Fatal(err)
		}
		if report.Format != "cj1-fb-instance-record-containers" || len(report.SourceSHA256) != 64 || !report.SavedPlacementUsed || report.StorageInitialized || report.GlobalPlacementAudited || report.Linked || report.HardwareQualified || len(report.Result.RecordWords) != 54 || len(report.Result.Body.Words) != 44 || report.Result.Source.Number != 3 {
			t.Fatalf("bad report: %s", data)
		}
	}
	for _, args := range [][]string{nil, {"-model", "CJ1M CPU21", "-instance", "missing", "-timer-mode", "bcd", "../../omron/cxp/testdata/cj1m.cxp"}, {"-model", "CJ1M CPU21", "-instance", "missing", "-timer-mode", "binary", "../../omron/cxp/testdata/cj1m.cxp"}} {
		data, err := captureOmronBodyCommand(t, omronCompileFBInstance, args)
		if err == nil || len(data) != 0 {
			t.Fatal("invalid request emitted output", err, string(data))
		}
	}
}

func TestOmronCompileFBInstancesCLI(t *testing.T) {
	for _, spec := range []struct{ file, model string }{{"cj1m.cxp", "CJ1M CPU21"}, {"cj1h.cxp", "CJ1H-H CPU65"}} {
		args := []string{"-model", spec.model, "-resource", "Resource[0]", "-timer-mode", "bcd", "../../omron/cxp/testdata/" + spec.file}
		data, err := captureOmronBodyCommand(t, omronCompileFBInstances, args)
		if err != nil {
			t.Fatal(err)
		}
		var report struct {
			Format, SourceSHA256                                                  string
			SavedPlacementUsed, InstanceIdentityAudited, InstanceOverlapAudited   bool
			StorageInitialized, GlobalPlacementAudited, Linked, HardwareQualified bool
			Result                                                                compiler.FBInstances
		}
		if err := json.Unmarshal(data, &report); err != nil {
			t.Fatal(err)
		}
		if report.Format != "cj1-fb-instance-record-set" || len(report.SourceSHA256) != 64 || !report.SavedPlacementUsed || !report.InstanceIdentityAudited || !report.InstanceOverlapAudited || report.StorageInitialized || report.GlobalPlacementAudited || report.Linked || report.HardwareQualified || report.Result.Resource != "Resource[0]" || len(report.Result.Instances) != 2 || len(report.Result.Regions) != 4 || report.Result.Instances[1].Source.Number != 3 {
			t.Fatal("invalid audit report", string(data))
		}
	}
	for _, args := range [][]string{nil, {"-model", "unknown", "-resource", "Resource[0]", "-timer-mode", "bcd", "../../omron/cxp/testdata/cj1m.cxp"}, {"-model", "CJ1M CPU21", "-resource", "Resource[1]", "-timer-mode", "bcd", "../../omron/cxp/testdata/cj1m.cxp"}, {"-model", "CJ1M CPU21", "-instance", "Resource[0]", "-timer-mode", "bcd", "../../omron/cxp/testdata/cj1m.cxp"}} {
		data, err := captureOmronBodyCommand(t, omronCompileFBInstances, args)
		if err == nil || len(data) != 0 {
			t.Fatal("invalid resource emitted output", err, string(data))
		}
	}
}

func TestOmronCompileFBInstanceSectionCLI(t *testing.T) {
	for _, spec := range []struct{ file, model string }{{"cj1m.cxp", "CJ1M CPU21"}, {"cj1h.cxp", "CJ1H-H CPU65"}} {
		args := []string{"-model", spec.model, "-resource", "Resource[0]", "-timer-mode", "bcd", "-next-enable", "0,0,0,0,0,0,0", "../../omron/cxp/testdata/" + spec.file}
		data, err := captureOmronBodyCommand(t, omronCompileFBInstanceSection, args)
		if err != nil {
			t.Fatal(err)
		}
		var report struct {
			Format                                                                                string
			SectionGenerated, NextEnableExplicit, InstanceIdentityAudited, InstanceOverlapAudited bool
			GlobalPlacementAudited, StorageInitialized, Linked, HardwareQualified                 bool
			Result                                                                                compiler.FBInstanceSection
		}
		if err := json.Unmarshal(data, &report); err != nil {
			t.Fatal(err)
		}
		if report.Format != "cj1-fb-instance-section-containers" || !report.SectionGenerated || !report.NextEnableExplicit || !report.InstanceIdentityAudited || !report.InstanceOverlapAudited || report.GlobalPlacementAudited || report.StorageInitialized || report.Linked || report.HardwareQualified || len(report.Result.Words) != 116 || len(report.Result.Records) != 2 {
			t.Fatal("invalid section report", string(data))
		}
	}
	for _, next := range []string{"", "0,0", "0,0,0,0,0,0,1", "0,0,0,0,0,0,-1", "0,0,0,0,0,0,01", "0,0,0,0,0,0,4294967296"} {
		args := []string{"-model", "CJ1M CPU21", "-resource", "Resource[0]", "-timer-mode", "bcd", "-next-enable", next, "../../omron/cxp/testdata/cj1m.cxp"}
		data, err := captureOmronBodyCommand(t, omronCompileFBInstanceSection, args)
		if err == nil || len(data) != 0 {
			t.Fatal("unqualified next state emitted output", next, string(data), err)
		}
	}
}
