package commission

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"

	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/fins"
	"github.com/joyautomation/nautilus/omron/transfer"
	"github.com/joyautomation/nautilus/omron/um"
)

// ProgramSession is an experimental program-only transfer mechanism, not live
// commissioning approval. The caller must establish stand isolation, exclusive
// controller access externally. NewControlledProgramSession adds explicit
// journaled mode/access steps; the legacy constructor requires PROGRAM externally.
// Methods serialize; Close waits for the current operation. Supply bounded
// contexts to operations (and cancel them to interrupt network I/O).
type ProgramSession struct {
	mu                          sync.Mutex
	client                      *fins.Client
	cfg                         ProgramSessionConfig
	binding                     fins.ConnectionBinding
	journal                     ProgramSessionJournal
	state                       string
	reason                      string
	original                    []byte
	candidateAudit, returnAudit *transfer.ProgramWriteAudit
	returnTried                 bool
	controlled                  bool
	accessState                 string
	programEntered, finished    bool
	expectedMode                fins.CPUMode
	executionRequested          bool
	observedExecutionMode       *fins.CPUMode
	archive, originalCatalog    []byte
	originalArchive             []byte
	archiveTransferred          bool
	originalArchiveRestored     bool
}

type ProgramSessionConfig struct {
	Model           string
	ExpectedPeer    string
	ExpectedCPUData []byte
	Candidate       []byte
	// CJ1MNew4Readback opts into the exact retained UM for the verified new4 wire image.
	CJ1MNew4Readback []byte
	// CJ1HNew4Readback pins the CPU65H Programs-only FB transfer.
	CJ1HNew4Readback []byte
	// CJ1MProbeReadback selects the retained two-section CPU21 Probe reference.
	CJ1MProbeReadback     []byte
	Build                 *compiler.VerifiedImageBuild
	BlockBytes            int
	ExpectedOriginal      []byte
	RequireInitialProgram bool
	// PreserveFBSource backs up and verifies the CPU file catalog even when
	// the candidate has no FB source archive.
	PreserveFBSource bool
}

// Journal implementations must retain copies and synchronously persist before
// returning success. Reports/files cannot be imported to resume a session.
type ProgramSessionJournal interface {
	SavePrepared(PreparedProgramBundle) error
	Append(ProgramSessionEvent) error
}

type PreparedProgramBundle struct {
	ResidentReferenceSHA256 string                          `json:"residentReferenceSHA256,omitempty"`
	Binding                 fins.ConnectionBinding          `json:"binding"`
	Before                  Snapshot                        `json:"before"`
	After                   Snapshot                        `json:"after"`
	Build                   compiler.ImageBuildVerification `json:"build"`
	Candidate               []byte                          `json:"-"`
	OriginalFirst           []byte                          `json:"-"`
	OriginalSecond          []byte                          `json:"-"`
	SourceArchive           []byte                          `json:"-"`
	OriginalArchive         []byte                          `json:"-"`
	OriginalCatalog         []byte                          `json:"-"`
}

type ProgramSessionEvent struct {
	AccessOwner   *fins.Address                       `json:"accessOwner,omitempty"`
	EndCode       *uint16                             `json:"endCode,omitempty"`
	Command       string                              `json:"command,omitempty"`
	RequestedMode *fins.CPUMode                       `json:"requestedMode,omitempty"`
	Binding       *fins.ConnectionBinding             `json:"binding,omitempty"`
	ReadbackData  []byte                              `json:"readbackData,omitempty"`
	Observation   *Snapshot                           `json:"observation,omitempty"`
	Phase         string                              `json:"phase"`
	Kind          string                              `json:"kind"`
	PayloadHex    string                              `json:"payloadHex,omitempty"`
	ACK           *transfer.ProgramWriteACK           `json:"validatedACK,omitempty"`
	Readback      *transfer.ProgramReadbackComparison `json:"readback,omitempty"`
	Reason        string                              `json:"reason,omitempty"`
}

