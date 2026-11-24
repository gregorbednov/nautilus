package commission

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/joyautomation/nautilus/omron/fins"
	"github.com/joyautomation/nautilus/omron/um"
)

// NewControlledProgramSession opts into explicit journaled control steps. Prepare
// captures a stable program pair in the current mode before any control command.
// EnterProgram must then be called before WriteCandidate. Successful transfers
// retain the connection for explicit RequestExecution/FinishProgram calls.
// Stand isolation and exclusion of other applications remain caller prerequisites.
// This experimental API does not grant hardware or complete-backup qualification.
func NewControlledProgramSession(c *fins.Client, cfg ProgramSessionConfig, journal ProgramSessionJournal) (*ProgramSession, error) {
	s, err := NewProgramSession(c, cfg, journal)
	if err != nil {
		return nil, err
	}
	s.controlled = true
	return s, nil
}

// control persists intent before issuing one command, then its validated ACK.
// Acknowledged access is an observation, never a device-enforced lease token.
func (s *ProgramSession) control(ctx context.Context, phase, command, payload string, mode *fins.CPUMode, invoke func() error) error {
	binding := s.client.Binding()
	if err := s.journal.Append(ProgramSessionEvent{Binding: &binding, Phase: phase, Kind: "control-attempt", Command: command, PayloadHex: payload, RequestedMode: copiedMode(mode)}); err != nil {
		return s.stop(phase, err, nil)
	}
	if command == "0C01" || command == "0C03" {
		s.accessState = "uncertain"
	}
	if err := invoke(); err != nil {
		return s.stop(phase, err, nil)
	}
	if err := s.journal.Append(ProgramSessionEvent{Phase: phase, Kind: "control-ack", Command: command, RequestedMode: copiedMode(mode)}); err != nil {
		return s.stop(phase, err, nil)
	}
	switch command {
	case "0C01":
		s.accessState = "acknowledged"
	case "0C03":
		s.accessState = "released"
	}
	return nil
}

func (s *ProgramSession) controlObservation(ctx context.Context, phase string, mode *fins.CPUMode) error {
	guard, observationErr := s.observeMode(ctx, false)
	if phase == "execute-candidate-matched" && mode != nil && *mode != fins.CPUModeProgram {
		// A successful RUN ACK can precede the first instruction scan.
		deadline := time.Now().Add(2 * time.Second)
		for observationErr == nil && !guard.Status.Executing() &&
			(guard.Status.Mode == byte(*mode) || guard.Status.Mode == byte(fins.CPUModeProgram)) && time.Now().Before(deadline) {
			select {
			case <-ctx.Done():
				observationErr = ctx.Err()
			case <-time.After(50 * time.Millisecond):
				guard, observationErr = s.observeMode(ctx, false)
			}
		}
	}
	if observationErr == nil && mode != nil && guard.Status.Mode != byte(*mode) {
		observationErr = errors.New("requested CPU mode not observed")
	}
	if observationErr == nil && mode != nil && *mode == fins.CPUModeProgram && guard.Status.Executing() {
		observationErr = errors.New("CPU reports PROGRAM mode but user program is executing")
	}
	if observationErr == nil && mode != nil && *mode != fins.CPUModeProgram && !guard.Status.Executing() {
		observationErr = errors.New("CPU reports requested mode but user program is stopped")
	}
	event := ProgramSessionEvent{Phase: phase, Kind: "control-observation", Observation: &guard, RequestedMode: copiedMode(mode)}
	if observationErr != nil {
		event.Reason = observationErr.Error()
	}
	journalErr := s.journal.Append(event)
	if observationErr != nil || journalErr != nil {
		return s.stop(phase, errors.Join(observationErr, journalErr), nil)
	}
	return nil
}

func (s *ProgramSession) enterControl(ctx context.Context, phase string) error {
	if err := s.controlObservation(ctx, phase, nil); err != nil {
		return err
	}
	if err := s.control(ctx, phase, "0C01", "ffff", nil, func() error { return s.client.AcquireProgramAccess(ctx) }); err != nil {
		return err
	}
	if err := s.controlObservation(ctx, phase, nil); err != nil {
		return err
	}
	mode := fins.CPUModeProgram
	if err := s.control(ctx, phase, "0402", "ffff", &mode, func() error { return s.client.RequestCPUMode(ctx, mode) }); err != nil {
		return err
	}
	if err := s.controlObservation(ctx, phase, &mode); err != nil {
		return err
	}
	s.programEntered = true
	s.expectedMode = mode
	return nil
}

// EnterProgram is explicit one-shot acquire->STOP->PROGRAM readback after the
// backup is saved. The current UM must still match that backup before control.
// A failure never triggers release, RUN, forced acquisition or reconnection.
func (s *ProgramSession) EnterProgram(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.controlled || s.state != "prepared" || s.accessState != "" || s.finished {
		return errors.New("enter PROGRAM requires a prepared controlled session")
	}
	if _, err := s.observeMode(ctx, false); err != nil {
		return s.stop("enter-program", err, nil)
	}
	current, err := s.client.ReadProgram(ctx, um.MaxSize)
	if err != nil {
		return s.stop("enter-program", err, nil)
	}
	if !bytes.Equal(current, s.original) {
		return s.stop("enter-program", errors.New("program changed since backup"), nil)
	}
	return s.enterControl(ctx, "enter-program")
}

