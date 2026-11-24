package commission

import (
	"bytes"
	"errors"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/fins"
	"github.com/joyautomation/nautilus/omron/um"
)

// ProgramBackupReview checks retained files, never the live device or the
// independence/freshness of reads. A consistent pair is not a restoration proof.
type ProgramBackupReview struct {
	Format                       string             `json:"format"`
	DeclaredModel                string             `json:"declaredModel"`
	CPUBefore                    fins.CPUUnitData   `json:"cpuBefore"`
	CPUAfter                     fins.CPUUnitData   `json:"cpuAfter"`
	StatusBefore                 fins.Status        `json:"statusBefore"`
	StatusAfter                  fins.Status        `json:"statusAfter"`
	Programs                     [2]ProgramArtifact `json:"programs"`
	Checks                       []Check            `json:"checks"`
	ReadPairConsistent           bool               `json:"readPairConsistent"`
	RetainedEvidenceOnly         bool               `json:"retainedEvidenceOnly"`
	IndependentReadsVerified     bool               `json:"independentReadsVerified"`
	ObservationFreshnessVerified bool               `json:"observationFreshnessVerified"`
	FullControllerBackup         bool               `json:"fullControllerBackup"`
	HardwareQualified            bool               `json:"hardwareQualified"`
	RestoreQualified             bool               `json:"restoreQualified"`
	ReadyForWrite                bool               `json:"readyForWrite"`
	WriteQualified               bool               `json:"writeQualified"`
	NetworkAccess                bool               `json:"networkAccess"`
}

func compilerModelForCPU(model string) string {
	switch model {
	case "CJ1M_CPU21":
		return codec.CJ1MCPU21
	case "CJ1H_CPU65H":
		return codec.CJ1HCPU65
	}
	return ""
}
func snapshotSummaryMatches(s Snapshot, c fins.CPUUnitData, t fins.Status) bool {
	return s.CPU.Model == c.Model && s.CPU.Version == c.InternalSystemVersion && s.Status.Status == t.Status && s.Status.Mode == t.Mode && s.Status.Fatal == t.Fatal && s.Status.NonFatal == t.NonFatal
}

// ReviewProgramBackup re-decodes raw CPU/status, checks both snapshots, and
// compares whole N1 files including service fields. RUN is valid for this
// file-consistency review; it does not become PROGRAM-mode write approval.
func ReviewProgramBackup(model string, before, after Snapshot, first, second []byte) (*ProgramBackupReview, error) {
	if model != codec.CJ1MCPU21 && model != codec.CJ1HCPU65 {
		return nil, errors.New("unsupported backup review model")
	}
	if len(first) > um.MaxSize || len(second) > um.MaxSize {
		return nil, errors.New("backup review program size limit")
	}
	cb, err := fins.DecodeCPUUnitData(before.CPU.Raw)
	if err != nil {
		return nil, err
	}
	ca, err := fins.DecodeCPUUnitData(after.CPU.Raw)
	if err != nil {
		return nil, err
	}
	sb, err := fins.DecodeStatusData(before.Status.Raw)
	if err != nil {
		return nil, err
	}
	sa, err := fins.DecodeStatusData(after.Status.Raw)
	if err != nil {
		return nil, err
	}
	report := &ProgramBackupReview{Format: "cj1-retained-program-backup-review", DeclaredModel: model, CPUBefore: cb, CPUAfter: ca, StatusBefore: sb, StatusAfter: sa, Programs: [2]ProgramArtifact{inspectArtifact(first), inspectArtifact(second)}, ReadPairConsistent: true, RetainedEvidenceOnly: true}
	check := func(id string, ok bool, detail string) {
		state := "passed"
		if !ok {
			state = "blocked"
			report.ReadPairConsistent = false
		}
		report.Checks = append(report.Checks, Check{ID: id, State: state, Detail: detail})
	}
	check("snapshot-summaries", snapshotSummaryMatches(before, cb, sb) && snapshotSummaryMatches(after, ca, sa), "both editable summaries must agree with raw data")
	check("declared-target-model", compilerModelForCPU(cb.Model) == model && compilerModelForCPU(ca.Model) == model, "exact observed CPU aliases only; no model inference from N1")
	check("identity-raw-stable", bytes.Equal(before.CPU.Raw, after.CPU.Raw), "whole identity responses must match; this does not prove observation time or source")
	check("status-raw-stable", bytes.Equal(before.Status.Raw, after.Status.Raw), "whole status responses must match; RUN/MONITOR/PROGRAM are accepted for backup review without changing mode")
	check("cpu-errors", sb.Fatal == 0 && sb.NonFatal == 0 && sa.Fatal == 0 && sa.NonFatal == 0, "nonzero fatal/nonfatal fields require review; nothing is cleared")
	check("program-pair", report.Programs[0].ParseableN1 && report.Programs[1].ParseableN1 && bytes.Equal(first, second), "both files must parse as supported N1 and match every byte; equality alone proves neither independent reads nor complete controller backup")
	check("reported-capacity", cb.ReportedProgramAreaBytes != 0 && ca.ReportedProgramAreaBytes != 0 && uint64(len(first)) <= uint64(cb.ReportedProgramAreaBytes) && uint64(len(second)) <= uint64(ca.ReportedProgramAreaBytes), "arithmetic reported-area upper bound only; no deployment qualification")
	return report, nil
}