type ProgramSessionSnapshot struct {
	Controlled              bool   `json:"controlled"`
	AccessState             string `json:"accessState"`
	ProgramEntered          bool   `json:"programEntered"`
	Finished                bool   `json:"finished"`
	ExecutionRequested      bool   `json:"executionRequested"`
	ArchiveTransferred      bool   `json:"archiveTransferred,omitempty"`
	OriginalArchiveRestored bool   `json:"originalArchiveRestored,omitempty"`
	// ObservedExecutionMode records a successful post-request execution check in this leg.
	ObservedExecutionMode  *fins.CPUMode                      `json:"observedExecutionMode,omitempty"`
	State                  string                             `json:"state"`
	StopReason             string                             `json:"stopReason,omitempty"`
	Binding                fins.ConnectionBinding             `json:"binding"`
	Candidate              transfer.ProgramWriteAuditSnapshot `json:"candidate"`
	OriginalReturn         transfer.ProgramWriteAuditSnapshot `json:"originalReturn"`
	Original               ProgramArtifact                    `json:"original"`
	ReturnAttempted        bool                               `json:"returnAttempted"`
	ProgramBytesReconciled bool                               `json:"programBytesReconciled"`
	HardwareQualified      bool                               `json:"hardwareQualified"`
	WriteQualified         bool                               `json:"writeQualified"`
	RestoreQualified       bool                               `json:"restoreQualified"`
	FullControllerBackup   bool                               `json:"fullControllerBackup"`
}

// NewProgramSession retires the supplied client on success. The old handle can
// neither exchange commands nor close the connection; SID continuity is kept.
// Only built-in direct TCP is supported here; no reconnect or route fallback.
func NewProgramSession(c *fins.Client, cfg ProgramSessionConfig, journal ProgramSessionJournal) (*ProgramSession, error) {
	if c == nil || journal == nil {
		return nil, errors.New("client and synchronous journal required")
	}
	if !cfg.Build.Matches(cfg.Model, cfg.Candidate) {
		return nil, errors.New("candidate requires exact in-process build proof")
	}
	cpu, err := fins.DecodeCPUUnitData(cfg.ExpectedCPUData)
	if err != nil {
		return nil, err
	}
	if compilerModelForCPU(cpu.Model) != cfg.Model || cpu.ReportedProgramAreaBytes == 0 || uint64(len(cfg.Candidate)) > uint64(cpu.ReportedProgramAreaBytes) {
		return nil, errors.New("expected CPU/model/capacity mismatch")
	}
	binding := c.Binding()
	if binding.Route.Destination.Network != 0 || binding.Route.Destination.Unit != 0 || binding.Route.Source.Network != 0 || binding.Route.Source.Unit != 0 || binding.Transport != "tcp" || binding.Peer != cfg.ExpectedPeer || cfg.ExpectedPeer == "" {
		return nil, errors.New("explicit direct TCP peer required")
	}
	if cfg.BlockBytes > c.ProgramWriteBlockLimit() {
		return nil, errors.New("block size exceeds connection limit")
	}
	archive := cfg.Build.SourceArchive()
	if (cfg.PreserveFBSource || len(archive) != 0) && cpu.Model != "CJ1M_CPU21" && cfg.CJ1HNew4Readback == nil {
		return nil, errors.New("FB source transfer requires CJ1M CPU21 or a pinned CJ1H CPU65H reference")
	}
	if len(archive) != 0 {
		if _, err := fins.FBSourceWritePayload(archive); err != nil {
			return nil, err
		}
	}
	var audit *transfer.ProgramWriteAudit
	if cfg.CJ1HNew4Readback != nil {
		if cpu.Model != "CJ1H_CPU65H" || len(archive) == 0 || cfg.CJ1MNew4Readback != nil || cfg.CJ1MProbeReadback != nil {
			return nil, errors.New("CJ1H new4 requires CPU65H, an FB archive and a single reference")
		}
		audit, err = transfer.NewCJ1HNew4Audit(cfg.Candidate, cfg.CJ1HNew4Readback, cfg.BlockBytes)
		cfg.CJ1HNew4Readback = nil // The audit owns its private copy.
	} else if cfg.CJ1MProbeReadback != nil {
		if cpu.Model != "CJ1M_CPU21" || cfg.CJ1MNew4Readback != nil {
			return nil, errors.New("Probe requires CJ1M CPU21 and a single reference")
		}
		audit, err = transfer.NewCJ1MProbeAudit(cfg.Candidate, cfg.CJ1MProbeReadback, cfg.BlockBytes)
		cfg.CJ1MProbeReadback = nil
	} else if cfg.CJ1MNew4Readback != nil {
		if cpu.Model != "CJ1M_CPU21" {
			return nil, errors.New("new4 resident reference requires CJ1M CPU21")
		}
		audit, err = transfer.NewCJ1MNew4Audit(cfg.Candidate, cfg.CJ1MNew4Readback, cfg.BlockBytes)
		cfg.CJ1MNew4Readback = nil // The audit owns its private copy.
	} else {
		audit, err = transfer.NewProgramWriteAudit(cfg.Candidate, cfg.BlockBytes)
	}
	if err != nil {
		return nil, err
	}
	owned, err := c.TakeExclusive()
	if err != nil {
		return nil, err
	}
	cfg.Candidate = bytes.Clone(cfg.Candidate)
	cfg.ExpectedCPUData = bytes.Clone(cfg.ExpectedCPUData)
	cfg.ExpectedOriginal = bytes.Clone(cfg.ExpectedOriginal)
	return &ProgramSession{client: owned, cfg: cfg, binding: binding, journal: journal, state: "new", candidateAudit: audit, archive: archive}, nil
}

