package main

import (
	"encoding/json"
	"testing"

	"github.com/joyautomation/nautilus/omron/compiler"
)

func TestOmronImageSettingsAuditRetainsConflicts(t *testing.T) {
	args := []string{"-model", "CJ1M CPU21", "-resource", "Resource[0]", "-settings", imageSettingsFile, "../../omron/cxp/testdata/cj1m.cxp"}
	raw, err := captureOmronBodyCommand(t, omronAuditImageSettings, args)
	if err == nil {
		t.Fatal("unqualified audit succeeded")
	}
	var r compiler.ImageSettingsAudit
	if err = json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err, string(raw))
	}
	if r.BR2DefaultCandidate == nil || r.FBSupportProfileVerified || r.Format != "cj1-source-settings-audit" || r.DeclarationRows != 7 || len(r.DeclarationValues) != 5 || r.SourceSettingsVerified || r.NetworkAccess || r.WriteQualified {
		t.Fatal("incorrect audit", r)
	}
	for _, args := range [][]string{nil, {"-endpoint", "10.0.174.37:9600"}, {"-model", "CJ1M CPU21", "-settings", imageSettingsFile, "missing.cxt"}} {
		if raw, err = captureOmronBodyCommand(t, omronAuditImageSettings, args); err == nil || len(raw) != 0 {
			t.Fatal("invalid command published report")
		}
	}
}
