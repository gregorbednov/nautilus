// Package commission reviews retained commissioning evidence and provides an
// experimental program-only session API. Reports are not live write approval.
package commission

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/fins"
	"github.com/joyautomation/nautilus/omron/um"
)

type Snapshot struct {
	CPU    fins.Identity `json:"cpu"`
	Status fins.Status   `json:"status"`
}

type Check struct {
	ID     string `json:"id"`
	State  string `json:"state"`
	Detail string `json:"detail"`
}

type ProgramArtifact struct {
	SHA256      string `json:"sha256"`
	ByteLength  int    `json:"byteLength"`
	ParseableN1 bool   `json:"parseableN1"`
}

type ProgramPreflight struct {
	BuildVerification     *compiler.ImageBuildVerification `json:"buildVerification,omitempty"`
	Format                string                           `json:"format"`
	DeclaredModel         string                           `json:"declaredModel"`
	ObservedCompilerModel string                           `json:"observedCompilerModel,omitempty"`
	CPU                   fins.CPUUnitData                 `json:"cpu"`
	Status                fins.Status                      `json:"status"`
	Candidate             ProgramArtifact                  `json:"candidate"`
	ProgramBackups        [2]ProgramArtifact               `json:"programBackups"`
	BackupPairEqual       bool                             `json:"backupPairEqual"`
	CandidateEqualsBackup bool                             `json:"candidateEqualsBackup"`
	Checks                []Check                          `json:"checks"`
	Blockers              []string                         `json:"blockers"`
	RetainedEvidenceOnly  bool                             `json:"retainedEvidenceOnly"`
	NetworkAccess         bool                             `json:"networkAccess"`
	ReadyForWrite         bool                             `json:"readyForWrite"`
	HardwareQualified     bool                             `json:"hardwareQualified"`
	WriteQualified        bool                             `json:"writeQualified"`
}

func inspectArtifact(data []byte) ProgramArtifact {
	_, err := um.ParseN1(data)
	return ProgramArtifact{SHA256: fmt.Sprintf("%x", sha256.Sum256(data)), ByteLength: len(data), ParseableN1: err == nil}
}

// AssessProgramWrite compares a declared build model with retained raw CPU data
// and checks two program-only backup files. Internal version is never treated
// as unit version, and a reported byte ceiling is not N1 admission proof.
// Unknown prerequisites remain blockers; this function has no permission input.
func AssessProgramWrite(model string, image []byte, snapshot Snapshot, backup, backupCheck []byte) (*ProgramPreflight, error) {
	return AssessProgramWriteWithBuild(model, image, snapshot, backup, backupCheck, nil)
}