func (s *ProgramSession) observe(ctx context.Context) (Snapshot, error) {
	return s.observeMode(ctx, true)
}

func (s *ProgramSession) observeMode(ctx context.Context, requireProgram bool) (Snapshot, error) {
	var o Snapshot
	var err error
	o.CPU, err = s.client.Identity(ctx)
	if err != nil {
		return o, err
	}
	o.Status, err = s.client.Status(ctx)
	if err != nil {
		return o, err
	}
	if !bytes.Equal(o.CPU.Raw, s.cfg.ExpectedCPUData) || (requireProgram && (o.Status.Mode != 0 || o.Status.Executing())) || o.Status.Fatal != 0 || o.Status.NonFatal != 0 {
		return o, errors.New("target identity, PROGRAM mode or CPU error guard failed")
	}
	return o, nil
}

func (s *ProgramSession) stop(phase string, cause error, audit *transfer.ProgramWriteAudit) error {
	s.state = "stopped"
	s.reason = cause.Error()
	if s.controlled && s.accessState != "" && s.accessState != "released" {
		s.accessState = "uncertain"
	}
	if audit != nil {
		_ = audit.RecordFailure()
	}
	_ = s.client.Close()
	// Failure to persist a failure is itself reported. Never resume on journal loss.
	event := ProgramSessionEvent{Phase: phase, Kind: "stopped", Reason: s.reason}
	var busy *fins.AccessRightBusyError
	if errors.As(cause, &busy) {
		owner := busy.Owner
		event.AccessOwner = &owner
	}
	var end *fins.EndError
	if errors.As(cause, &end) {
		code := end.Code
		event.EndCode = &code
		event.Command = fmt.Sprintf("%04X", end.Command)
	}
	journalErr := s.journal.Append(event)
	if journalErr != nil {
		s.reason += "; stop journal: " + journalErr.Error()
	}
	return fmt.Errorf("program session stopped: %w", errors.Join(cause, journalErr))
}

