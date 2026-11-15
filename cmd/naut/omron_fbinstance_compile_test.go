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