// AssessProgramWriteWithBuild accepts only an in-process recompilation proof.
// A passed recipe check does not remove source-settings or hardware blockers.
func AssessProgramWriteWithBuild(model string, image []byte, snapshot Snapshot, backup, backupCheck []byte, build *compiler.VerifiedImageBuild) (*ProgramPreflight, error) {
	if model != codec.CJ1MCPU21 && model != codec.CJ1HCPU65 {
		return nil, errors.New("unsupported declared compiler model")
	}
	if len(image) > um.MaxSize || len(backup) > um.MaxSize || len(backupCheck) > um.MaxSize {
		return nil, errors.New("preflight program image exceeds size limit")
	}
	candidate := inspectArtifact(image)
	if !candidate.ParseableN1 {
		return nil, errors.New("preflight candidate must be supported raw N1")
	}
	cpu, err := fins.DecodeCPUUnitData(snapshot.CPU.Raw)
	if err != nil {
		return nil, err
	}
	status, err := fins.DecodeStatusData(snapshot.Status.Raw)
	if err != nil {
		return nil, err
	}
	report := &ProgramPreflight{Format: "cj1-offline-program-preflight", DeclaredModel: model, CPU: cpu, Status: status, Candidate: candidate, ProgramBackups: [2]ProgramArtifact{inspectArtifact(backup), inspectArtifact(backupCheck)}, RetainedEvidenceOnly: true}
	add := func(id, state, detail string) {
		report.Checks = append(report.Checks, Check{id, state, detail})
		if state != "passed" {
			report.Blockers = append(report.Blockers, id)
		}
	}
	known := func(id string, passed bool, detail string) {
		state := "blocked"
		if passed {
			state = "passed"
		}
		add(id, state, detail)
	}
	consistent := snapshotSummaryMatches(snapshot, cpu, status)
	known("snapshot-summary", consistent, "summary fields must agree with decoded raw bytes; raw bytes are authoritative")
	// These two aliases are grounded in retained device responses, not fuzzy
	// family matching or a declaration embedded in the N1 file itself.
	report.ObservedCompilerModel = compilerModelForCPU(cpu.Model)
	known("declared-target-model", report.ObservedCompilerModel == model, "only the two observed exact CPU identifiers are mapped; this does not prove candidate build provenance")
	known("program-mode", status.Mode == 0, "retained snapshot must show PROGRAM; no mode change is performed or live state inferred")
	known("cpu-errors", status.Fatal == 0 && status.NonFatal == 0, "nonzero fatal/nonfatal fields require review; no error is cleared")
	if cpu.ReportedProgramAreaBytes == 0 {
		add("program-capacity-upper-bound", "unverified", "reported program size is zero; usable target capacity is not established")
	} else {
		known("program-capacity-upper-bound", uint64(len(image)) <= uint64(cpu.ReportedProgramAreaBytes), "arithmetic comparison with reported Kwords*1024*2; passing is not target-layout qualification")
	}
	report.BackupPairEqual = bytes.Equal(backup, backupCheck)
	validPair := report.ProgramBackups[0].ParseableN1 && report.ProgramBackups[1].ParseableN1 && report.BackupPairEqual
	known("program-backup-pair", validPair, "both retained program files must parse as N1 and match exactly; this proves neither fresh reads nor full-controller backup")
	report.CandidateEqualsBackup = report.ProgramBackups[0].ParseableN1 && bytes.Equal(image, backup)
	if cpu.ReportedProgramAreaBytes != 0 {
		known("backup-capacity-upper-bound", uint64(len(backup)) <= uint64(cpu.ReportedProgramAreaBytes) && uint64(len(backupCheck)) <= uint64(cpu.ReportedProgramAreaBytes), "retained backup lengths must not exceed the reported program-area byte ceiling")
	}
	if build == nil {
		add("candidate-build-provenance", "unverified", "no local source/settings recompilation was supplied")
	} else {
		matched := build.Matches(model, image)
		known("candidate-build-provenance", matched, "exact local recipe reproduction; does not authenticate historical origin or uniquely identify a CPU model")
		if matched {
			verified := build.Report()
			report.BuildVerification = &verified
		}
	}
	for _, missing := range []Check{
		{"observation-authentication-and-freshness", "unverified", "retained files do not authenticate a current device, route, SID or observation time"},
		{"unit-version-profile", "unverified", "0501 internal system version is not a qualified unit-version mapping; current compiler profiles have unknown unit version"},
		{"source-settings-agreement", "unverified", "explicit recipe settings have not been proven to agree with opaque original project/device settings"},
		{"target-layout-and-finalization", "unverified", "reported size does not qualify the complete N1 mapping, protection branch or CPU finalization"},
		{"full-backup-and-recovery", "unverified", "program-only copies omit settings, I/O tables, retained data and a qualified restoration procedure"},
		{"initial-data-and-address-audit", "unverified", "source allocation, initial values and affected target memory have not been audited here"},
		{"write-authorization", "unverified", "this offline assessment does not establish stand isolation or operator authorization"},
	} {
		add(missing.ID, missing.State, missing.Detail)
	}
	// No current path resolves the unverified prerequisites above. Positive
	// consistency checks must never be promoted into permission or qualification.
	report.ReadyForWrite = len(report.Blockers) == 0
	return report, nil
}
