package transfer_test

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/joyautomation/nautilus/omron/transfer"
	"github.com/joyautomation/nautilus/omron/um"
)

// All requests/responses in this file are synthetic, never sent to a PLC.
func TestCJ1MProbeReferencePair(t *testing.T) {
	wire, err := os.ReadFile("testdata/cj1m-probe/cx-wire.n1")
	if err != nil {
		t.Fatal(err)
	}
	resident, err := os.ReadFile("testdata/cj1m-probe/resident.um")
	if err != nil {
		t.Fatal(err)
	}
	a, err := transfer.NewCJ1MProbeAudit(wire, resident, 996)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		data  []byte
		exact bool
	}{{wire, false}, {resident, true}} {
		got, err := a.InspectReadback(tc.data)
		if err != nil || got.Exact != tc.exact {
			t.Fatalf("comparison=%v err=%v", got, err)
		}
	}
	resident[2] ^= 1
	if _, err := transfer.NewCJ1MProbeAudit(wire, resident, 996); err == nil {
		t.Fatal("accepted changed service header")
	}
}

func syntheticProgramACK(payload []byte) []byte {
	response := make([]byte, 12)
	binary.BigEndian.PutUint16(response, 0x0307)
	copy(response[4:10], payload[:6])
	binary.BigEndian.PutUint16(response[10:], binary.BigEndian.Uint16(payload[6:])&0x7fff)
	return response
}

