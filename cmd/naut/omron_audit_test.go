package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	omrontransfer "github.com/joyautomation/nautilus/omron/transfer"
	"github.com/joyautomation/nautilus/omron/um"
)

func TestOmronCompareServicePredictionsUseExpectedFile(t *testing.T) {
	const file = "../../omron/compiler/testdata/n1-no-fb-cj1h-h.n1"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var sum uint32
	for pos := 24; pos < len(data); pos += 4 {
		sum += binary.BigEndian.Uint32(data[pos:])
	}
	// Use a different, structurally valid UM. Its body length and checksum
	// differ, but place the expected file's predictions in its service header.
	other, err := os.ReadFile("../../omron/compiler/testdata/cj1m-f7-bool-control-unit30.n1")
	if err != nil {
		t.Fatal(err)
	}
	binary.BigEndian.PutUint32(other, sum&0xffff)
	binary.BigEndian.PutUint32(other[4:], sum>>16)
	binary.BigEndian.PutUint32(other[16:], uint32((len(data)+63)/64))
	path := filepath.Join(t.TempDir(), "other.um")
	if err = os.WriteFile(path, other, 0600); err != nil {
		t.Fatal(err)
	}
	raw, err := captureOmronBodyCommand(t, omronCompareReadback, []string{"-service-fields", file, path})
	var report struct {
		omrontransfer.ProgramReadbackComparison
		ExpectedSum32FromWord6            uint32
		PredictedPairMatchesObserved      bool
		Word4CandidatesFromExpectedLength []um.N1BlockCountCandidate
	}
	if err == nil || json.Unmarshal(raw, &report) != nil || report.Exact || report.BodyMatches || report.SameLength || !report.PredictedPairMatchesObserved || report.ExpectedSum32FromWord6 != sum || len(report.Word4CandidatesFromExpectedLength) != 4 {
		t.Fatal(string(raw), err)
	}
	for _, c := range report.Word4CandidatesFromExpectedLength {
		want := (uint32(len(data)) - c.ExcludedPrefixBytes + c.QuantumBytes - 1) / c.QuantumBytes
		if c.Count != want || c.StoredMatches != (want == binary.BigEndian.Uint32(other[16:])) {
			t.Fatal(c)
		}
	}
}

func TestOmronCompareReadback(t *testing.T) {
	const file = "../../omron/compiler/testdata/n1-no-fb-cj1h-h.n1"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, offset := range []int{-1, 3, len(data) - 1} {
		observed := bytes.Clone(data)
		if offset >= 0 {
			observed[offset] ^= 1
		}
		path := filepath.Join(t.TempDir(), "observed.um")
		if err := os.WriteFile(path, observed, 0600); err != nil {
			t.Fatal(err)
		}
		report, err := captureOmronBodyCommand(t, omronCompareReadback, []string{file, path})
		var c omrontransfer.ProgramReadbackComparison
		if json.Unmarshal(report, &c) != nil || (err == nil) != (offset == -1) || c.Exact != (offset == -1) ||
			c.FirstDifferenceByte != offset || c.OnlyServiceWords014Differ != (offset == 3) {
			t.Fatalf("offset %d: %s, %v", offset, report, err)
		}
	}
}

func syntheticOmronAuditTrace(t *testing.T) (string, []byte, omronProgramAuditTrace, []omronProgramAuditEvent) {
	t.Helper()
	file := "../../omron/compiler/testdata/n1-no-fb-cj1h-h.n1"
	image, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := omrontransfer.PlanProgramWrite(image, 996)
	if err != nil {
		t.Fatal(err)
	}
	events := []omronProgramAuditEvent{}
	for _, block := range plan.Blocks {
		request, err := hex.DecodeString(block.PayloadHex)
		if err != nil {
			t.Fatal(err)
		}
		// Synthetic candidate response, never captured from a PLC.
		response := append([]byte{3, 7, 0, 0}, request[:8]...)
		response[10] &= 0x7f
		events = append(events, omronProgramAuditEvent{Kind: "attempt", PayloadHex: block.PayloadHex}, omronProgramAuditEvent{Kind: "response", PayloadHex: hex.EncodeToString(response)})
	}
	return file, image, omronProgramAuditTrace{SchemaVersion: 1, Origin: "synthetic", ImageSHA256: plan.ImageSHA256, BlockBytes: 996}, events
}

