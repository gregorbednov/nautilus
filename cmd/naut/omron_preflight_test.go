package main

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/commission"
	"github.com/joyautomation/nautilus/omron/fins"
)

func preflightSnapshotFile(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../omron/fins/testdata/cpu-unit-data-prefix-observations.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct{ RawPrefixHex, StatusHex string }
	}
	if err = json.Unmarshal(b, &corpus); err != nil {
		t.Fatal(err)
	}
	raw, err := hex.DecodeString(corpus.Cases[0].RawPrefixHex)
	if err != nil {
		t.Fatal(err)
	}
	id, err := fins.DecodeIdentityData(raw)
	if err != nil {
		t.Fatal(err)
	}
	raw, err = hex.DecodeString(corpus.Cases[0].StatusHex)
	if err != nil {
		t.Fatal(err)
	}
	status, err := fins.DecodeStatusData(raw)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(commission.Snapshot{CPU: id, Status: status})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "status.json")
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestOmronRetainedProgramPreflight(t *testing.T) {
	file := "../../omron/compiler/testdata/n1-no-fb-cj1h-h.n1"
	snapshot := preflightSnapshotFile(t)
	args := []string{"-model", codec.CJ1MCPU21, "-snapshot", snapshot, "-backup", file, "-backup-check", file, file}
	data, err := captureOmronBodyCommand(t, omronPreflightProgramWrite, args)
	if err == nil {
		t.Fatal("archived snapshot authorized write")
	}
	var report commission.ProgramPreflight
	if err = json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if report.CPU.Model != "CJ1M_CPU21" || report.CPU.ReportedProgramAreaBytes != 20480 || report.Status.Mode != 4 || !report.BackupPairEqual || !report.RetainedEvidenceOnly || report.ReadyForWrite || report.WriteQualified || report.NetworkAccess || report.HardwareQualified {
		t.Fatal("wrong retained preflight", report)
	}
	states := map[string]string{}
	for _, c := range report.Checks {
		states[c.ID] = c.State
	}
	if states["declared-target-model"] != "passed" || states["program-capacity-upper-bound"] != "passed" || states["program-mode"] != "blocked" || states["unit-version-profile"] != "unverified" {
		t.Fatal("required distinctions missing", states)
	}
	args[1] = codec.CJ1HCPU65
	data, err = captureOmronBodyCommand(t, omronPreflightProgramWrite, args)
	if err == nil {
		t.Fatal("wrong CPU accepted")
	}
	if err = json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	for _, c := range report.Checks {
		if c.ID == "declared-target-model" && c.State != "blocked" {
			t.Fatal("wrong CPU not reported")
		}
	}
}

func TestOmronPreflightRejectsMissingOrConnectionInputs(t *testing.T) {
	file := "../../omron/compiler/testdata/n1-no-fb-cj1h-h.n1"
	for _, args := range [][]string{nil, {"-model", codec.CJ1MCPU21, file}, {"-endpoint", "10.0.174.37:9600", file}, {"-execute", file}} {
		if data, err := captureOmronBodyCommand(t, omronPreflightProgramWrite, args); err == nil || len(data) != 0 {
			t.Fatal("invalid preflight published", args)
		}
	}
}

func TestOmronPreflightRecompilesRecipeWithoutAuthorizingWrite(t *testing.T) {
	file := "../../omron/compiler/testdata/n1-no-fb-cj1h-h.n1"
	recipe := []string{"-source", "../../omron/compiler/testdata/n1-no-fb.cxt", "-settings", "../../omron/compiler/testdata/n1-finalization-probe-settings.json", "-resource", "Resource[0]", "-timer-mode", "bcd"}
	args := []string{"-model", codec.CJ1MCPU21, "-snapshot", preflightSnapshotFile(t), "-backup", file, "-backup-check", file}
	data, err := captureOmronBodyCommand(t, omronPreflightProgramWrite, append(append(args, recipe...), file))
	if err == nil {
		t.Fatal("recipe evidence authorized write")
	}
	var report commission.ProgramPreflight
	if err = json.Unmarshal(data, &report); err != nil {
		t.Fatal(err, string(data))
	}
	if report.BuildVerification == nil || report.BuildVerification.Model != codec.CJ1MCPU21 || report.BuildVerification.Method != "local-recompile-exact-bytes-v1" || report.ReadyForWrite || report.NetworkAccess {
		t.Fatal("missing/overstated recipe evidence", report)
	}
	states := map[string]string{}
	for _, c := range report.Checks {
		states[c.ID] = c.State
	}
	if states["candidate-build-provenance"] != "passed" || states["source-settings-agreement"] != "unverified" {
		t.Fatal("recipe/settings claims conflated")
	}
	for _, bad := range [][]string{{"-encoding", "auto"}, {"-source", recipe[1]}, {"-settings", recipe[3], "-resource", "Resource[0]", "-timer-mode", "bcd"}} {
		if got, err := captureOmronBodyCommand(t, omronPreflightProgramWrite, append(append(append([]string{}, args...), bad...), file)); err == nil || len(got) != 0 {
			t.Fatal("incomplete recipe emitted report")
		}
	}
	changed, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	changed[0] ^= 1
	path := filepath.Join(t.TempDir(), "changed.n1")
	if err = os.WriteFile(path, changed, 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := captureOmronBodyCommand(t, omronPreflightProgramWrite, append(append(append([]string{}, args...), recipe...), path)); err == nil || len(got) != 0 {
		t.Fatal("changed candidate accepted")
	}
}