// RequestExecution explicitly starts the matched candidate, returned original,
// or unchanged original after a mode-only prepared trial (no0307 attempted).
// The UM is re-read before RUN/MONITOR; only one request per transfer is allowed.
// There is no implicit restoration of the mode observed during Prepare.
func (s *ProgramSession) RequestExecution(ctx context.Context, mode fins.CPUMode) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if mode != fins.CPUModeRun && mode != fins.CPUModeMonitor {
		return errors.New("execution requires explicit RUN or MONITOR")
	}
	if !s.controlled || s.finished || s.executionRequested || s.accessState != "acknowledged" || (s.state != "prepared" && s.state != "candidate-matched" && s.state != "original-matched") {
		return errors.New("execution requires matched bytes and active controlled session")
	}
	phase := "execute-" + s.state
	program := fins.CPUModeProgram
	if err := s.controlObservation(ctx, phase, &program); err != nil {
		return err
	}
	current, err := s.client.ReadProgram(ctx, um.MaxSize)
	if err != nil {
		return s.stop(phase, err, nil)
	}
	matched := bytes.Equal(current, s.original)
	if s.state == "candidate-matched" {
		comparison, err := s.candidateAudit.InspectReadback(current)
		if err != nil {
			return s.stop(phase, err, nil)
		}
		matched = comparison.Exact
	}
	if err = s.journal.Append(ProgramSessionEvent{Phase: phase, Kind: "execution-program-check", ReadbackData: bytes.Clone(current)}); err != nil {
		return s.stop(phase, err, nil)
	}
	if !matched {
		return s.stop(phase, errors.New("program changed before execution"), nil)
	}
	if len(s.originalCatalog) != 0 {
		if s.state == "candidate-matched" {
			_, err = checkSourceArchive(ctx, s.client, s.archive)
		} else {
			err = s.checkOriginalFiles(ctx, s.client)
		}
		if err != nil {
			return s.stop(phase, err, nil)
		}
	}
	if err = s.controlObservation(ctx, phase, &program); err != nil {
		return err
	}
	s.executionRequested = true
	if err = s.control(ctx, phase, "0401", fmt.Sprintf("ffff%02x", byte(mode)), &mode, func() error { return s.client.RequestCPUMode(ctx, mode) }); err != nil {
		return err
	}
	if err = s.controlObservation(ctx, phase, &mode); err != nil {
		return err
	}
	s.expectedMode = mode
	s.observedExecutionMode = copiedMode(&mode)
	return nil
}

// ReadExecutionTasks observes consecutive task flags during confirmed execution.
func (s *ProgramSession) ReadExecutionTasks(ctx context.Context, count uint16) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finished || s.state != "candidate-matched" || s.observedExecutionMode == nil || s.expectedMode == fins.CPUModeProgram {
		return nil, errors.New("task observation requires an executing candidate")
	}
	guard, err := s.observeMode(ctx, false)
	if err != nil {
		return nil, err
	}
	if !guard.Status.Executing() || guard.Status.Mode != byte(s.expectedMode) {
		return nil, errors.New("candidate no longer executing in requested mode")
	}
	flags, err := s.client.ReadMemory(ctx, fins.Memory{Area: 0x06, BitAccess: true}, count)
	if err != nil {
		return nil, err
	}
	err = s.journal.Append(ProgramSessionEvent{Phase: "execute-candidate-matched", Kind: "task-flags", Observation: &guard, ReadbackData: bytes.Clone(flags)})
	return flags, err
}

// RestoreOriginalInSession explicitly stops a matched candidate and rewrites the
// saved original while keeping the current access. The CPU stays in PROGRAM.
func (s *ProgramSession) RestoreOriginalInSession(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.controlled || s.finished || s.returnTried || !s.programEntered || s.accessState != "acknowledged" || s.state != "candidate-matched" {
		return errors.New("in-session original return requires an active matched candidate")
	}
	const phase = "original-return"
	if s.expectedMode != fins.CPUModeProgram {
		if err := s.controlObservation(ctx, phase, nil); err != nil {
			return err
		}
		program := fins.CPUModeProgram
		if err := s.control(ctx, phase, "0402", "ffff", &program, func() error { return s.client.RequestCPUMode(ctx, program) }); err != nil {
			return err
		}
		if err := s.controlObservation(ctx, phase, &program); err != nil {
			return err
		}
		s.expectedMode = program
	}
	s.returnTried = true
	s.state = "returning-original"
	return s.sendOriginal(ctx, s.binding)
}

// FinishProgram explicitly releases global access on the still-owned connection
// and closes it. It leaves the CPU in its observed requested mode. RELEASE may
// affect another owner: external exclusion remains necessary even after0C01 ACK.
// A stopped connection cannot be used for cleanup, and Close never releases.
func (s *ProgramSession) FinishProgram(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.controlled || s.finished || !s.programEntered || s.accessState != "acknowledged" || (s.state != "prepared" && s.state != "candidate-matched" && s.state != "original-matched") {
		return errors.New("finish requires active controlled session")
	}
	if err := s.controlObservation(ctx, "finish", &s.expectedMode); err != nil {
		return err
	}
	if err := s.control(ctx, "finish", "0C03", "ffff", nil, func() error { return s.client.ReleaseProgramAccess(ctx) }); err != nil {
		return err
	}
	if err := s.controlObservation(ctx, "finish", &s.expectedMode); err != nil {
		return err
	}
	if err := s.client.Close(); err != nil {
		return s.stop("finish-close", err, nil)
	}
	s.finished = true
	return nil
}

func copiedMode(mode *fins.CPUMode) *fins.CPUMode {
	if mode == nil {
		return nil
	}
	owned := *mode
	return &owned
}
