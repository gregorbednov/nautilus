package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func transcript(t *testing.T, name string) []byte {
	t.Helper()
	file, err := os.Open("testdata/evidence/" + name + ".events.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	zip, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer zip.Close()
	data, err := io.ReadAll(zip)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestPortableOracleCorpus(t *testing.T) {
	data, err := os.ReadFile("testdata/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	report, err := verifyManifestReader("../../..", bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Cases) != 3 || report.Comparisons != 2 || report.HardwareQualified {
		t.Fatalf("bad corpus report: %+v", report)
	}
	for _, entry := range report.Cases {
		if !entry.SourceRecipeVerified || entry.SourceIdentityInCompilerVerified {
			t.Fatal("source recipe confused with observed source identity")
		}
		if entry.ID == "cj1h-baseline" {
			if entry.CaptureComplete || entry.BinaryBindingVerified {
				t.Fatal("historical copy promoted to embedded capture")
			}
		}
		if entry.ID == "cj1m-baseline" {
			if !entry.CaptureComplete || !entry.BinaryBindingVerified || entry.CPUIdentitySource != "runtime-initialization" {
				t.Fatal("current capture lost verified facts")
			}
		}
	}
}

func TestCaptureReplay(t *testing.T) {
	report, err := auditCapture(bytes.NewReader(transcript(t, "cj1m-baseline")), "")
	if err != nil {
		t.Fatal(err)
	}
	if !report.Complete || report.EventCount != 1125 || len(report.Compilations) != 1 {
		t.Fatalf("bad capture: %+v", report)
	}
	call := report.Compilations[0]
	if call.Sequence != 316 || call.ObservedCPU != "CJ1M CPU21" || call.Profile.Low != 1024 || call.Profile.Selector != 8 || call.FileSHA256 != "8a88c6e726542537787cea1419ccc42a1d203a37efb2304218c2a4e1687ce4f3" {
		t.Fatalf("wrong compilation: %+v", call)
	}
	if report.SourceIdentityVerified || report.HardwareQualified {
		t.Fatal("unsupported qualification")
	}
}

func mutate(t *testing.T, input []byte, kind string, change func(map[string]any)) []byte {
	t.Helper()
	lines := strings.Split(string(input), "\n")
	for index, line := range lines {
		if !strings.HasPrefix(line, "CJ1_ORACLE ") {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "CJ1_ORACLE ")), &entry); err != nil {
			t.Fatal(err)
		}
		if entry["kind"] == kind {
			change(entry)
			encoded, err := json.Marshal(entry)
			if err != nil {
				t.Fatal(err)
			}
			lines[index] = "CJ1_ORACLE " + string(encoded)
			return []byte(strings.Join(lines, "\n"))
		}
	}
	t.Fatalf("missing mutation event %s", kind)
	return nil
}

func TestCaptureRejectsBrokenEvidence(t *testing.T) {
	input := transcript(t, "cj1m-baseline")
	tests := []struct {
		name, kind, want string
		change           func(map[string]any)
	}{
		{"sequence", "instruction", "nonconsecutive", func(e map[string]any) { e["sequence"] = 99999 }},
		{"process", "compiler-file", "process changed", func(e map[string]any) { e["pid"] = 12345 }},
		{"orphan file", "compiler-file", "unique successful compile", func(e map[string]any) { e["thread"] = 12345 }},
		{"failed compile", "compile-leave", "unique successful compile", func(e map[string]any) { e["result"] = 0 }},
		{"file operation", "compiler-file", "operation mismatch", func(e map[string]any) { e["compileSequence"] = 1 }},
		{"return operation", "compile-leave", "operation mismatch", func(e map[string]any) { e["compileSequence"] = 1 }},
		{"file size", "compiler-file", "file length", func(e map[string]any) { e["byteLength"] = 1 }},
		{"file hex", "compiler-file", "file hex", func(e map[string]any) { e["fileHex"] = "zz" + e["fileHex"].(string)[2:] }},
		{"dropped", "capture-ended", "dropped-event", func(e map[string]any) { e["droppedEvents"] = 1 }},
		{"observer error", "instruction", "capture-error", func(e map[string]any) { e["kind"] = "capture-error" }},
		{"module", "module", "compiler module", func(e map[string]any) { e["expectedSHA256"] = "wrong" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			report, err := auditCapture(bytes.NewReader(mutate(t, input, test.kind, test.change)), "")
			if err != nil {
				t.Fatal(err)
			}
			if report.Complete || !strings.Contains(strings.Join(report.Problems, ";"), test.want) {
				t.Fatalf("accepted corrupted capture or wrong reason: %+v", report)
			}
		})
	}
	for _, stderr := range []string{"injector failed", "script exception"} {
		report, err := auditCapture(bytes.NewReader(input), stderr)
		if err != nil || report.Complete {
			t.Fatal("ignored stderr", err)
		}
	}
	lines := bytes.Split(input, []byte("\n"))
	truncated := bytes.Join(lines[:len(lines)-2], []byte("\n"))
	report, err := auditCapture(bytes.NewReader(truncated), "")
	if err != nil || report.Complete {
		t.Fatal("accepted missing capture-ended", err)
	}
}

