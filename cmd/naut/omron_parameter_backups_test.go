package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOmronCompareParameterBackups(t *testing.T) {
	snapshot, err := os.ReadFile(preflightSnapshotFile(t))
	if err != nil {
		t.Fatal(err)
	}
	baseline, observed := t.TempDir(), t.TempDir()
	writePair := func(root string, area parameterCaptureArea, changed bool, endpoint string) {
		t.Helper()
		dir := filepath.Join(root, "cj1m-"+area.name)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		data := make([]byte, area.code.ByteLength())
		if changed {
			data[17] = 1
			data[20] = 3
		}
		hash := fmt.Sprintf("%x", sha256.Sum256(data))
		meta := map[string]any{"before": json.RawMessage(snapshot), "after": json.RawMessage(snapshot), "area": area.name, "target": "cj1m", "code": fmt.Sprintf("%04x", area.code), "endpoint": endpoint,
			"expectedBytes": len(data), "firstBytes": len(data), "secondBytes": len(data), "firstSHA256": hash, "secondSHA256": hash}
		record, err := json.Marshal(meta)
		if err != nil {
			t.Fatal(err)
		}
		for name, b := range map[string][]byte{"copy-1.bin": data, "copy-2.bin": data, "result.json": record} {
			if err = os.WriteFile(filepath.Join(dir, name), b, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, a := range capturedParameterAreas {
		writePair(baseline, a, false, "cpu:9600")
		writePair(observed, a, false, "cpu:9600")
	}
	run := func() ([]byte, error) {
		return captureOmronBodyCommand(t, omronCompareParameterBackups, []string{"-model", "CJ1M CPU21", baseline, observed})
	}
	if _, err := run(); err != nil {
		t.Fatal(err)
	}
	area := capturedParameterAreas[1]
	writePair(observed, area, true, "cpu:9600")
	raw, err := run()
	var report struct {
		AllAreasEqual, RestoreQualified bool
		Areas                           []parameterDifference
	}
	if err == nil || json.Unmarshal(raw, &report) != nil || report.AllAreasEqual || report.RestoreQualified || len(report.Areas) != 4 || report.Areas[1].FirstDifferenceByte != 17 || report.Areas[1].ChangedBytes != 2 {
		t.Fatal(string(raw), err)
	}
	writePair(observed, area, true, "other:9600")
	if b, e := run(); e == nil || len(b) != 0 || !strings.Contains(e.Error(), "different recorded endpoint") {
		t.Fatal(string(b), e)
	}
	writePair(observed, area, true, "cpu:9600")
	if err = os.WriteFile(filepath.Join(observed, "cj1m-io-table", "copy-2.bin"), []byte{1}, 0600); err != nil {
		t.Fatal(err)
	}
	if b, e := run(); e == nil || len(b) != 0 || !strings.Contains(e.Error(), "retained pair") {
		t.Fatal(string(b), e)
	}
}
