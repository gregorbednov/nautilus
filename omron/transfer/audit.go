package transfer

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/joyautomation/nautilus/omron/um"
)

var ErrProgramWriteAudit = errors.New("invalid offline program-write audit")

type ProgramAuditState string

const (
	ProgramAuditInvalid              ProgramAuditState = "invalid"
	ProgramAuditReady                ProgramAuditState = "ready"
	ProgramAuditAwaitingACK          ProgramAuditState = "awaiting-ack"
	ProgramAuditAwaitingReadback     ProgramAuditState = "awaiting-readback"
	ProgramAuditSourceBytesMatched   ProgramAuditState = "source-bytes-matched"
	ProgramAuditResidentBytesMatched ProgramAuditState = "resident-bytes-matched"
	ProgramAuditStopped              ProgramAuditState = "stopped"
)

// ProgramReadbackComparison compares retained bytes only. Even an exact match
// does not authenticate their device, time, route, source project or execution.
type ProgramReadbackComparison struct {
	SHA256                    string  `json:"sha256"`
	ByteLength                int     `json:"byteLength"`
	SameLength                bool    `json:"sameLength"`
	Exact                     bool    `json:"exact"`
	BodyMatches               bool    `json:"bodyMatches"`
	ServiceHeaderComparable   bool    `json:"serviceHeaderComparable"`
	ChangedServiceWords       [6]bool `json:"changedServiceWords"`
	OnlyServiceWords014Differ bool    `json:"onlyServiceWords014Differ"`
	FirstDifferenceByte       int     `json:"firstDifferenceByte"`
}

type ProgramWriteAuditSnapshot struct {
	Format                  string                     `json:"format"`
	State                   ProgramAuditState          `json:"state"`
	ImageSHA256             string                     `json:"imageSHA256"`
	ResidentReferenceSHA256 string                     `json:"residentReferenceSHA256,omitempty"`
	ImageBytes              int                        `json:"imageBytes"`
	PlannedBlocks           int                        `json:"plannedBlocks"`
	AttemptedBlocks         int                        `json:"attemptedBlocks"`
	AcknowledgedBlocks      int                        `json:"acknowledgedBlocks"`
	AcknowledgedBytes       int                        `json:"acknowledgedBytes"`
	NextOffsetBytes         uint32                     `json:"nextOffsetBytes"`
	AllBlocksAcknowledged   bool                       `json:"allBlocksAcknowledged"`
	UncertainWrite          bool                       `json:"uncertainWrite"`
	ReconciliationRequired  bool                       `json:"reconciliationRequired"`
	StopReason              string                     `json:"stopReason,omitempty"`
	Readback                *ProgramReadbackComparison `json:"readback,omitempty"`
	NetworkAccess           bool                       `json:"networkAccess"`
	RouteSIDVerified        bool                       `json:"routeSIDVerified"`
	HardwareQualified       bool                       `json:"hardwareQualified"`
	WriteQualified          bool                       `json:"writeQualified"`
	RollbackPerformed       bool                       `json:"rollbackPerformed"`
}

// ProgramWriteAudit consumes serialized, retained evidence. It never sends,
// retries, resumes, acquires access, changes mode or performs rollback. Its
// constructor is not preflight approval; route/SID correlation is external.
// A stopped audit cannot resume. Subsequent events after a source-byte match
// invalidate completion; readback must be the last event. Not goroutine-safe.
type ProgramWriteAudit struct {
	expectedReadback                           []byte
	residentReferenceSHA256                    string
	plan                                       *ProgramPlan
	state                                      ProgramAuditState
	attempted, acknowledged, acknowledgedBytes int
	pending                                    []byte
	stopReason                                 string
	readback                                   *ProgramReadbackComparison
}

func NewProgramWriteAudit(image []byte, blockBytes int) (*ProgramWriteAudit, error) {
	plan, err := PlanProgramWrite(image, blockBytes)
	if err != nil {
		return nil, err
	}
	return &ProgramWriteAudit{expectedReadback: bytes.Clone(image), plan: plan, state: ProgramAuditReady}, nil
}

// NewCJ1MNew4Audit pins both artifacts of the verified CPU21 new4 bench run.
// The plan always uses the wire image; only readback comparisons use the UM.
func NewCJ1MNew4Audit(image, resident []byte, blockBytes int) (*ProgramWriteAudit, error) {
	const wireHash = "6853547bfd8ac930d1cf359f130a817e9d6fa345bee3d65f3424faa18066582d"
	const residentHash = "971f2292da905172de8b09d10c1dbb845de1fb37018bb01cc9d5f592b6fe6ef8"
	return newPinnedAudit(image, resident, blockBytes, wireHash, residentHash)
}