// Prepare acquires two independent live 0306 reads, checks the entire raw CPU
// and status before/after and persists the pair before enabling WriteCandidate.
// This backs up only program bytes, not settings, I/O tables or retained data.
func (s *ProgramSession) Prepare(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != "new" {
		return errors.New("prepare requires a new session")
	}
	requireProgram := !s.controlled || s.cfg.RequireInitialProgram
	before, err := s.observeMode(ctx, requireProgram)
	if err != nil {
		return s.stop("prepare", err, nil)
	}
	first, err := s.client.ReadProgram(ctx, um.MaxSize)
	if err != nil {
		return s.stop("prepare", err, nil)
	}
	second, err := s.client.ReadProgram(ctx, um.MaxSize)
	if err != nil {
		return s.stop("prepare", err, nil)
	}
	if s.cfg.PreserveFBSource || len(s.archive) != 0 {
		if err := s.backupOriginalFiles(ctx); err != nil {
			return s.stop("prepare", fmt.Errorf("back up CPU file catalog: %w", err), nil)
		}
	}
	after, err := s.observeMode(ctx, requireProgram)
	if err != nil {
		return s.stop("prepare", err, nil)
	}
	review, err := ReviewProgramBackup(s.cfg.Model, before, after, first, second)
	if err != nil {
		return s.stop("prepare", err, nil)
	}
	if !review.ReadPairConsistent {
		return s.stop("prepare", errors.New("fresh program backup pair inconsistent"), nil)
	}
	if len(s.cfg.ExpectedOriginal) != 0 && !bytes.Equal(first, s.cfg.ExpectedOriginal) {
		return s.stop("prepare", errors.New("fresh program differs from the expected original"), nil)
	}
	originalAudit, err := transfer.NewProgramWriteAudit(first, s.cfg.BlockBytes)
	if err != nil {
		return s.stop("prepare", err, nil)
	}
	// Journal receives distinct copies, never the private original/candidate.
	original := bytes.Clone(first)
	bundle := PreparedProgramBundle{Binding: s.binding, Before: before, After: after, Build: s.cfg.Build.Report(), Candidate: bytes.Clone(s.cfg.Candidate), OriginalFirst: bytes.Clone(first), OriginalSecond: bytes.Clone(second)}
	bundle.ResidentReferenceSHA256 = s.candidateAudit.Snapshot().ResidentReferenceSHA256
	bundle.SourceArchive = bytes.Clone(s.archive)
	bundle.OriginalArchive = bytes.Clone(s.originalArchive)
	bundle.OriginalCatalog = bytes.Clone(s.originalCatalog)
	if err = s.journal.SavePrepared(bundle); err != nil {
		return s.stop("prepare", err, nil)
	}
	if err = s.journal.Append(ProgramSessionEvent{Phase: "prepare", Kind: "prepared"}); err != nil {
		return s.stop("prepare", err, nil)
	}
	s.original, s.returnAudit, s.state = original, originalAudit, "prepared"
	return nil
}

// WriteCandidate is one-shot. It rechecks the original before the first write,
// persists each attempt before 0307 and requires every raw byte on readback.
// Any failure stops and closes the connection; there is no automatic rollback.
func (s *ProgramSession) WriteCandidate(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if (s.cfg.PreserveFBSource || len(s.archive) != 0) && !s.controlled {
		return errors.New("SRCPRGRM.FBL transfer requires a controlled session")
	}
	return s.writePrepared(ctx, "candidate", "candidate-matched", s.candidateAudit)
}

// RewriteOriginal transfers only the two matching backups captured by Prepare.
// The caller pins the expected bytes before entering this one-shot bench trial.
func (s *ProgramSession) RewriteOriginal(ctx context.Context, expected []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.controlled || len(expected) == 0 || !bytes.Equal(s.original, expected) {
		return errors.New("original rewrite requires controlled session and exact pinned backup")
	}
	return s.writePrepared(ctx, "original-rewrite", "original-matched", s.returnAudit)
}

