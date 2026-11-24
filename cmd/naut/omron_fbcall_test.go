package main

import (
	"encoding/json"
	"testing"

	"github.com/joyautomation/nautilus/omron/cxp"
)

func TestOmronFBCallCLI(t *testing.T) {
	const program = "Resource[0].Programs.Program[0]"
	const rung = program + ".Sections.Sec[0].ProgramData.R[4]"
	const file = "../../omron/cxp/testdata/cj1m.cxp"
	data, err := captureOmronBodyCommand(t, omronFBCall, []string{"-program", program, "-rung", rung, file})
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		SchemaVersion                                   int
		Format, SourceSHA256                            string
		Compiled, TypeCodesQualified, HardwareQualified bool
		DeclarationTypesIdentified                      bool
		Plan                                            cxp.FBCallPlan
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != 1 || report.Format != "cxp-fb-call-plan" || report.SourceSHA256 != "b819ef1bbc3e7179dccb021b382410b807e164d4fd5c1c07ac14e356ff4dcef0" || report.Compiled || report.TypeCodesQualified || report.HardwareQualified || !report.DeclarationTypesIdentified || report.Plan.Instance != "BLINK_L1" || len(report.Plan.Inputs) != 3 || report.Plan.Inputs[2].Value.Operand != "4.14" || report.Plan.Inputs[0].Declaration.TypeName != "INT" {
		t.Fatal("invalid call plan report", string(data))
	}
	for _, args := range [][]string{nil, {file}, {"-program", program, file}, {"-program", program, "-rung", program, file}} {
		data, err := captureOmronBodyCommand(t, omronFBCall, args)
		if err == nil || len(data) != 0 {
			t.Fatal("invalid inspection emitted a partial plan", args, string(data), err)
		}
	}
}