func writeOmronAuditTrace(t *testing.T, trace omronProgramAuditTrace, events []omronProgramAuditEvent) string {
	t.Helper()
	raw, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	trace.Events = raw
	raw, err = json.Marshal(trace)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "trace.json")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestOmronAuditExactSyntheticReadbackAndIncompleteEvidence(t *testing.T) {
	image, _, trace, events := syntheticOmronAuditTrace(t)
	path := writeOmronAuditTrace(t, trace, events)
	data, err := captureOmronBodyCommand(t, omronAuditProgramWrite, []string{"-readback", image, image, path})
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		omrontransfer.ProgramWriteAuditSnapshot
		DeclaredTraceOrigin string
		EventsProcessed     int
		Error               string
	}
	if err = json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result.State != omrontransfer.ProgramAuditSourceBytesMatched || result.DeclaredTraceOrigin != "synthetic" || result.EventsProcessed != 6 || result.Error != "" || result.Readback == nil || !result.Readback.Exact || result.NetworkAccess || result.RouteSIDVerified || result.WriteQualified || result.HardwareQualified || result.RollbackPerformed {
		t.Fatal("incorrect offline completion", result)
	}
	data, err = captureOmronBodyCommand(t, omronAuditProgramWrite, []string{image, path})
	if err == nil {
		t.Fatal("ACK-only trace treated as completion")
	}
	if err = json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result.State != omrontransfer.ProgramAuditAwaitingReadback || !result.AllBlocksAcknowledged || !result.UncertainWrite || result.Error == "" {
		t.Fatal("missing readback omitted", result)
	}
	path = writeOmronAuditTrace(t, trace, events[:1])
	data, err = captureOmronBodyCommand(t, omronAuditProgramWrite, []string{image, path})
	if err == nil {
		t.Fatal("missing ACK accepted")
	}
	var prefix omrontransfer.ProgramWriteAuditSnapshot
	if err = json.Unmarshal(data, &prefix); err != nil {
		t.Fatal(err)
	}
	if prefix.State != omrontransfer.ProgramAuditAwaitingACK || prefix.StopReason != "" || prefix.AcknowledgedBytes != 0 || !prefix.UncertainWrite {
		t.Fatal("EOF invented transport outcome", prefix)
	}
}

func TestOmronAuditStopsAtBadACKAndServiceOnlyReadback(t *testing.T) {
	image, raw, trace, events := syntheticOmronAuditTrace(t)
	events[1].PayloadHex = "03170000ffff0000000003e4"
	path := writeOmronAuditTrace(t, trace, events)
	data, err := captureOmronBodyCommand(t, omronAuditProgramWrite, []string{image, path})
	if err == nil {
		t.Fatal("bad ACK accepted")
	}
	var result struct {
		omrontransfer.ProgramWriteAuditSnapshot
		EventsProcessed int
	}
	if err = json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result.State != omrontransfer.ProgramAuditStopped || result.EventsProcessed != 2 || result.AcknowledgedBytes != 0 || !result.ReconciliationRequired {
		t.Fatal("later events continued after bad ACK", result)
	}
	image, raw, trace, events = syntheticOmronAuditTrace(t)
	path = writeOmronAuditTrace(t, trace, events)
	changed := bytes.Clone(raw)
	changed[19] = 41
	readback := filepath.Join(t.TempDir(), "readback.n1")
	if err = os.WriteFile(readback, changed, 0600); err != nil {
		t.Fatal(err)
	}
	data, err = captureOmronBodyCommand(t, omronAuditProgramWrite, []string{"-readback", readback, image, path})
	if err == nil {
		t.Fatal("unqualified header stamping accepted")
	}
	if err = json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result.State != omrontransfer.ProgramAuditStopped || result.Readback == nil || !result.Readback.OnlyServiceWords014Differ || result.Readback.Exact || !result.ReconciliationRequired || result.WriteQualified {
		t.Fatal("service mismatch silently normalized", result)
	}
	unchanged, err := os.ReadFile(image)
	if err != nil || !bytes.Equal(unchanged, raw) {
		t.Fatal("source file changed")
	}
}

