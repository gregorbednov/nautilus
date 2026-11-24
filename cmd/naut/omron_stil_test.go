package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOmronBuildSTILProject(t *testing.T) {
	dir := t.TempDir()
	fixture := "../../omron/internal/research/cpu21sample"
	for _, name := range []string{"cj1.json", "settings.json", "program.json", "Temperature.st", "Humidity.st"} {
		raw, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	source := filepath.Join(dir, "program.json")
	raw, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	edit := func(old, new string) {
		t.Helper()
		if !bytes.Contains(raw, []byte(old)) {
			t.Fatal("fixture missing", old)
		}
		if err := os.WriteFile(source, bytes.Replace(raw, []byte(old), []byte(new), 1), 0600); err != nil {
			t.Fatal(err)
		}
	}
	build := func() []byte {
		t.Helper()
		report, err := captureOmronBodyCommand(t, omronBuildProject, []string{dir})
		if err != nil {
			t.Fatal(err)
		}
		var r struct {
			Output            string
			HardwareQualified bool
		}
		if err = json.Unmarshal(report, &r); err != nil || r.HardwareQualified {
			t.Fatal(string(report), err)
		}
		image, err := os.ReadFile(r.Output)
		if err != nil {
			t.Fatal(err)
		}
		return image
	}
	original := build()
	const hash = "3b70cab1f76a274072b0b82cc88dfa9cda37ffdd39fdc3ff4855b1b8794aa6d7"
	golden, err := os.ReadFile(filepath.Join(dir, "build", hash+".n1"))
	if err != nil {
		t.Fatal(err)
	}
	if len(original) != 4672 || !bytes.Equal(golden, original) || !bytes.Equal(original, build()) {
		t.Fatal("rebuild differs")
	}
	edit("CALL 6 2097 D100", "CALL 6 2097 D101")
	changed := build()
	// Task code starts at word 1114. The first call's MOVL destination is word 16.
	expected := bytes.Clone(original)
	expected[(1114+16)*4+3]++
	if !bytes.Equal(changed, expected) {
		t.Fatal("editing the result address changed unexpected bytes")
	}
	for _, tc := range []struct{ old, new, why string }{
		{"CALL 6", "CALL 7", "unknown instance"},
		{`"holding":544`, `"holding":539`, "overlaps"},
		{"CALL 5 2098 D200", "CALL 5 D1 D200", "expected CIO input"},
		{"OUT W0.02", "MOV D0 D1", "unexpected MOV"},
	} {
		edit(tc.old, tc.new)
		report, err := captureOmronBodyCommand(t, omronBuildProject, []string{dir})
		if err == nil || !strings.Contains(err.Error(), tc.why) || len(report) != 0 {
			t.Fatal("invalid source accepted or wrong error", tc, err)
		}
	}
	entries, err := os.ReadDir(filepath.Join(dir, "build"))
	if err != nil || len(entries) != 2 {
		t.Fatal("unexpected build artifacts", entries, err)
	}
}
