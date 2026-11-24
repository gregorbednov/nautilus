package main

import (
	"encoding/json"
	"testing"

	"github.com/joyautomation/nautilus/omron/compiler"
)

func TestOmronFBCallCompilationCLI(t *testing.T) {
	const program = "Resource[0].Programs.Program[0]"
	const rung = program + ".Sections.Sec[0].ProgramData.R[4]"
	const file = "../../omron/cxp/testdata/cj1m.cxp"
	data, err := captureOmronBodyCommand(t, omronCompileFBCallBody, []string{"-model", "CJ1M CPU21", "-program", program, "-rung", rung, file})
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Format, CodeHex                                            string
		Body                                                       compiler.Body
		FullProjectCompiled, NativeTaskCompiled, HardwareQualified bool
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if report.Format != "cj1-fb-call-body-containers" || len(report.Body.Words) != 27 || len(report.CodeHex) != 216 || len(report.Body.Generated) != 3 || len(report.Body.Bindings) != 3 || report.FullProjectCompiled || report.NativeTaskCompiled || report.HardwareQualified {
		t.Fatal("invalid fragment report", string(data))
	}
	for _, args := range [][]string{nil, {file}, {"-model", "CJ2", "-program", program, "-rung", rung, file}, {"-model", "CJ1M CPU21", "-program", program, "-rung", program, file}} {
		data, err := captureOmronBodyCommand(t, omronCompileFBCallBody, args)
		if err == nil || len(data) != 0 {
			t.Fatal("invalid call emitted partial report", args, string(data), err)
		}
	}
}