func TestOmronAuditRejectsMalformedOrUnboundTraceWithoutReport(t *testing.T) {
	image, _, trace, events := syntheticOmronAuditTrace(t)
	cases := map[string]func(*omronProgramAuditTrace, []omronProgramAuditEvent){
		"wrong-image":    func(tr *omronProgramAuditTrace, _ []omronProgramAuditEvent) { tr.ImageSHA256 = strings.Repeat("0", 64) },
		"missing-origin": func(tr *omronProgramAuditTrace, _ []omronProgramAuditEvent) { tr.Origin = "" },
		"wrong-schema":   func(tr *omronProgramAuditTrace, _ []omronProgramAuditEvent) { tr.SchemaVersion = 2 },
		"bad-block-size": func(tr *omronProgramAuditTrace, _ []omronProgramAuditEvent) { tr.BlockBytes = 1000 },
		"bad-hex":        func(_ *omronProgramAuditTrace, e []omronProgramAuditEvent) { e[0].PayloadHex = "zz" },
		"oversized-payload": func(_ *omronProgramAuditTrace, e []omronProgramAuditEvent) {
			e[0].PayloadHex = strings.Repeat("00", 1005)
		},
		"unsupported-event": func(_ *omronProgramAuditTrace, e []omronProgramAuditEvent) { e[0].Kind = "execute" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			tr := trace
			ev := append([]omronProgramAuditEvent(nil), events...)
			change(&tr, ev)
			path := writeOmronAuditTrace(t, tr, ev)
			if data, err := captureOmronBodyCommand(t, omronAuditProgramWrite, []string{image, path}); err == nil || len(data) != 0 {
				t.Fatal("invalid trace published", err)
			}
		})
	}
	path := writeOmronAuditTrace(t, trace, events)
	for _, args := range [][]string{nil, {"-endpoint", "10.0.174.37:9600", image, path}, {"-execute", image, path}, {image, path, "extra"}} {
		if data, err := captureOmronBodyCommand(t, omronAuditProgramWrite, args); err == nil || len(data) != 0 {
			t.Fatal("connection/execution option accepted", args)
		}
	}
	malformed := filepath.Join(t.TempDir(), "trace.json")
	for _, content := range []string{`{}`, `{"schemaVersion":1,"endpoint":"x"}`, `{} {}`} {
		if err := os.WriteFile(malformed, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		if data, err := captureOmronBodyCommand(t, omronAuditProgramWrite, []string{image, malformed}); err == nil || len(data) != 0 {
			t.Fatal("malformed trace accepted")
		}
	}
}

func TestOmronAuditInspectionAfterTimeoutDoesNotClearUncertainty(t *testing.T) {
	image, _, trace, events := syntheticOmronAuditTrace(t)
	path := writeOmronAuditTrace(t, trace, []omronProgramAuditEvent{events[0], {Kind: "failure"}})
	data, err := captureOmronBodyCommand(t, omronAuditProgramWrite, []string{"-readback", image, image, path})
	if err == nil {
		t.Fatal("readback cleared timeout")
	}
	var result struct {
		omrontransfer.ProgramWriteAuditSnapshot
		ReadbackInspection *omrontransfer.ProgramReadbackComparison
	}
	if err = json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result.State != omrontransfer.ProgramAuditStopped || !result.ReconciliationRequired || !result.UncertainWrite || result.Readback != nil || result.ReadbackInspection == nil || !result.ReadbackInspection.Exact || result.WriteQualified {
		t.Fatal("diagnostic inspection resumed audit", result)
	}
}

func TestOmronAuditIndependentSyntheticFixtures(t *testing.T) {
	image := "../../omron/compiler/testdata/n1-no-fb-cj1h-h.n1"
	data, err := captureOmronBodyCommand(t, omronAuditProgramWrite, []string{"-readback", image, image, "../../omron/transfer/testdata/program-audit-synthetic.json"})
	if err != nil {
		t.Fatal(err)
	}
	var result omrontransfer.ProgramWriteAuditSnapshot
	if err = json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result.State != omrontransfer.ProgramAuditSourceBytesMatched || result.AcknowledgedBytes != 2624 || result.WriteQualified {
		t.Fatal("independent synthetic fixture no longer matches plan", result)
	}
	data, err = captureOmronBodyCommand(t, omronAuditProgramWrite, []string{"-readback", image, image, "../../omron/transfer/testdata/program-audit-synthetic-timeout.json"})
	if err == nil {
		t.Fatal("independent timeout fixture accepted")
	}
	if err = json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result.State != omrontransfer.ProgramAuditStopped || !result.ReconciliationRequired || result.AcknowledgedBytes != 0 {
		t.Fatal("independent timeout fixture advanced audit", result)
	}
}
