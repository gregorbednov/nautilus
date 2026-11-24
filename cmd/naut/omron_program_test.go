package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/compiler"
)

func TestOmronCompileProgramBody(t *testing.T) {
	args := []string{"-model", "CJ1M CPU21", "-timer-mode", "bcd", "-program", "Resource[0].Programs.Program[1]", "../../omron/cxp/testdata/cj1m.cxp"}
	data, err := captureOmronBodyCommand(t, omronCompileProgramBody, args)
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		Format, Program, SourceSHA256, CodeHex                     string
		Body                                                       compiler.Body
		NativeTaskCompiled, FullProjectCompiled, HardwareQualified bool
	}
	if err = json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if report.Format != "cj1-program-source-containers" || report.Program != args[5] || len(report.Body.Words) != 133 || len(report.CodeHex) != 133*8 || !report.Body.HasEnd || report.NativeTaskCompiled || report.FullProjectCompiled || report.HardwareQualified || report.SourceSHA256 != "b819ef1bbc3e7179dccb021b382410b807e164d4fd5c1c07ac14e356ff4dcef0" {
		t.Fatalf("unexpected program report %s", data)
	}
	withSections := append([]string{"-section-markers"}, args...)
	data, err = captureOmronBodyCommand(t, omronCompileProgramBody, withSections)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if report.Format != "cj1-sectioned-program-containers" || len(report.Body.Words) != 139 || len(report.Body.Generated) != 2 || report.NativeTaskCompiled || report.FullProjectCompiled || report.HardwareQualified {
		t.Fatal("sectioned report qualification changed")
	}
	args[5] = "Resource[0].Programs.Program[0]"
	if data, err = captureOmronBodyCommand(t, omronCompileProgramBody, args); err == nil || len(data) != 0 || !strings.Contains(err.Error(), "graphical") {
		t.Fatal("FB call silently omitted", err)
	}
	for _, bad := range [][]string{nil, {"-model", "CJ1M CPU21", "../../omron/cxp/testdata/cj1m.cxp"}, {"-model", "CJ1M CPU21", "-program", "missing", "../../omron/cxp/testdata/cj1m.cxp"}} {
		if data, err := captureOmronBodyCommand(t, omronCompileProgramBody, bad); err == nil || len(data) != 0 {
			t.Fatal("invalid selection emitted output", err)
		}
	}
}
