package main

import (
	"encoding/json"
	"testing"

	"github.com/joyautomation/nautilus/omron/compiler"
)

func TestOmronTaskConfigurationCLI(t *testing.T) {
	for _, spec := range []struct{ file, model string }{{"cj1m.cxp", "CJ1M CPU21"}, {"cj1h.cxp", "CJ1H-H CPU65"}} {
		data, err := captureOmronBodyCommand(t, omronCompileTaskConfiguration, []string{"-model", spec.model, "-resource", "Resource[0]", "-timer-mode", "bcd", "../../omron/cxp/testdata/" + spec.file})
		if err != nil {
			t.Fatal(err)
		}
		var report struct {
			Format, Model, TimerCounterMode, SourceSHA256                                string
			TaskConfigurationGenerated, GlobalHeaderGenerated, Linked, HardwareQualified bool
			Result                                                                       compiler.TaskConfiguration
		}
		if err := json.Unmarshal(data, &report); err != nil {
			t.Fatal(err)
		}
		if report.Format != "cj1-task-configuration" || report.Model != spec.model || report.TimerCounterMode != "bcd" || len(report.SourceSHA256) != 64 || !report.TaskConfigurationGenerated || report.GlobalHeaderGenerated || report.Linked || report.HardwareQualified || len(report.Result.Words) != 288 || report.Result.HeaderOffsetWords != 42 || report.Result.Words[256] != 0xd100 || report.Result.Words[257] != 0xd101 {
			t.Fatal("invalid configuration report", string(data))
		}
	}
	for _, args := range [][]string{nil, {"-model", "CJ1M CPU21", "-resource", "Resource[1]", "-timer-mode", "bcd", "../../omron/cxp/testdata/cj1m.cxp"}, {"-model", "CJ1M CPU21", "-resource", "Resource[0]", "-timer-mode", "binary", "../../omron/cxp/testdata/cj1m.cxp"}} {
		data, err := captureOmronBodyCommand(t, omronCompileTaskConfiguration, args)
		if err == nil || len(data) != 0 {
			t.Fatal("invalid configuration request emitted output", args)
		}
	}
}
