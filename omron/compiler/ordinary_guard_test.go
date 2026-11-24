package compiler_test

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
)

// Compare complete task bodies, including the input/output context in
// which the native program checker accepted or rejected each instruction.
func TestNativeOrdinaryProgramGuards(t *testing.T) {
	f, err := os.Open("testdata/native-ordinary-guards.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var matrix struct {
		Runs []struct {
			Mode, Version, Context string
			Events                 []struct {
				Kind, Device, ID                           string
				Accepted                                   bool
				SelectedGuardVersionBytes                  []int
				AfterPackedGuardVersion, UseCurrentVersion int
				OrdinaryGuard                              struct {
					Lines [][]json.RawMessage
					Words []uint32
				}
			}
		}
	}
	if err := json.NewDecoder(z).Decode(&matrix); err != nil {
		t.Fatal(err)
	}
	seen := map[[5]string]bool{}
	for _, run := range matrix.Runs {
		if len(run.Events)%2 != 0 {
			t.Fatal("incomplete trace")
		}
		for i := 0; i < len(run.Events); i += 2 {
			guard, c := run.Events[i], run.Events[i+1]
			major := 3
			if run.Version == "4.0" {
				major = 4
			} else if run.Version != "3.0" {
				t.Fatal("unknown version")
			}
			if guard.Kind != "program-compile-version" || c.Kind != "case" || guard.ID != c.ID || guard.Device != c.Device || guard.UseCurrentVersion != 0 || !slices.Equal(guard.SelectedGuardVersionBytes, []int{major, 0}) || guard.AfterPackedGuardVersion != major<<8 {
				t.Fatal("missing native program guard")
			}
			key := [5]string{run.Mode, run.Version, c.Device, c.ID, run.Context}
			if seen[key] {
				t.Fatal("duplicate", key)
			}
			seen[key] = true
			r := compiler.Rung{Path: c.ID}
			for _, line := range c.OrdinaryGuard.Lines {
				var op string
				var args []string
				if len(line) != 2 || json.Unmarshal(line[0], &op) != nil || json.Unmarshal(line[1], &args) != nil {
					t.Fatal("invalid instruction", key)
				}
				r.Lines = append(r.Lines, compiler.Line{Text: strings.Join(append([]string{op}, args...), " ")})
			}
			model := map[string]string{"CJ1M": codec.CJ1MCPU21, "CJ1H-H": codec.CJ1HCPU65}[c.Device]
			if model == "" {
				t.Fatal("unknown model")
			}
			body, err := compiler.CompileRungs(codec.Profile{Model: model, TimerCounterMode: codec.TimerCounterMode(run.Mode)}, []compiler.Rung{r})
			want := c.OrdinaryGuard.Words
			if c.ID == "form-395" && run.Context == "baseline" {
				// Raw " T0" was passed as one operand directly to the vendor
				// library. The text parser tokenizes it as T0. The canonical
				// control in the same corpus tests that spelling natively.
				if len(want) != 5 || want[2] != 0x60000000 {
					t.Fatal("raw padded operand changed", key)
				}
				want = slices.Clone(want)
				want[2] = 0x6000e000
			}
			if (err == nil) != c.Accepted || (err == nil && !slices.Equal(body.Words, want)) {
				t.Errorf("guard mismatch %v: native accepted=%t, Go error=%v", key, c.Accepted, err)
			}
		}
	}
	if len(seen) != 5888 {
		t.Fatal("incomplete matrix", len(seen))
	}
}
