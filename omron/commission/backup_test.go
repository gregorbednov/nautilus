package commission_test

import (
	"bytes"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/commission"
	"github.com/joyautomation/nautilus/omron/fins"
)

func TestBackupReviewConsistentRUNDoesNotAuthorizeRestore(t *testing.T) {
	a, b := observation(t, "cj1h"), observation(t, "cj1h")
	image := candidate(t)
	r, err := commission.ReviewProgramBackup(codec.CJ1HCPU65, a, b, image, bytes.Clone(image))
	if err != nil {
		t.Fatal(err)
	}
	if !r.ReadPairConsistent || !r.RetainedEvidenceOnly || r.StatusBefore.Mode != 4 || r.Programs[0].SHA256 != r.Programs[1].SHA256 || r.FullControllerBackup || r.RestoreQualified || r.HardwareQualified || r.ReadyForWrite || r.WriteQualified || r.NetworkAccess || r.IndependentReadsVerified || r.ObservationFreshnessVerified {
		t.Fatal("backup consistency promoted into qualification", r)
	}
	r.StatusBefore.Raw[1] = 0
	if a.Status.Raw[1] != 4 {
		t.Fatal("review aliases input")
	}
}
func TestBackupReviewStopsOnSnapshotOrProgramConflict(t *testing.T) {
	image := candidate(t)
	cases := []struct {
		name, check string
		change      func(*commission.Snapshot, []byte)
	}{
		{"summary", "snapshot-summaries", func(s *commission.Snapshot, _ []byte) { s.Status.Mode = 0 }},
		{"cpu-tail", "identity-raw-stable", func(s *commission.Snapshot, _ []byte) { s.CPU.Raw = append(s.CPU.Raw, 0) }},
		{"mode-change", "status-raw-stable", func(s *commission.Snapshot, _ []byte) {
			s.Status.Raw[1] = 2
			s.Status, _ = fins.DecodeStatusData(s.Status.Raw)
		}},
		{"service-word-change", "program-pair", func(_ *commission.Snapshot, b []byte) { b[0] ^= 1 }},
		{"capacity-change", "reported-capacity", func(s *commission.Snapshot, _ []byte) { s.CPU.Raw[80] = 0; s.CPU.Raw[81] = 1 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a, b := observation(t, "cj1h"), observation(t, "cj1h")
			second := bytes.Clone(image)
			tc.change(&b, second)
			r, err := commission.ReviewProgramBackup(codec.CJ1HCPU65, a, b, image, second)
			if err != nil {
				t.Fatal(err)
			}
			if r.ReadPairConsistent || r.RestoreQualified {
				t.Fatal("conflict ignored")
			}
			found := false
			for _, c := range r.Checks {
				if c.ID == tc.check {
					found = true
					if c.State != "blocked" {
						t.Fatal("wrong check state", c)
					}
				}
			}
			if !found {
				t.Fatal("missing conflict")
			}
		})
	}
	if r, err := commission.ReviewProgramBackup(codec.CJ1MCPU21, observation(t, "cj1h"), observation(t, "cj1h"), image, image); err != nil || r.ReadPairConsistent {
		t.Fatal("wrong declared CPU accepted")
	}
	if r, err := commission.ReviewProgramBackup(codec.CJ1HCPU65, observation(t, "cj1h"), observation(t, "cj1h"), []byte("bad"), []byte("bad")); err != nil || r.ReadPairConsistent {
		t.Fatal("equal garbage accepted")
	}
	bad := observation(t, "cj1h")
	bad.CPU.Raw = bad.CPU.Raw[:40]
	if _, err := commission.ReviewProgramBackup(codec.CJ1HCPU65, bad, bad, image, image); err == nil {
		t.Fatal("partial CPU data accepted")
	}
}