// NewCJ1HNew4Audit pins the CPU65H program sent by CX-Programmer and the
// independently read resident image for the same transfer.
func NewCJ1HNew4Audit(image, resident []byte, blockBytes int) (*ProgramWriteAudit, error) {
	return newPinnedAudit(image, resident, blockBytes,
		"97495506fc1e61bf1a2b19c38890d1659ff13ea21caca1740d3520589e164ef4",
		"493b7cb9f806fcd25338d3e6c5c7eb6e99f5e6a7e9fc658306b78ae57dc01ce5")
}

// NewCJ1MProbeAudit uses the two-section Probe captured from CX on CPU21.
func NewCJ1MProbeAudit(image, resident []byte, blockBytes int) (*ProgramWriteAudit, error) {
	return newPinnedAudit(image, resident, blockBytes,
		"07ea1c03b4f065d2fcc29433cf21ce05aa3ff45bed03f7a69eebfa7ddd010257",
		"3cae408d74e8c86a301ad2a6f08a8eb133e6cb617e7a5682d88bf760f7f1f524")
}

func newPinnedAudit(image, resident []byte, blockBytes int, wireHash, residentHash string) (*ProgramWriteAudit, error) {
	if fmt.Sprintf("%x", sha256.Sum256(image)) != wireHash || fmt.Sprintf("%x", sha256.Sum256(resident)) != residentHash {
		return nil, fmt.Errorf("%w: unrecognized program reference pair", ErrProgramWriteAudit)
	}
	a, err := NewProgramWriteAudit(image, blockBytes)
	if err != nil {
		return nil, err
	}
	a.expectedReadback = bytes.Clone(resident)
	a.residentReferenceSHA256 = residentHash
	return a, nil
}

func (a *ProgramWriteAudit) usable() bool {
	return a != nil && a.plan != nil && a.state != ProgramAuditStopped
}

func (a *ProgramWriteAudit) stop(reason string, cause error) error {
	a.state = ProgramAuditStopped
	a.stopReason = reason
	a.pending = nil
	if cause != nil {
		return fmt.Errorf("%w: %s: %w", ErrProgramWriteAudit, reason, cause)
	}
	return fmt.Errorf("%w: %s", ErrProgramWriteAudit, reason)
}

// ExpectedBlock returns a value, without recording any transmission attempt.
func (a *ProgramWriteAudit) ExpectedBlock() (ProgramBlock, error) {
	if !a.usable() || a.state != ProgramAuditReady {
		return ProgramBlock{}, fmt.Errorf("%w: no next block", ErrProgramWriteAudit)
	}
	return a.plan.Blocks[a.acknowledged], nil
}

// RecordAttempt records a claimed attempt from a transcript, not a network
// action. A deviating or repeated request stops the audit with no new progress.
func (a *ProgramWriteAudit) RecordAttempt(payload []byte) error {
	if !a.usable() {
		return ErrProgramWriteAudit
	}
	a.attempted++
	if a.state != ProgramAuditReady {
		return a.stop("unexpected-attempt", nil)
	}
	expected, err := hex.DecodeString(a.plan.Blocks[a.acknowledged].PayloadHex)
	if err != nil {
		return a.stop("invalid-internal-plan", err)
	}
	if !bytes.Equal(payload, expected) {
		return a.stop("request-mismatch", nil)
	}
	a.pending = bytes.Clone(payload)
	a.state = ProgramAuditAwaitingACK
	return nil
}

// RecordResponse requires one exact ACK for the outstanding retained request.
// A PLC error is conservatively uncertain: it cannot prove an unchanged UM.
func (a *ProgramWriteAudit) RecordResponse(body []byte) (ProgramWriteACK, error) {
	if !a.usable() {
		return ProgramWriteACK{}, ErrProgramWriteAudit
	}
	if a.state != ProgramAuditAwaitingACK {
		return ProgramWriteACK{}, a.stop("unexpected-response", nil)
	}
	ack, err := ValidateProgramWriteResponse(a.pending, body)
	if err != nil {
		return ProgramWriteACK{}, a.stop("invalid-response", err)
	}
	a.acknowledged++
	a.acknowledgedBytes += ack.DataBytes
	a.pending = nil
	if a.acknowledged == len(a.plan.Blocks) {
		a.state = ProgramAuditAwaitingReadback
	} else {
		a.state = ProgramAuditReady
	}
	return ack, nil
}

// RecordFailure records a transport/readback failure or a missing response.
// EOF in a partial transcript alone is not proof of such a transport failure.
func (a *ProgramWriteAudit) RecordFailure() error {
	if !a.usable() {
		return ErrProgramWriteAudit
	}
	return a.stop("transport-failure", nil)
}