func auditNext(t *testing.T, a *transfer.ProgramWriteAudit) []byte {
	t.Helper()
	block, err := a.ExpectedBlock()
	if err != nil {
		t.Fatal(err)
	}
	payload, err := hex.DecodeString(block.PayloadHex)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func auditACKNext(t *testing.T, a *transfer.ProgramWriteAudit) {
	t.Helper()
	payload := auditNext(t, a)
	if err := a.RecordAttempt(payload); err != nil {
		t.Fatal(err)
	}
	if _, err := a.RecordResponse(syntheticProgramACK(payload)); err != nil {
		t.Fatal(err)
	}
}

func auditAllACKed(t *testing.T) (*transfer.ProgramWriteAudit, []byte) {
	t.Helper()
	raw := nativeImage(t)
	a, err := transfer.NewProgramWriteAudit(raw, 996)
	if err != nil {
		t.Fatal(err)
	}
	for a.Snapshot().State == transfer.ProgramAuditReady {
		auditACKNext(t, a)
	}
	return a, raw
}

func TestProgramAuditRequiresReadbackAfterAllACKs(t *testing.T) {
	raw := nativeImage(t)
	saved := bytes.Clone(raw)
	a, err := transfer.NewProgramWriteAudit(raw, 996)
	if err != nil {
		t.Fatal(err)
	}
	raw[24] ^= 1 // Caller mutation cannot change the intended image or plan.
	if a.Snapshot().State != transfer.ProgramAuditReady {
		t.Fatal("wrong initial state")
	}
	for a.Snapshot().State == transfer.ProgramAuditReady {
		payload := auditNext(t, a)
		response := syntheticProgramACK(payload)
		before := a.Snapshot()
		if err = a.RecordAttempt(payload); err != nil {
			t.Fatal(err)
		}
		payload[len(payload)-1] ^= 1 // Pending request must own its bytes.
		if s := a.Snapshot(); s.State != transfer.ProgramAuditAwaitingACK || s.AcknowledgedBytes != before.AcknowledgedBytes || !s.UncertainWrite {
			t.Fatal("attempt acknowledged without response", s)
		}
		if _, err = a.ExpectedBlock(); err == nil {
			t.Fatal("next block offered before ACK")
		}
		if _, err = a.RecordResponse(response); err != nil {
			t.Fatal(err)
		}
		response[0] ^= 1
	}
	pending := a.Snapshot()
	if pending.State != transfer.ProgramAuditAwaitingReadback || !pending.AllBlocksAcknowledged || pending.AcknowledgedBlocks != 3 || pending.AcknowledgedBytes != len(saved) || pending.NextOffsetBytes != uint32(len(saved)) || !pending.UncertainWrite || pending.Readback != nil {
		t.Fatal("final ACK treated as verified image", pending)
	}
	comparison, err := a.RecordReadback(saved)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := a.Snapshot()
	if !comparison.Exact || comparison.FirstDifferenceByte != -1 || !comparison.BodyMatches || snapshot.State != transfer.ProgramAuditSourceBytesMatched || snapshot.UncertainWrite || snapshot.ReconciliationRequired || snapshot.NetworkAccess || snapshot.RouteSIDVerified || snapshot.HardwareQualified || snapshot.WriteQualified || snapshot.RollbackPerformed {
		t.Fatal("wrong offline completion scope", snapshot)
	}
	snapshot.Readback.Exact = false
	if !a.Snapshot().Readback.Exact || !bytes.Equal(saved, nativeImage(t)) {
		t.Fatal("snapshot or readback aliases audit state")
	}
	// Events after the last readback invalidate completion, rather than leaving
	// a stale successful status after a newly recorded transmission attempt.
	if err = a.RecordAttempt(auditPayloadForFirst(t, saved)); err == nil || a.Snapshot().State != transfer.ProgramAuditStopped || !a.Snapshot().UncertainWrite {
		t.Fatal("post-readback attempt left stale success")
	}
}

func auditPayloadForFirst(t *testing.T, raw []byte) []byte {
	t.Helper()
	plan, err := transfer.PlanProgramWrite(raw, 996)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := hex.DecodeString(plan.Blocks[0].PayloadHex)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func TestProgramAuditFailureAtEveryBlockStopsWithoutRetry(t *testing.T) {
	for failed := 0; failed < 3; failed++ {
		t.Run(string(rune('0'+failed)), func(t *testing.T) {
			a, err := transfer.NewProgramWriteAudit(nativeImage(t), 996)
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < failed; i++ {
				auditACKNext(t, a)
			}
			payload := auditNext(t, a)
			if err = a.RecordAttempt(payload); err != nil {
				t.Fatal(err)
			}
			if err = a.RecordFailure(); !errors.Is(err, transfer.ErrProgramWriteAudit) {
				t.Fatal("transport failure ignored")
			}
			stopped := a.Snapshot()
			if stopped.State != transfer.ProgramAuditStopped || stopped.AcknowledgedBlocks != failed || stopped.AcknowledgedBytes != 996*failed || !stopped.UncertainWrite || !stopped.ReconciliationRequired || stopped.RollbackPerformed {
				t.Fatal("uncertain progress accepted", stopped)
			}
			if _, err = a.ExpectedBlock(); err == nil {
				t.Fatal("retry offered")
			}
			if err = a.RecordAttempt(payload); err == nil {
				t.Fatal("stopped audit resumed")
			}
			if _, err = a.RecordResponse(syntheticProgramACK(payload)); err == nil {
				t.Fatal("late ACK resumed audit")
			}
			if _, err = a.RecordReadback(nativeImage(t)); err == nil {
				t.Fatal("readback silently resumed stopped audit")
			}
			if !reflect.DeepEqual(stopped, a.Snapshot()) {
				t.Fatal("terminal failure was rewritten")
			}
		})
	}
}

func TestProgramAuditRejectsBadACKWithoutProgress(t *testing.T) {
	for name, mutate := range map[string]func([]byte) []byte{
		"partial":       func(b []byte) []byte { binary.BigEndian.PutUint16(b[10:], 4); return b },
		"wrong-offset":  func(b []byte) []byte { binary.BigEndian.PutUint32(b[6:], 4); return b },
		"other-command": func(b []byte) []byte { b[1] = 0x17; return b },
		"plc-error":     func(b []byte) []byte { return []byte{3, 7, 0x11, 4} },
		"missing":       func([]byte) []byte { return nil },
	} {
		t.Run(name, func(t *testing.T) {
			a, err := transfer.NewProgramWriteAudit(nativeImage(t), 996)
			if err != nil {
				t.Fatal(err)
			}
			payload := auditNext(t, a)
			if err = a.RecordAttempt(payload); err != nil {
				t.Fatal(err)
			}
			ack, err := a.RecordResponse(mutate(syntheticProgramACK(payload)))
			s := a.Snapshot()
			if !errors.Is(err, transfer.ErrProgramWriteAudit) || !errors.Is(err, transfer.ErrProgramWriteResponse) || ack != (transfer.ProgramWriteACK{}) || s.AcknowledgedBlocks != 0 || s.AcknowledgedBytes != 0 || !s.ReconciliationRequired || s.State != transfer.ProgramAuditStopped {
				t.Fatal("invalid ACK advanced audit", s, err)
			}
		})
	}
}

func TestProgramAuditStopsOnRequestOrEventDeviation(t *testing.T) {
	cases := map[string]func(*transfer.ProgramWriteAudit, []byte) error{
		"changed-data": func(a *transfer.ProgramWriteAudit, p []byte) error { p[len(p)-1] ^= 1; return a.RecordAttempt(p) },
		"skipped-block": func(a *transfer.ProgramWriteAudit, p []byte) error {
			binary.BigEndian.PutUint32(p[2:], 996)
			return a.RecordAttempt(p)
		},
		"duplicate-attempt": func(a *transfer.ProgramWriteAudit, p []byte) error {
			if err := a.RecordAttempt(p); err != nil {
				return err
			}
			return a.RecordAttempt(p)
		},
		"stray-response": func(a *transfer.ProgramWriteAudit, p []byte) error {
			_, err := a.RecordResponse(syntheticProgramACK(p))
			return err
		},
		"early-readback": func(a *transfer.ProgramWriteAudit, _ []byte) error {
			_, err := a.RecordReadback(nativeImage(t))
			return err
		},
	}
	for name, action := range cases {
		t.Run(name, func(t *testing.T) {
			a, err := transfer.NewProgramWriteAudit(nativeImage(t), 996)
			if err != nil {
				t.Fatal(err)
			}
			if err = action(a, auditNext(t, a)); !errors.Is(err, transfer.ErrProgramWriteAudit) {
				t.Fatal("event deviation accepted")
			}
			if s := a.Snapshot(); s.State != transfer.ProgramAuditStopped || s.AcknowledgedBytes != 0 {
				t.Fatal("bad event advanced audit", s)
			}
		})
	}
	a, err := transfer.NewProgramWriteAudit(nativeImage(t), 996)
	if err != nil {
		t.Fatal(err)
	}
	payload := auditNext(t, a)
	auditACKNext(t, a)
	if _, err = a.RecordResponse(syntheticProgramACK(payload)); err == nil || a.Snapshot().AcknowledgedBlocks != 1 {
		t.Fatal("duplicate ACK advanced audit")
	}
}

func TestProgramAuditReadbackDifferencesRequireReconciliation(t *testing.T) {
	for name, mutate := range map[string]func([]byte) []byte{
		"service-014": func(b []byte) []byte {
			binary.BigEndian.PutUint32(b, 1)
			binary.BigEndian.PutUint32(b[4:], 2)
			binary.BigEndian.PutUint32(b[16:], 41)
			return b
		},
		"unknown-service-word": func(b []byte) []byte { b[8] ^= 1; return b },
		"body":                 func(b []byte) []byte { b[len(b)-1] ^= 1; return b },
		"truncated":            func(b []byte) []byte { return b[:len(b)-4] },
		"empty":                func([]byte) []byte { return nil },
	} {
		t.Run(name, func(t *testing.T) {
			a, raw := auditAllACKed(t)
			comparison, err := a.RecordReadback(mutate(bytes.Clone(raw)))
			s := a.Snapshot()
			if !errors.Is(err, transfer.ErrProgramWriteAudit) || comparison.Exact || s.State != transfer.ProgramAuditStopped || !s.AllBlocksAcknowledged || !s.ReconciliationRequired || s.Readback == nil || s.WriteQualified {
				t.Fatal("readback mismatch accepted", s)
			}
			if comparison.OnlyServiceWords014Differ != (name == "service-014") || comparison.BodyMatches != (name == "service-014" || name == "unknown-service-word") {
				t.Fatal("incorrect mismatch classification", comparison)
			}
		})
	}
	a, _ := auditAllACKed(t)
	if _, err := a.RecordReadback(make([]byte, um.MaxSize+1)); err == nil || a.Snapshot().StopReason != "readback-size-limit" {
		t.Fatal("unbounded readback accepted")
	}
}

func TestProgramAuditInvalidConstructionAndZeroValue(t *testing.T) {
	if a, err := transfer.NewProgramWriteAudit(nil, 996); a != nil || err == nil {
		t.Fatal("invalid image accepted")
	}
	if a, err := transfer.NewProgramWriteAudit(nativeImage(t), 1000); a != nil || err == nil {
		t.Fatal("invalid block bound accepted")
	}
	for _, a := range []*transfer.ProgramWriteAudit{nil, {}} {
		if a.Snapshot().State != transfer.ProgramAuditInvalid {
			t.Fatal("zero audit ready")
		}
		if _, err := a.ExpectedBlock(); err == nil {
			t.Fatal("zero audit offered block")
		}
		if err := a.RecordAttempt(nil); err == nil {
			t.Fatal("zero audit accepted attempt")
		}
		if _, err := a.RecordResponse(nil); err == nil {
			t.Fatal("zero audit accepted response")
		}
		if _, err := a.RecordReadback(nil); err == nil {
			t.Fatal("zero audit accepted readback")
		}
		if err := a.RecordFailure(); err == nil {
			t.Fatal("zero audit accepted failure")
		}
	}
}

func TestProgramAuditInspectionCannotResumeFailedTrace(t *testing.T) {
	a, raw := auditAllACKed(t)
	if err := a.RecordFailure(); err == nil {
		t.Fatal("failure accepted")
	}
	stopped := a.Snapshot()
	comparison, err := a.InspectReadback(raw)
	if err != nil || !comparison.Exact || !reflect.DeepEqual(stopped, a.Snapshot()) {
		t.Fatal("diagnostic readback changed failed audit", comparison, err)
	}
	if _, err = a.RecordReadback(raw); err == nil {
		t.Fatal("matching diagnostic resumed trace")
	}
	if _, err = a.InspectReadback(make([]byte, um.MaxSize+1)); err == nil || !reflect.DeepEqual(stopped, a.Snapshot()) {
		t.Fatal("inspection size limit changed terminal state")
	}
}