func (s *ProgramSession) writePrepared(ctx context.Context, phase, matched string, audit *transfer.ProgramWriteAudit) error {
	if s.state != "prepared" || (s.controlled && (!s.programEntered || s.finished || s.executionRequested || s.accessState != "acknowledged")) {
		return errors.New("write requires prepared session and controlled PROGRAM/access steps")
	}
	if _, err := s.observe(ctx); err != nil {
		return s.stop(phase, err, audit)
	}
	current, err := s.client.ReadProgram(ctx, um.MaxSize)
	if err != nil {
		return s.stop(phase, err, audit)
	}
	if !bytes.Equal(current, s.original) {
		return s.stop(phase, errors.New("program changed since backup"), audit)
	}
	if err = s.checkOriginalFiles(ctx, s.client); err != nil {
		return s.stop(phase, err, audit)
	}
	guard, err := s.observe(ctx)
	if err != nil {
		return s.stop(phase, err, audit)
	}
	if err = s.journal.Append(ProgramSessionEvent{Phase: phase, Kind: "guard", Observation: &guard}); err != nil {
		return s.stop(phase, err, audit)
	}
	s.state = "writing-" + phase
	if err = s.send(ctx, phase, audit); err != nil {
		return err
	}
	if len(s.originalCatalog) != 0 {
		if phase == "candidate" {
			err = s.sendCandidateFiles(ctx)
		} else {
			err = s.restoreOriginalFiles(ctx)
		}
		if err != nil {
			if s.state == "stopped" {
				return err
			}
			return s.stop(phase+"-archive", err, nil)
		}
	}
	s.state = matched
	if !s.controlled {
		_ = s.client.Close()
	}
	return nil
}

func (s *ProgramSession) send(ctx context.Context, phase string, audit *transfer.ProgramWriteAudit) error {
	for audit.Snapshot().State == transfer.ProgramAuditReady {
		block, err := audit.ExpectedBlock()
		if err != nil {
			return s.stop(phase, err, audit)
		}
		payload, err := hex.DecodeString(block.PayloadHex)
		if err != nil {
			return s.stop(phase, err, audit)
		}
		if err = s.journal.Append(ProgramSessionEvent{Phase: phase, Kind: "write-attempt", PayloadHex: block.PayloadHex}); err != nil {
			return s.stop(phase, err, audit)
		}
		if err = audit.RecordAttempt(payload); err != nil {
			return s.stop(phase, err, audit)
		}
		ack, err := s.client.WriteProgramBlock(ctx, payload)
		if err != nil {
			return s.stop(phase, err, audit)
		}
		// Typed ACK was already route/SID/body validated by fins.Client. This is a
		// derived audit body, not a retained raw wire response or packet capture.
		body := make([]byte, 12)
		binary.BigEndian.PutUint16(body, 0x0307)
		binary.BigEndian.PutUint16(body[4:], 0xffff)
		binary.BigEndian.PutUint32(body[6:], ack.OffsetBytes)
		binary.BigEndian.PutUint16(body[10:], uint16(ack.DataBytes))
		if _, err = audit.RecordResponse(body); err != nil {
			return s.stop(phase, err, audit)
		}
		if err = s.journal.Append(ProgramSessionEvent{Phase: phase, Kind: "ack", ACK: &ack}); err != nil {
			return s.stop(phase, err, audit)
		}
	}
	readback, err := s.client.ReadProgram(ctx, um.MaxSize)
	if err != nil {
		return s.stop(phase, err, audit)
	}
	observedComparison, err := audit.InspectReadback(readback)
	if err != nil {
		return s.stop(phase, err, audit)
	}
	if err = s.journal.Append(ProgramSessionEvent{Phase: phase, Kind: "readback-observed", ReadbackData: bytes.Clone(readback), Readback: &observedComparison}); err != nil {
		return s.stop(phase, err, audit)
	}
	guard, err := s.observe(ctx)
	if err != nil {
		return s.stop(phase, err, audit)
	}
	comparison, compareErr := audit.RecordReadback(readback)
	if err = s.journal.Append(ProgramSessionEvent{Phase: phase, Kind: "readback", Readback: &comparison, Observation: &guard}); err != nil {
		return s.stop(phase, err, audit)
	}
	if compareErr != nil {
		return s.stop(phase, compareErr, audit)
	}
	return nil
}