// InspectReadback compares retained bytes in any initialized state, including
// after a failure. It does not change progress, clear uncertainty or authorize
// resuming a stopped audit. The caller supplies and authenticates any live read.
func (a *ProgramWriteAudit) InspectReadback(data []byte) (ProgramReadbackComparison, error) {
	if a == nil || a.plan == nil {
		return ProgramReadbackComparison{}, ErrProgramWriteAudit
	}
	if len(data) > um.MaxSize {
		return ProgramReadbackComparison{}, fmt.Errorf("%w: readback-size-limit", ErrProgramWriteAudit)
	}
	return CompareProgramReadback(a.expectedReadback, data), nil
}

func (a *ProgramWriteAudit) RecordReadback(data []byte) (ProgramReadbackComparison, error) {
	if !a.usable() {
		return ProgramReadbackComparison{}, ErrProgramWriteAudit
	}
	if a.state != ProgramAuditAwaitingReadback {
		return ProgramReadbackComparison{}, a.stop("unexpected-readback", nil)
	}
	if len(data) > um.MaxSize {
		return ProgramReadbackComparison{}, a.stop("readback-size-limit", nil)
	}
	comparison := CompareProgramReadback(a.expectedReadback, data)
	a.readback = &comparison
	if !comparison.Exact {
		return comparison, a.stop("readback-mismatch", nil)
	}
	a.state = ProgramAuditSourceBytesMatched
	if a.residentReferenceSHA256 != "" {
		a.state = ProgramAuditResidentBytesMatched
	}
	return comparison, nil
}

// CompareProgramReadback compares the supplied bytes, including the service
// header. Callers validate the expected N1 and bound the input files.
func CompareProgramReadback(expected, observed []byte) ProgramReadbackComparison {
	comparison := ProgramReadbackComparison{
		SHA256: fmt.Sprintf("%x", sha256.Sum256(observed)), ByteLength: len(observed),
		SameLength: len(expected) == len(observed), FirstDifferenceByte: -1,
	}
	n := min(len(expected), len(observed))
	for i := 0; i < n; i++ {
		if expected[i] != observed[i] {
			comparison.FirstDifferenceByte = i
			break
		}
	}
	if comparison.FirstDifferenceByte < 0 && !comparison.SameLength {
		comparison.FirstDifferenceByte = n
	}
	comparison.Exact = comparison.SameLength && comparison.FirstDifferenceByte < 0
	comparison.ServiceHeaderComparable = n >= 24
	if comparison.ServiceHeaderComparable {
		for i := range comparison.ChangedServiceWords {
			comparison.ChangedServiceWords[i] = !bytes.Equal(expected[i*4:i*4+4], observed[i*4:i*4+4])
		}
	}
	comparison.BodyMatches = comparison.SameLength && n >= 24 && bytes.Equal(expected[24:], observed[24:])
	comparison.OnlyServiceWords014Differ = comparison.BodyMatches && !comparison.Exact && !comparison.ChangedServiceWords[2] && !comparison.ChangedServiceWords[3] && !comparison.ChangedServiceWords[5]
	return comparison
}

func (a *ProgramWriteAudit) Snapshot() ProgramWriteAuditSnapshot {
	snapshot := ProgramWriteAuditSnapshot{Format: "cj1-offline-program-write-audit", State: ProgramAuditInvalid}
	if a == nil || a.plan == nil {
		return snapshot
	}
	snapshot.State = a.state
	snapshot.ImageSHA256 = a.plan.ImageSHA256
	snapshot.ImageBytes = a.plan.ImageBytes
	snapshot.ResidentReferenceSHA256 = a.residentReferenceSHA256
	snapshot.PlannedBlocks = len(a.plan.Blocks)
	snapshot.AttemptedBlocks = a.attempted
	snapshot.AcknowledgedBlocks = a.acknowledged
	snapshot.AcknowledgedBytes = a.acknowledgedBytes
	snapshot.NextOffsetBytes = uint32(a.acknowledgedBytes)
	snapshot.AllBlocksAcknowledged = a.acknowledged == len(a.plan.Blocks)
	snapshot.UncertainWrite = a.attempted > 0 && a.state != ProgramAuditSourceBytesMatched && a.state != ProgramAuditResidentBytesMatched
	snapshot.ReconciliationRequired = a.attempted > 0 && a.state == ProgramAuditStopped
	snapshot.StopReason = a.stopReason
	if a.readback != nil {
		copy := *a.readback
		snapshot.Readback = &copy
	}
	return snapshot
}
