package main

import (
	"encoding/json"
	"testing"

	"github.com/joyautomation/nautilus/omron/compiler"
)

func TestOmronCompileTaskRecordCLI(t *testing.T) {
	for _, spec := range []struct{ file, model string }{{"cj1m.cxp", "CJ1M CPU21"}, {"cj1h.cxp", "CJ1H-H CPU65"}} {
		args := []string{"-model", spec.model, "-program", "Resource[0].Programs.Program[1]", "-timer-mode", "bcd", "../../omron/cxp/testdata/" + spec.file}
		data, err := captureOmronBodyCommand(t, omronCompileTaskRecord, args)
		if err != nil {
			t.Fatal(err)
		}
		var report struct {
			Format, SourceSHA256                                                                           string
			TaskRecordGenerated, TaskConfigurationGenerated, StorageInitialized, Linked, HardwareQualified bool
			Result                                                                                         compiler.TaskRecord
		}
		if err := json.Unmarshal(data, &report); err != nil {
			t.Fatal(err)
		}
		if report.Format != "cj1-task-record-containers" || len(report.SourceSHA256) != 64 || !report.TaskRecordGenerated || report.TaskConfigurationGenerated || report.StorageInitialized || report.Linked || report.HardwareQualified || len(report.Result.RecordWords) != 176 || report.Result.Source.Number != 257 || len(report.Result.Code.Words) != 139 {
			t.Fatal("invalid task report", string(data))
		}
	}
	for _, args := range [][]string{nil, {"-model", "CJ1M CPU21", "-program", "missing", "-timer-mode", "bcd", "../../omron/cxp/testdata/cj1m.cxp"}, {"-model", "CJ1M CPU21", "-program", "Resource[0].Programs.Program[0]", "-timer-mode", "binary", "../../omron/cxp/testdata/cj1m.cxp"}} {
		data, err := captureOmronBodyCommand(t, omronCompileTaskRecord, args)
		if err == nil || len(data) != 0 {
			t.Fatal("invalid task request emitted output", string(data), err)
		}
	}
}
