package commission_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/joyautomation/nautilus/omron/codec"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/joyautomation/nautilus/omron/commission"
	"github.com/joyautomation/nautilus/omron/fins"
)

func controlledSession(t *testing.T, p *programPeer, j commission.ProgramSessionJournal) *commission.ProgramSession {
	t.Helper()
	p.mu.Lock()
	p.controlEnabled = true
	p.status[1] = 4
	p.mu.Unlock()
	c := p.dial(t)
	s, err := commission.NewControlledProgramSession(c, sessionConfig(t, p), j)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func controlList(p *programPeer) []uint16 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]uint16(nil), p.controls...)
}
func owner(p *programPeer) byte { p.mu.Lock(); defer p.mu.Unlock(); return p.owner }
func mustPrepareEnter(t *testing.T, s *commission.ProgramSession) {
	t.Helper()
	if err := s.Prepare(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.EnterProgram(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestControlledRewriteOriginal(t *testing.T) {
	p := newProgramPeer(t)
	j := &memoryJournal{}
	s := controlledSession(t, p, j)
	original := p.programBytes()
	ctx := context.Background()
	if err := s.Prepare(ctx); err != nil {
		t.Fatal(err)
	}
	if s.RewriteOriginal(ctx, original) == nil {
		t.Fatal("accepted before PROGRAM")
	}
	if err := s.EnterProgram(ctx); err != nil {
		t.Fatal(err)
	}
	if s.RewriteOriginal(ctx, []byte("wrong")) == nil {
		t.Fatal("accepted wrong original")
	}
	if err := s.RewriteOriginal(ctx, original); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, p.programBytes()) || !s.Snapshot().ProgramBytesReconciled ||
		s.Snapshot().Candidate.AttemptedBlocks != 0 || s.Snapshot().OriginalReturn.AttemptedBlocks == 0 {
		t.Fatal(s.Snapshot())
	}
	if s.RewriteOriginal(ctx, original) == nil || s.WriteCandidate(ctx) == nil {
		t.Fatal("permitted second write")
	}
	if err := s.FinishProgram(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestControlledSessionBackupControlTransferFinishAndFreshReturn(t *testing.T) {
	p := newProgramPeer(t)
	original := p.programBytes()
	j := &memoryJournal{}
	s := controlledSession(t, p, j)
	ctx := context.Background()
	if err := s.Prepare(ctx); err != nil {
		t.Fatal(err)
	}
	if len(controlList(p)) != 0 || j.bundle.Before.Status.Mode != 4 {
		t.Fatal("control before RUN backup")
	}
	if s.WriteCandidate(ctx) == nil || len(controlList(p)) != 0 {
		t.Fatal("write skipped explicit control")
	}
	if err := s.EnterProgram(ctx); err != nil {
		t.Fatal(err)
	}
	if s.EnterProgram(ctx) == nil {
		t.Fatal("entered twice")
	}
	if err := s.WriteCandidate(ctx); err != nil {
		t.Fatal(err)
	}
	fresh := p.dial(t)
	defer fresh.Close()
	if s.ReturnOriginal(ctx, fresh) == nil {
		t.Fatal("abandoned acknowledged access for fresh return")
	}
	if err := s.RequestExecution(ctx, fins.CPUModeMonitor); err != nil {
		t.Fatal(err)
	}
	if mode := s.Snapshot().ObservedExecutionMode; mode == nil || *mode != fins.CPUModeMonitor {
		t.Fatal("confirmed MONITOR execution was not recorded")
	}
	if s.RequestExecution(ctx, fins.CPUModeRun) == nil {
		t.Fatal("execution repeated")
	}
	if err := s.FinishProgram(ctx); err != nil {
		t.Fatal(err)
	}
	if !s.Snapshot().Finished || owner(p) != 0 {
		t.Fatal("finish missing", s.Snapshot())
	}
	if s.FinishProgram(ctx) == nil {
		t.Fatal("finished twice")
	}
	if err := s.ReturnOriginal(ctx, fresh); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(p.programBytes(), original) || !s.Snapshot().ProgramBytesReconciled {
		t.Fatal("original bytes differ")
	}
	if err := s.FinishProgram(ctx); err != nil {
		t.Fatal(err)
	}
	expected := []uint16{0x0c01, 0x0402, 0x0401, 0x0c03, 0x0c01, 0x0402, 0x0c03}
	if !reflect.DeepEqual(controlList(p), expected) {
		t.Fatal("unexpected control workflow", controlList(p))
	}
	// Each command intent is persisted before its ACK, with explicit source mode.
	var pending string
	attempts := 0
	for _, e := range j.events {
		if e.Kind == "control-attempt" {
			if pending != "" {
				t.Fatal("overlapping controls")
			}
			pending = e.Command
			attempts++
			if _, err := hex.DecodeString(e.PayloadHex); err != nil {
				t.Fatal(err)
			}
		}
		if e.Kind == "control-ack" {
			if e.Command != pending {
				t.Fatal("ACK lacks intent")
			}
			pending = ""
		}
	}
	if pending != "" || attempts != len(expected) {
		t.Fatal("incomplete control journal")
	}
	snap := s.Snapshot()
	if !snap.Controlled || snap.AccessState != "released" || !snap.Finished || snap.HardwareQualified || snap.RestoreQualified || snap.FullControllerBackup {
		t.Fatal(snap)
	}
}

type controlFaultJournal struct {
	memoryJournal
	command, kind string
	mutate        bool
}

func (j *controlFaultJournal) Append(e commission.ProgramSessionEvent) error {
	if j.mutate && e.RequestedMode != nil {
		*e.RequestedMode = fins.CPUModeRun
	}
	if e.Command == j.command && e.Kind == j.kind {
		return errors.New("control journal failed")
	}
	return j.memoryJournal.Append(e)
}
func TestControlledSessionControlFailuresNeverCleanupOrRun(t *testing.T) {
	for _, fault := range []string{"backup-save", "acquire-intent", "acquire-ack", "acquire-lost", "busy", "stop-intent", "stop-ack", "stop-lost", "mode-not-observed", "changed-program"} {
		t.Run(fault, func(t *testing.T) {
			p := newProgramPeer(t)
			j := &controlFaultJournal{}
			s := controlledSession(t, p, j)
			if fault == "backup-save" {
				j.fail = "save"
			}
			err := s.Prepare(context.Background())
			if err == nil {
				switch fault {
				case "acquire-intent":
					j.command = "0C01"
					j.kind = "control-attempt"
				case "acquire-ack":
					j.command = "0C01"
					j.kind = "control-ack"
				case "stop-intent":
					j.command = "0402"
					j.kind = "control-attempt"
				case "stop-ack":
					j.command = "0402"
					j.kind = "control-ack"
				}
				p.mu.Lock()
				switch fault {
				case "acquire-lost":
					p.dropControl = 0x0c01
				case "stop-lost":
					p.dropControl = 0x0402
				case "busy":
					p.owner = 77
				case "mode-not-observed":
					p.ignoreMode = true
				case "changed-program":
					p.stored[0] ^= 1
				}
				p.mu.Unlock()
				err = s.EnterProgram(context.Background())
			}
			if err == nil || s.Snapshot().State != "stopped" || p.writeCount() != 0 {
				t.Fatal("control failure ignored", err, s.Snapshot())
			}
			if fault == "busy" {
				var busy *fins.AccessRightBusyError
				if !errors.As(err, &busy) || busy.Owner.Node != 77 {
					t.Fatal("busy diagnostics lost", err)
				}
			}
			before := controlList(p)
			for _, command := range before {
				if command == 0x0401 || command == 0x0c02 || command == 0x0c03 {
					t.Fatal("automatic execution/force/cleanup", before)
				}
			}
			if s.EnterProgram(context.Background()) == nil || s.FinishProgram(context.Background()) == nil {
				t.Fatal("stopped session resumed")
			}
			_ = s.Close()
			if !reflect.DeepEqual(controlList(p), before) {
				t.Fatal("Close sent cleanup")
			}
		})
	}
}
func TestControlledSessionFinishFailuresDoNotRetryOrReconnect(t *testing.T) {
	for _, fault := range []string{"intent", "ack", "lost", "guard"} {
		t.Run(fault, func(t *testing.T) {
			p := newProgramPeer(t)
			j := &controlFaultJournal{}
			s := controlledSession(t, p, j)
			mustPrepareEnter(t, s)
			switch fault {
			case "intent":
				j.command = "0C03"
				j.kind = "control-attempt"
			case "ack":
				j.command = "0C03"
				j.kind = "control-ack"
			}
			p.mu.Lock()
			if fault == "lost" {
				p.dropControl = 0x0c03
			}
			if fault == "guard" {
				p.status[1] = 4
			}
			p.mu.Unlock()
			if s.FinishProgram(context.Background()) == nil || s.Snapshot().Finished {
				t.Fatal("finish failure ignored")
			}
			before := controlList(p)
			if s.FinishProgram(context.Background()) == nil {
				t.Fatal("release retried")
			}
			_ = s.Close()
			if !reflect.DeepEqual(before, controlList(p)) {
				t.Fatal("extra control after failure")
			}
			want := 2
			if fault == "ack" || fault == "lost" {
				want = 3
			}
			if len(before) != want {
				t.Fatal(before)
			}
		})
	}
}
func TestControlledSessionFreshReturnCannotInheritOldAccess(t *testing.T) {
	p := newProgramPeer(t)
	s := controlledSession(t, p, &memoryJournal{})
	mustPrepareEnter(t, s)
	p.mu.Lock()
	p.dropACK = true
	p.mu.Unlock()
	if s.WriteCandidate(context.Background()) == nil {
		t.Fatal("lost write ACK accepted")
	}
	previous := owner(p)
	if previous == 0 {
		t.Fatal("peer did not retain access")
	}
	err := s.ReturnOriginal(context.Background(), p.dial(t))
	var busy *fins.AccessRightBusyError
	if !errors.As(err, &busy) || busy.Owner.Node != previous {
		t.Fatal("fresh return assumed ownership", err)
	}
	if !reflect.DeepEqual(controlList(p), []uint16{0x0c01, 0x0402, 0x0c01}) || p.writeCount() != 1 {
		t.Fatal("force/release/write after busy", controlList(p))
	}
}
func TestControlledSessionNoImplicitExecutionAndJournalOwnsModeCopies(t *testing.T) {
	p := newProgramPeer(t)
	j := &controlFaultJournal{mutate: true}
	s := controlledSession(t, p, j)
	mustPrepareEnter(t, s)
	if err := s.WriteCandidate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if s.RequestExecution(context.Background(), fins.CPUModeProgram) == nil {
		t.Fatal("invalid execution mode")
	}
	if err := s.FinishProgram(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(controlList(p), []uint16{0x0c01, 0x0402, 0x0c03}) {
		t.Fatal("journal mutation caused RUN", controlList(p))
	}
}
func TestControlledSessionExecutionRechecksBytesAndACK(t *testing.T) {
	for _, fault := range []string{"changed", "intent", "ack", "lost", "mode", "stopped"} {
		t.Run(fault, func(t *testing.T) {
			p := newProgramPeer(t)
			j := &controlFaultJournal{}
			s := controlledSession(t, p, j)
			mustPrepareEnter(t, s)
			if err := s.WriteCandidate(context.Background()); err != nil {
				t.Fatal(err)
			}
			switch fault {
			case "intent":
				j.command = "0401"
				j.kind = "control-attempt"
			case "ack":
				j.command = "0401"
				j.kind = "control-ack"
			}
			p.mu.Lock()
			switch fault {
			case "changed":
				p.stored[0] ^= 1
			case "stopped":
				p.stayStopped = true
			case "lost":
				p.dropControl = 0x0401
			case "mode":
				p.ignoreMode = true
			}
			p.mu.Unlock()
			if s.RequestExecution(context.Background(), fins.CPUModeRun) == nil {
				t.Fatal("execution guard ignored")
			}
			if s.Snapshot().ObservedExecutionMode != nil {
				t.Fatal("failed execution request reported a confirmed mode")
			}
			before := controlList(p)
			if s.RequestExecution(context.Background(), fins.CPUModeRun) == nil || s.FinishProgram(context.Background()) == nil {
				t.Fatal("execution/cleanup resumed")
			}
			if !reflect.DeepEqual(before, controlList(p)) {
				t.Fatal("extra control")
			}
		})
	}
}

func TestControlledSessionFileJournalBothModels(t *testing.T) {
	for _, spec := range []struct{ name, model string }{{"cj1h", codec.CJ1HCPU65}, {"cj1m", codec.CJ1MCPU21}} {
		t.Run(spec.name, func(t *testing.T) {
			p := newProgramPeer(t)
			p.mu.Lock()
			p.cpu = observation(t, spec.name).CPU.Raw
			p.status[1] = 4
			p.controlEnabled = true
			p.mu.Unlock()
			dir := filepath.Join(t.TempDir(), "trial")
			j, err := commission.NewFileProgramJournal(dir)
			if err != nil {
				t.Fatal(err)
			}
			s, err := commission.NewControlledProgramSession(p.dial(t), sessionConfigForModel(t, p, spec.model), j)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			mustPrepareEnter(t, s)
			if err = s.WriteCandidate(context.Background()); err != nil {
				t.Fatal(err)
			}
			if err = s.FinishProgram(context.Background()); err != nil {
				t.Fatal(err)
			}
			if err = s.ReturnOriginal(context.Background(), p.dial(t)); err != nil {
				t.Fatal(err)
			}
			if err = s.RequestExecution(context.Background(), fins.CPUModeRun); err != nil {
				t.Fatal(err)
			}
			if err = s.FinishProgram(context.Background()); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(filepath.Join(dir, "events.jsonl"))
			if err != nil {
				t.Fatal(err)
			}
			var sources []byte
			observedProgram, observedRun := false, false
			for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte{'\n'}) {
				var e commission.ProgramSessionEvent
				if err = json.Unmarshal(line, &e); err != nil {
					t.Fatal(err)
				}
				if e.Kind == "control-attempt" && e.Command == "0C01" {
					if e.Binding == nil {
						t.Fatal("missing control route")
					}
					sources = append(sources, e.Binding.Route.Source.Node)
				}
				if e.Kind == "control-observation" && e.RequestedMode != nil && e.Observation != nil {
					if e.Observation.Status.Mode != byte(*e.RequestedMode) {
						t.Fatal("mode not confirmed")
					}
					if *e.RequestedMode == fins.CPUModeProgram {
						observedProgram = true
					}
					if *e.RequestedMode == fins.CPUModeRun {
						observedRun = true
					}
				}
			}
			if len(sources) != 2 || sources[0] == sources[1] || !observedProgram || !observedRun || !s.Snapshot().Finished || !s.Snapshot().ProgramBytesReconciled {
				t.Fatal("incomplete controlled file trace", sources, s.Snapshot())
			}
		})
	}
}
func TestControlledSessionCloseDoesNotReleaseAcknowledgedAccess(t *testing.T) {
	p := newProgramPeer(t)
	s := controlledSession(t, p, &memoryJournal{})
	mustPrepareEnter(t, s)
	previous := owner(p)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if previous == 0 || owner(p) != previous || s.Snapshot().AccessState != "uncertain" || s.Snapshot().Finished || !reflect.DeepEqual(controlList(p), []uint16{0x0c01, 0x0402}) {
		t.Fatal("Close claimed release", s.Snapshot())
	}
}

func TestControlledSessionModeOnlyTrialRestartsOnlyUnchangedOriginal(t *testing.T) {
	for _, mode := range []fins.CPUMode{fins.CPUModeRun, fins.CPUModeMonitor} {
		t.Run(fmt.Sprintf("%02x", mode), func(t *testing.T) {
			p := newProgramPeer(t)
			original := p.programBytes()
			s := controlledSession(t, p, &memoryJournal{})
			mustPrepareEnter(t, s)
			if err := s.RequestExecution(context.Background(), mode); err != nil {
				t.Fatal(err)
			}
			if s.WriteCandidate(context.Background()) == nil {
				t.Fatal("write allowed after explicit execution")
			}
			if err := s.FinishProgram(context.Background()); err != nil {
				t.Fatal(err)
			}
			if p.writeCount() != 0 || !bytes.Equal(original, p.programBytes()) || !s.Snapshot().Finished || s.Snapshot().ProgramBytesReconciled {
				t.Fatal("mode-only trial changed or claimed restoration")
			}
			if !reflect.DeepEqual(controlList(p), []uint16{0x0c01, 0x0402, 0x0401, 0x0c03}) {
				t.Fatal(controlList(p))
			}
		})
	}
}
