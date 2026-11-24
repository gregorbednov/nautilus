package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/compiler"
)

func TestEditableFloatDefinition(t *testing.T) {
	compile := func(src string) []uint32 {
		t.Helper()
		record, err := compiler.CompileSTFloatDefinition(profile, src, 0, 1024)
		if err != nil {
			t.Fatal(err)
		}
		return record.RecordWords
	}
	original := compile(temperatureSource)
	changed := compile(strings.Replace(temperatureSource, "90.0", "91.0", 1))
	if len(original) != len(changed) {
		t.Fatal("literal changed length")
	}
	diffs := 0
	for i, w := range original {
		if w != changed[i] {
			diffs++
			if w != 0x614042b4 || changed[i] != 0x614042b6 {
				t.Fatalf("unexpected difference at %d", i)
			}
		}
	}
	if diffs != 1 {
		t.Fatal("expected one immediate word to change")
	}
	renamed := strings.ReplaceAll(temperatureSource, "Converted", "rawAsReal")
	if !slices.Equal(original, compile(renamed)) {
		t.Fatal("renaming a local changed the compiled record")
	}
	for _, bad := range []string{
		strings.Replace(temperatureSource, "Scaled - 4.0", "Scaled - Converted", 1),
		strings.Replace(temperatureSource, "Scaled := 4.0", "Raw := 4.0", 1),
		strings.Replace(temperatureSource, "Scaled : REAL", "Scaled : INT", 1),
		strings.Replace(temperatureSource, "INT_TO_REAL(Raw)", "INT_TO_REAL(Missing)", 1),
		temperatureSource + "\nPROGRAM main\nEND_PROGRAM",
	} {
		if r, err := compiler.CompileSTFloatDefinition(profile, bad, 0, 1024); err == nil || r != nil {
			t.Fatal("accepted unsupported source")
		}
	}
}
