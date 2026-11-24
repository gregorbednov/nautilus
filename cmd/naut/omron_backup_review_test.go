package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/joyautomation/nautilus/omron/commission"
)

func TestOmronBackupReviewMatchesFilesWithoutWriteApproval(t *testing.T) {
	snapshot := preflightSnapshotFile(t)
	image := "../../omron/compiler/testdata/n1-no-fb-cj1h-h.n1"
	args := []string{"-model", "CJ1M CPU21", "-before", snapshot, "-after", snapshot, image, image}
	data, err := captureOmronBodyCommand(t, omronAuditProgramBackup, args)
	if err != nil {
		t.Fatal(err)
	}
	var r commission.ProgramBackupReview
	if err = json.Unmarshal(data, &r); err != nil {
		t.Fatal(err)
	}
	if !r.ReadPairConsistent || r.FullControllerBackup || r.RestoreQualified || r.ReadyForWrite || r.NetworkAccess || r.IndependentReadsVerified {
		t.Fatal("consistency granted permission", r)
	}
	changed, err := os.ReadFile(image)
	if err != nil {
		t.Fatal(err)
	}
	changed[0] ^= 1
	path := filepath.Join(t.TempDir(), "changed.um")
	if err = os.WriteFile(path, changed, 0600); err != nil {
		t.Fatal(err)
	}
	args[len(args)-1] = path
	data, err = captureOmronBodyCommand(t, omronAuditProgramBackup, args)
	if err == nil {
		t.Fatal("changed file accepted")
	}
	if err = json.Unmarshal(data, &r); err != nil || r.ReadPairConsistent {
		t.Fatal("conflict report missing")
	}
	for _, bad := range [][]string{nil, {"-endpoint", "10.0.174.37:9600"}, {"-execute"}} {
		if data, err = captureOmronBodyCommand(t, omronAuditProgramBackup, bad); err == nil || len(data) != 0 {
			t.Fatal("invalid input published report")
		}
	}
}