func TestManifestRejectsUnsupportedClaims(t *testing.T) {
	data, err := os.ReadFile("testdata/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var original manifest
	if err := json.Unmarshal(data, &original); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		change func(*manifest)
	}{
		{"source recipe", func(m *manifest) { m.Cases[0].Source.Source = "LD W0.01\nOUT W0.01" }},
		{"cpu profile", func(m *manifest) { m.Cases[1].Capture.Profile.Low = 524288 }},
		{"cpu identity", func(m *manifest) {
			m.Cases[1].CPU = "CJ1H-H CPU65"
			m.Cases[1].CheckLog = nil
			m.Cases[1].ProgramCheckVersion = ""
		}},
		{"binary hash", func(m *manifest) { m.Cases[0].Image.SHA256 = strings.Repeat("0", 64) }},
		{"source claim", func(m *manifest) { m.Cases[0].SourceIdentity = "process-verified" }},
		{"legacy promotion", func(m *manifest) { m.Cases[0].Capture.Binding = "embedded-file" }},
		{"comparison", func(m *manifest) { m.Comparisons[1].Differences[0].Offset = 3282 }},
		{"duplicate id", func(m *manifest) { m.Cases[1].ID = m.Cases[0].ID }},
		{"traversal", func(m *manifest) { m.Cases[0].Image.Path = "../outside" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var changed manifest
			if err := json.Unmarshal(data, &changed); err != nil {
				t.Fatal(err)
			}
			test.change(&changed)
			encoded, err := json.Marshal(changed)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := verifyManifestReader("../../..", bytes.NewReader(encoded)); err == nil {
				t.Fatal("accepted inconsistent manifest")
			}
		})
	}
	for _, path := range []string{"../outside", `C:\outside`, "/outside", "a/../outside"} {
		if _, err := localPath(t.TempDir(), path); err == nil {
			t.Fatalf("accepted path %q", path)
		}
	}
	// Root resolution must be independent of the caller's current directory.
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifyManifestReader(root, bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
}

func TestSourceCheckResultIsParsedExactly(t *testing.T) {
	good := "PLC Model 'CJ1M CPU21'\nNewPLC1 - 0 errors, 79 warnings.\nThe programs have been checked with the program check option set to Unit Ver.4.0.\n"
	if err := validateSourceCheck([]byte(good), "CJ1M CPU21", "4.0"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{strings.Replace(good, "0 errors", "100 errors", 1), strings.Replace(good, "0 errors", "1 errors", 1), good + good, strings.Replace(good, "CPU21", "CPU65", 1), strings.Replace(good, "Ver.4.0", "Ver.3.0", 1)} {
		if err := validateSourceCheck([]byte(bad), "CJ1M CPU21", "4.0"); err == nil {
			t.Fatal("accepted mismatched or failed source check")
		}
	}
}

func TestArtifactCannotEscapeViaSymlink(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.bin")
	if err := os.WriteFile(outside, []byte("outside"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skip("symlink privilege unavailable:", err)
	}
	if _, err := localPath(root, "link"); err == nil {
		t.Fatal("followed artifact outside corpus root")
	}
}

func TestAuditCaptureCLIReportsFailure(t *testing.T) {
	root := t.TempDir()
	logPath := filepath.Join(root, "events.log")
	stderrPath := filepath.Join(root, "stderr.log")
	if err := os.WriteFile(logPath, transcript(t, "cj1m-baseline"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stderrPath, []byte("injector error"), 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := run([]string{"audit-capture", "-log", logPath, "-stderr", stderrPath}, &output); err == nil {
		t.Fatal("CLI accepted stderr failure")
	}
	var report captureReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Complete || len(report.Problems) == 0 {
		t.Fatal("CLI failed without a usable report")
	}
}