// ReturnOriginal is an explicit, separate, one-shot recovery operation on a
// newly supplied TCP client. It can only send the private pre-attempt original.
// Failed candidate audits remain stopped. Exact original readback reconciles
// program bytes only; it is not a full-controller restoration qualification.
func (s *ProgramSession) ReturnOriginal(ctx context.Context, fresh *fins.Client) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if fresh == nil || s.returnTried || len(s.original) == 0 || s.candidateAudit.Snapshot().AttemptedBlocks == 0 || (s.state != "stopped" && s.state != "candidate-matched") {
		return errors.New("explicit original return not available")
	}
	if s.controlled && s.accessState == "acknowledged" && !s.finished {
		return errors.New("finish the current access session explicitly before original return")
	}
	b := fresh.Binding()
	if b.Route.Source.Network != 0 || b.Route.Source.Unit != 0 || b.Transport != "tcp" || b.Peer != s.binding.Peer || b.Route.Destination != s.binding.Route.Destination {
		return errors.New("original return target binding mismatch")
	}
	if s.cfg.BlockBytes > fresh.ProgramWriteBlockLimit() {
		return errors.New("original return block limit mismatch")
	}
	owned, err := fresh.TakeExclusive()
	if err != nil {
		return err
	}
	_ = s.client.Close()
	s.client = owned
	s.returnTried = true
	s.state = "returning-original"
	if s.controlled {
		s.accessState, s.programEntered, s.finished, s.executionRequested = "", false, false, false
		s.observedExecutionMode = nil
		if err = s.enterControl(ctx, "original-return"); err != nil {
			return err
		}
	}
	return s.sendOriginal(ctx, b)
}

func (s *ProgramSession) sendOriginal(ctx context.Context, binding fins.ConnectionBinding) error {
	guard, err := s.observe(ctx)
	if err != nil {
		return s.stop("original-return", err, s.returnAudit)
	}
	if err = s.journal.Append(ProgramSessionEvent{Phase: "original-return", Kind: "explicit-return", Observation: &guard, Binding: &binding}); err != nil {
		return s.stop("original-return", err, s.returnAudit)
	}
	if err = s.send(ctx, "original-return", s.returnAudit); err != nil {
		return err
	}
	if err = s.restoreOriginalFiles(ctx); err != nil {
		if s.state == "stopped" {
			return err
		}
		return s.stop("original-return", err, nil)
	}
	s.state = "original-matched"
	if !s.controlled {
		_ = s.client.Close()
	}
	return nil
}

func (s *ProgramSession) Snapshot() ProgramSessionSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return ProgramSessionSnapshot{Controlled: s.controlled, AccessState: s.accessState, ProgramEntered: s.programEntered, Finished: s.finished, ExecutionRequested: s.executionRequested, ArchiveTransferred: s.archiveTransferred, OriginalArchiveRestored: s.originalArchiveRestored, ObservedExecutionMode: copiedMode(s.observedExecutionMode), State: s.state, StopReason: s.reason, Binding: s.binding, Candidate: s.candidateAudit.Snapshot(), OriginalReturn: s.returnAudit.Snapshot(), Original: inspectArtifact(s.original), ReturnAttempted: s.returnTried, ProgramBytesReconciled: s.returnAudit.Snapshot().State == transfer.ProgramAuditSourceBytesMatched}
}

func (s *ProgramSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == "closed" {
		return nil
	}
	if s.controlled && s.accessState != "" && s.accessState != "released" {
		s.accessState = "uncertain"
	}
	s.state = "closed"
	if err := s.client.Close(); err != nil {
		return fmt.Errorf("close program session: %w", err)
	}
	return nil
}
