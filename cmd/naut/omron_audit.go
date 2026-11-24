package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	omrontransfer "github.com/joyautomation/nautilus/omron/transfer"
	"github.com/joyautomation/nautilus/omron/um"
)

const maxOmronAuditTraceBytes = 40 << 20

func omronCompareReadback(args []string) error {
	flags := flag.NewFlagSet("compare-readback", flag.ContinueOnError)
	service := flags.Bool("service-fields", false, "compare service field hypotheses from the expected N1 against the observed UM")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 2 {
		return errors.New("compare-readback requires an expected N1 and an observed UM file")
	}
	expected, err := omronAuditReadFile(flags.Arg(0), um.MaxSize)
	if err != nil {
		return err
	}
	image, err := um.ParseN1(expected)
	if err != nil {
		return fmt.Errorf("expected N1: %w", err)
	}
	observed, err := omronAuditReadFile(flags.Arg(1), um.MaxSize)
	if err != nil {
		return err
	}
	comparison := omrontransfer.CompareProgramReadback(expected, observed)
	var report any = comparison
	if *service {
		readback, err := um.ParseN1(observed)
		if err != nil {
			return fmt.Errorf("observed UM: %w", err)
		}
		prediction := image.ServiceFields()
		stored := readback.ServiceFields().Words
		pair := [2]uint32{prediction.Sum32FromWord6 & 0xffff, prediction.Sum32FromWord6 >> 16}
		candidates := prediction.BlockCountCandidates
		for i := range candidates {
			candidates[i].StoredMatches = candidates[i].Count == stored[4]
		}
		report = struct {
			omrontransfer.ProgramReadbackComparison
			ExpectedSum32 uint32                     `json:"expectedSum32FromWord6"`
			ExpectedPair  [2]uint32                  `json:"predictedWords01"`
			ObservedWords [6]uint32                  `json:"observedServiceWords"`
			PairMatches   bool                       `json:"predictedPairMatchesObserved"`
			Counts        []um.N1BlockCountCandidate `json:"word4CandidatesFromExpectedLength"`
		}{comparison, prediction.Sum32FromWord6, pair, stored, pair == [2]uint32{stored[0], stored[1]}, candidates}
	}
	if err := json.NewEncoder(os.Stdout).Encode(report); err != nil {
		return err
	}
	if !comparison.Exact {
		return errors.New("observed UM differs from the expected N1")
	}
	return nil
}

type omronProgramAuditTrace struct {
	SchemaVersion int             `json:"schemaVersion"`
	Origin        string          `json:"origin"`
	ImageSHA256   string          `json:"imageSHA256"`
	BlockBytes    int             `json:"blockBytes"`
	Events        json.RawMessage `json:"events"`
}

type omronProgramAuditEvent struct {
	Kind       string `json:"kind"`
	PayloadHex string `json:"payloadHex,omitempty"`
}

func omronAuditReadFile(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("offline audit input exceeds size limit")
	}
	return data, nil
}

func omronAuditProgramWrite(args []string) error {
	flags := flag.NewFlagSet("omron audit-program-write", flag.ContinueOnError)
	readbackFile := flags.String("readback", "", "retained readback file; compare only after all exact ACKs")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 2 {
		return errors.New("expected N1 image and trace JSON filenames; offline only")
	}
	image, err := omronAuditReadFile(flags.Arg(0), um.MaxSize)
	if err != nil {
		return err
	}
	raw, err := omronAuditReadFile(flags.Arg(1), maxOmronAuditTraceBytes)
	if err != nil {
		return err
	}
	var trace omronProgramAuditTrace
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&trace); err != nil {
		return fmt.Errorf("invalid audit trace: %w", err)
	}
	var extra any
	if err = decoder.Decode(&extra); err != io.EOF {
		return errors.New("audit trace must contain exactly one JSON value")
	}
	if trace.SchemaVersion != 1 || (trace.Origin != "synthetic" && trace.Origin != "retained") {
		return errors.New("trace requires schemaVersion1 and explicit synthetic/retained origin")
	}
	audit, err := omrontransfer.NewProgramWriteAudit(image, trace.BlockBytes)
	if err != nil {
		return err
	}
	if trace.ImageSHA256 != audit.Snapshot().ImageSHA256 {
		return errors.New("trace image SHA256 does not match supplied image")
	}
	events := json.NewDecoder(bytes.NewReader(trace.Events))
	events.DisallowUnknownFields()
	token, err := events.Token()
	if err != nil || token != json.Delim('[') {
		return errors.New("trace events must be an array")
	}
	var auditErr error
	processed := 0
	limit := 2*audit.Snapshot().PlannedBlocks + 1
	for events.More() {
		if processed >= limit {
			return errors.New("audit trace event count exceeds plan bound")
		}
		var event omronProgramAuditEvent
		if err = events.Decode(&event); err != nil {
			return fmt.Errorf("invalid audit event: %w", err)
		}
		if event.Kind != "attempt" && event.Kind != "response" && event.Kind != "failure" {
			return errors.New("unsupported audit event kind")
		}
		if len(event.PayloadHex) > 2*(8+omrontransfer.MaxProgramWriteBytes) || (event.Kind == "failure" && event.PayloadHex != "") {
			return errors.New("invalid audit event payload bound")
		}
		payload, err := hex.DecodeString(event.PayloadHex)
		if err != nil {
			return fmt.Errorf("invalid audit event hex: %w", err)
		}
		processed++
		switch event.Kind {
		case "attempt":
			auditErr = audit.RecordAttempt(payload)
		case "response":
			_, auditErr = audit.RecordResponse(payload)
		case "failure":
			auditErr = audit.RecordFailure()
		}
		// The report covers only the prefix through the first protocol failure.
		// Later records cannot authorize continuation of this stopped sequence.
		if auditErr != nil {
			break
		}
	}
	if auditErr == nil {
		if token, err = events.Token(); err != nil || token != json.Delim(']') {
			return errors.New("invalid audit event array")
		}
	}
	var inspection *omrontransfer.ProgramReadbackComparison
	if *readbackFile != "" {
		readback, err := omronAuditReadFile(*readbackFile, um.MaxSize)
		if err != nil {
			return err
		}
		if auditErr == nil && audit.Snapshot().State == omrontransfer.ProgramAuditAwaitingReadback {
			_, auditErr = audit.RecordReadback(readback)
		} else {
			comparison, err := audit.InspectReadback(readback)
			if err != nil {
				return err
			}
			inspection = &comparison
		}
	}
	if auditErr == nil && audit.Snapshot().State != omrontransfer.ProgramAuditSourceBytesMatched {
		auditErr = errors.New("retained evidence incomplete; all exact ACKs and matching readback are required")
	}

	report := struct {
		omrontransfer.ProgramWriteAuditSnapshot
		ReadbackInspection  *omrontransfer.ProgramReadbackComparison `json:"readbackInspection,omitempty"`
		SchemaVersion       int                                      `json:"schemaVersion"`
		DeclaredTraceOrigin string                                   `json:"declaredTraceOrigin"`
		EventsProcessed     int                                      `json:"eventsProcessed"`
		Error               string                                   `json:"error,omitempty"`
	}{ProgramWriteAuditSnapshot: audit.Snapshot(), ReadbackInspection: inspection, SchemaVersion: 1, DeclaredTraceOrigin: trace.Origin, EventsProcessed: processed}
	if auditErr != nil {
		report.Error = auditErr.Error()
	}
	output := json.NewEncoder(os.Stdout)
	output.SetIndent("", "  ")
	if err = output.Encode(report); err != nil {
		return err
	}
	return auditErr
}
