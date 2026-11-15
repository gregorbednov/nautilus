package main

import (
	"encoding/json"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
)

func TestOmronCompileFBDefinitionBodyCLI(t *testing.T) {
	const definition = "Resource[0].FunctionBlocks.FunctionBlockWrapper[0].FunctionBlock"
	for _, spec := range []struct{ file, model string }{{"cj1m.cxp", codec.CJ1MCPU21}, {"cj1h.cxp", codec.CJ1HCPU65}} {
		file := "../../omron/cxp/testdata/" + spec.file
		data, err := captureOmronBodyCommand(t, omronCompileFBDefinitionBody, []string{"-model", spec.model, "-definition", definition, "-timer-mode", "bcd", file})
		if err != nil {
			t.Fatal(err)
		}
		var report struct {
			Format, SourceSHA256, Model, TimerCounterMode                      string
			RelativeFrameAllocated, InstancesPlaced, Linked, HardwareQualified bool
			Result                                                             compiler.FBDefinitionBody
		}
		if err := json.Unmarshal(data, &report); err != nil {
			t.Fatal(err)
		}
		if report.Format != "cj1-fb-definition-body-containers" || len(report.SourceSHA256) != 64 || report.Model != spec.model || report.TimerCounterMode != "bcd" || !report.RelativeFrameAllocated || report.InstancesPlaced || report.Linked || report.HardwareQualified || report.Result.Definition != definition || len(report.Result.Body.Words) != 19 || len(report.Result.Body.Spans) != 15 || len(report.Result.Allocation.Variables) != 9 {
			t.Fatal("invalid definition compilation report", string(data))
		}
		for _, args := range [][]string{nil, {file}, {"-model", spec.model, "-definition", definition, file}, {"-model", "unknown", "-definition", definition, "-timer-mode", "bcd", file}, {"-model", spec.model, "-definition", definition, "-timer-mode", "binary", file}} {
			data, err := captureOmronBodyCommand(t, omronCompileFBDefinitionBody, args)
			if err == nil || len(data) != 0 {
				t.Fatal("invalid/partial body report", args, string(data), err)
			}
		}
	}
}
