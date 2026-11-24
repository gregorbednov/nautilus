package commission_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/commission"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/fins"
)

func observation(t *testing.T, name string) commission.Snapshot {
	t.Helper()
	b, err := os.ReadFile("../fins/testdata/cpu-unit-data-prefix-observations.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Cases []struct{ Name, RawPrefixHex, StatusHex string }
	}
	if err = json.Unmarshal(b, &corpus); err != nil {
		t.Fatal(err)
	}
	for _, r := range corpus.Cases {
		if r.Name == name {
			raw, err := hex.DecodeString(r.RawPrefixHex)
			if err != nil {
				t.Fatal(err)
			}
			id, err := fins.DecodeIdentityData(raw)
			if err != nil {
				t.Fatal(err)
			}
			raw, err = hex.DecodeString(r.StatusHex)
			if err != nil {
				t.Fatal(err)
			}
			status, err := fins.DecodeStatusData(raw)
			if err != nil {
				t.Fatal(err)
			}
			return commission.Snapshot{CPU: id, Status: status}
		}
	}
	t.Fatal("missing retained CPU")
	return commission.Snapshot{}
}

func candidate(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("../compiler/testdata/n1-no-fb-cj1h-h.n1")
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func checkState(t *testing.T, r *commission.ProgramPreflight, id string) string {
	t.Helper()
	for _, c := range r.Checks {
		if c.ID == id {
			return c.State
		}
	}
	t.Fatal("missing check", id)
	return ""
}

func TestPreflightRetainedCPUsDecodeCapacityWithoutUnitVersionGuess(t *testing.T) {
	for _, spec := range []struct {
		name, model string
		capacity    uint32
	}{{"cj1m", codec.CJ1MCPU21, 20480}, {"cj1h", codec.CJ1HCPU65, 245760}} {
		t.Run(spec.name, func(t *testing.T) {
			image := candidate(t)
			before := bytes.Clone(image)
			r, err := commission.AssessProgramWrite(spec.model, image, observation(t, spec.name), image, bytes.Clone(image))
			if err != nil {
				t.Fatal(err)
			}
			if r.CPU.ReportedProgramAreaBytes != spec.capacity || r.ObservedCompilerModel != spec.model || checkState(t, r, "declared-target-model") != "passed" || checkState(t, r, "program-capacity-upper-bound") != "passed" || checkState(t, r, "program-backup-pair") != "passed" {
				t.Fatal("retained consistency check failed", r)
			}
			if checkState(t, r, "program-mode") != "blocked" || checkState(t, r, "unit-version-profile") != "unverified" || !r.RetainedEvidenceOnly || r.ReadyForWrite || r.NetworkAccess || r.HardwareQualified || r.WriteQualified || !bytes.Equal(image, before) {
				t.Fatal("archive promoted into write permission", r)
			}
		})
	}
}

func TestPreflightKnownChecksCanPassWithoutGrantingWrite(t *testing.T) {
	snapshot := observation(t, "cj1m")
	// Controlled synthetic mode variant, not an observed PROGRAM-mode device.
	snapshot.Status.Raw[1] = 0
	var err error
	snapshot.Status, err = fins.DecodeStatusData(snapshot.Status.Raw)
	if err != nil {
		t.Fatal(err)
	}
	image := candidate(t)
	r, err := commission.AssessProgramWrite(codec.CJ1MCPU21, image, snapshot, image, image)
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range r.Checks {
		if check.State == "blocked" {
			t.Fatal("known consistency unexpectedly blocked", check)
		}
	}
	if r.ReadyForWrite || r.WriteQualified || len(r.Blockers) == 0 || checkState(t, r, "observation-authentication-and-freshness") != "unverified" || checkState(t, r, "candidate-build-provenance") != "unverified" || checkState(t, r, "full-backup-and-recovery") != "unverified" {
		t.Fatal("synthetic favorable facts bypassed unknown prerequisites")
	}
}

func TestPreflightRejectsConflictsAndUsesRawEvidence(t *testing.T) {
	image := candidate(t)
	cases := []struct {
		name, check string
		change      func(*commission.Snapshot, []byte)
	}{
		{"summary-spoof", "snapshot-summary", func(s *commission.Snapshot, _ []byte) { s.CPU.Model = "CJ1H_CPU65H"; s.Status.Mode = 0 }},
		{"reported-capacity-too-small", "program-capacity-upper-bound", func(s *commission.Snapshot, _ []byte) { s.CPU.Raw[80] = 0; s.CPU.Raw[81] = 1 }},
		{"unknown-capacity", "program-capacity-upper-bound", func(s *commission.Snapshot, _ []byte) { s.CPU.Raw[80] = 0; s.CPU.Raw[81] = 0 }},
		{"changed-backup", "program-backup-pair", func(_ *commission.Snapshot, b []byte) { b[0] ^= 1 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := observation(t, "cj1m")
			backup := bytes.Clone(image)
			tc.change(&snapshot, backup)
			r, err := commission.AssessProgramWrite(codec.CJ1MCPU21, image, snapshot, image, backup)
			if err != nil {
				t.Fatal(err)
			}
			state := checkState(t, r, tc.check)
			if state == "passed" || r.ReadyForWrite || r.WriteQualified {
				t.Fatal("conflict ignored", r)
			}
			if tc.name == "summary-spoof" && (r.CPU.Model != "CJ1M_CPU21" || r.Status.Mode != 4) {
				t.Fatal("convenience fields overrode raw evidence")
			}
		})
	}
	r, err := commission.AssessProgramWrite(codec.CJ1HCPU65, image, observation(t, "cj1m"), image, image)
	if err != nil || checkState(t, r, "declared-target-model") != "blocked" {
		t.Fatal("wrong CPU accepted")
	}
	r, err = commission.AssessProgramWrite(codec.CJ1MCPU21, image, observation(t, "cj1m"), []byte("bad"), []byte("bad"))
	if err != nil || checkState(t, r, "program-backup-pair") != "blocked" || !r.BackupPairEqual || r.ProgramBackups[0].ParseableN1 {
		t.Fatal("equal garbage treated as backup")
	}
}

func TestPreflightMalformedInputs(t *testing.T) {
	image := candidate(t)
	snapshot := observation(t, "cj1m")
	if _, err := commission.AssessProgramWrite("CJ1M CPU22", image, snapshot, image, image); err == nil {
		t.Fatal("unqualified model accepted")
	}
	if _, err := commission.AssessProgramWrite(codec.CJ1MCPU21, nil, snapshot, image, image); err == nil {
		t.Fatal("invalid candidate accepted")
	}
	snapshot.CPU.Raw = snapshot.CPU.Raw[:40]
	if _, err := commission.AssessProgramWrite(codec.CJ1MCPU21, image, snapshot, image, image); err == nil {
		t.Fatal("missing capacity bytes accepted")
	}
}

func TestPreflightRecompileEvidenceCannotBeReusedForAnotherTargetOrImage(t *testing.T) {
	source, err := os.ReadFile("../compiler/testdata/n1-no-fb.cxt")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../compiler/testdata/n1-finalization-probe-settings.json")
	if err != nil {
		t.Fatal(err)
	}
	var settings compiler.N1ImageSettings
	if err = json.Unmarshal(raw, &settings); err != nil {
		t.Fatal(err)
	}
	image := candidate(t)
	proof, err := compiler.VerifyCXPImageBuild(codec.Profile{Model: codec.CJ1HCPU65, TimerCounterMode: codec.TimerModeBCD}, source, "auto", "Resource[0]", settings, image)
	if err != nil {
		t.Fatal(err)
	}
	r, err := commission.AssessProgramWriteWithBuild(codec.CJ1HCPU65, image, observation(t, "cj1h"), image, image, proof)
	if err != nil {
		t.Fatal(err)
	}
	if checkState(t, r, "candidate-build-provenance") != "passed" || r.BuildVerification == nil || checkState(t, r, "source-settings-agreement") != "unverified" || r.ReadyForWrite || r.WriteQualified {
		t.Fatal("reproduction bypassed independent blockers", r)
	}
	r, err = commission.AssessProgramWriteWithBuild(codec.CJ1MCPU21, image, observation(t, "cj1m"), image, image, proof)
	if err != nil || checkState(t, r, "candidate-build-provenance") != "blocked" || r.BuildVerification != nil {
		t.Fatal("proof reused for another profile")
	}
	changed := bytes.Clone(image)
	changed[0] ^= 1
	r, err = commission.AssessProgramWriteWithBuild(codec.CJ1HCPU65, changed, observation(t, "cj1h"), image, image, proof)
	if err != nil || checkState(t, r, "candidate-build-provenance") != "blocked" || r.BuildVerification != nil {
		t.Fatal("proof reused for changed image")
	}
	var zero compiler.VerifiedImageBuild
	r, err = commission.AssessProgramWriteWithBuild(codec.CJ1HCPU65, image, observation(t, "cj1h"), image, image, &zero)
	if err != nil || checkState(t, r, "candidate-build-provenance") != "blocked" {
		t.Fatal("zero proof accepted")
	}
}
