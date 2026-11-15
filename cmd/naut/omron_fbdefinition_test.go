package main

import (
	"encoding/json"
	"testing"

	"github.com/joyautomation/nautilus/omron/cxp"
)

func TestOmronFBDefinitionCLI(t *testing.T) {
	const definition = "Resource[0].FunctionBlocks.FunctionBlockWrapper[0].FunctionBlock"
	for _, file := range []string{"../../omron/cxp/testdata/cj1m.cxp", "../../omron/cxp/testdata/cj1h.cxp"} {
		data, err := captureOmronBodyCommand(t, omronFBDefinition, []string{"-definition", definition, file})
		if err != nil {
			t.Fatal(err)
		}
		var report struct {
			SchemaVersion                                   int
			Format, SourceSHA256                            string
			Compiled, AddressesAllocated, HardwareQualified bool
			Source                                          cxp.FBDefinitionSource
		}
		if err := json.Unmarshal(data, &report); err != nil {
			t.Fatal(err)
		}
		if report.SchemaVersion != 1 || report.Format != "cxp-fb-definition-source" || len(report.SourceSHA256) != 64 || report.Compiled || report.AddressesAllocated || report.HardwareQualified || report.Source.Name != "BLINK" || len(report.Source.Rungs) != 6 || report.Source.Rungs[0].Instructions[3].Operands[0].TypeName != "TIMER" {
			t.Fatal("invalid definition report", string(data))
		}
		for _, args := range [][]string{nil, {file}, {"-definition", "invalid", file}, {"-definition", definition}} {
			data, err := captureOmronBodyCommand(t, omronFBDefinition, args)
			if err == nil || len(data) != 0 {
				t.Fatal("partial or invalid report", string(data), err)
			}
		}
	}
}
