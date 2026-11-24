package main

import (
	"encoding/json"
	"testing"

	omrontransfer "github.com/joyautomation/nautilus/omron/transfer"
)

func TestOmronOfflineProgramPlan(t *testing.T) {
	file := "../../omron/compiler/testdata/n1-no-fb-cj1h-h.n1"
	data, err := captureOmronBodyCommand(t, omronPlanProgramWrite, []string{file})
	if err != nil {
		t.Fatal(err)
	}
	var p omrontransfer.ProgramPlan
	if err = json.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}
	if p.Format != "cj1-offline-program-write-plan" || p.NetworkAccess || p.WriteQualified || p.ImageModified || p.BlockBytes != 996 || len(p.Blocks) != 3 || p.Blocks[2].DataBytes != 632 {
		t.Fatal("incorrect offline plan")
	}
	if p.ServiceFields.Blocks64FromImageLength != 41 || p.ServiceFields.StoredBlockCountMatches || p.ServiceFields.Words[4] != 0 {
		t.Fatal("compiler header was stamped or observation omitted")
	}
	for _, args := range [][]string{nil, {"-endpoint", "10.0.174.37:9600", file}, {"-execute", file}, {"-block-bytes", "1000", file}, {"../../omron/cxp/testdata/cj1h.cxp"}} {
		if data, err := captureOmronBodyCommand(t, omronPlanProgramWrite, args); err == nil || len(data) != 0 {
			t.Fatal("invalid plan published", args)
		}
	}
}
