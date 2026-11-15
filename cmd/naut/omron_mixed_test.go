package main

import (
	"encoding/json"
	"testing"

	"github.com/joyautomation/nautilus/omron/compiler"
)

func TestOmronMixedProgramBodyCLI(t *testing.T) {
	for _, spec := range []struct{ file, model string }{{"cj1m.cxp", "CJ1M CPU21"}, {"cj1h.cxp", "CJ1H-H CPU65"}} {
		args := []string{"-model", spec.model, "-program", "Resource[0].Programs.Program[0]", "-timer-mode", "bcd", "-fb-calls", "-section-markers", "../../omron/cxp/testdata/" + spec.file}
		data, err := captureOmronBodyCommand(t, omronCompileProgramBody, args)
		if err != nil {
			t.Fatal(err)
		}
		var report struct {
			Format                                                     string
			Body                                                       compiler.Body
			FullProjectCompiled, NativeTaskCompiled, HardwareQualified bool
		}
		if err := json.Unmarshal(data, &report); err != nil {
			t.Fatal(err)
		}
		if report.Format != "cj1-mixed-sectioned-program-containers" || len(report.Body.Words) != 154 || !report.Body.HasEnd || len(report.Body.Generated) != 6 || report.FullProjectCompiled || report.NativeTaskCompiled || report.HardwareQualified {
			t.Fatal("invalid mixed report", string(data))
		}
	}
	for _, mode := range []string{"", "binary"} {
		args := []string{"-model", "CJ1M CPU21", "-program", "Resource[0].Programs.Program[0]", "-timer-mode", mode, "-fb-calls", "../../omron/cxp/testdata/cj1m.cxp"}
		data, err := captureOmronBodyCommand(t, omronCompileProgramBody, args)
		if err == nil || len(data) != 0 {
			t.Fatal("unqualified mixed timer profile emitted output", mode, err)
		}
	}
}
