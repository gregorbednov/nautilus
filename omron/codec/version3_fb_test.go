package codec_test

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
)

// All comparisons run from retained data. No DLL, Windows API or PLC is used.
func TestExplicitCurrent3FBSettingsAgainstQualifiedBaseline(t *testing.T) {
	f, err := os.Open("testdata/native-version3-fb.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var corpus struct {
		SchemaVersion                                                                                                   int
		Method, CompilerSHA256                                                                                          string
		VendorCodeExecuted, NetworkAccess, HardwareQualified, WriteQualified                                            bool
		ExplicitCodecUnitVersionQualified, BinaryFBQualified, Default3InitializationQualified, FullUserProjectsCompiled bool
		Runs                                                                                                            []struct {
			Group, LocalCurrentVersion string
			Request                    []struct {
				ID             string
				MemorySettings *codec.N1FBMemorySettings
			}
			Events []json.RawMessage
		}
	}
	if err = json.NewDecoder(io.LimitReader(z, 8<<20)).Decode(&corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.SchemaVersion != 1 || corpus.Method != "explicit-local-current-fb-settings-oracle-v1" || corpus.CompilerSHA256 != "a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141" || !corpus.VendorCodeExecuted || corpus.NetworkAccess || corpus.HardwareQualified || corpus.WriteQualified || corpus.ExplicitCodecUnitVersionQualified || corpus.BinaryFBQualified || corpus.Default3InitializationQualified || corpus.FullUserProjectsCompiled || len(corpus.Runs) != 20 {
		t.Fatal("corpus scope")
	}
	specs := map[string]struct {
		probe string
		count int
	}{
		"br-compile": {"fb-instruction-compile-v1", 292}, "definition-wrapper": {"fb-instruction-compile-v1", 38},
		"definition-properties": {"fb-definition-properties-v1", 16}, "instance-compile": {"fb-instance-compile-v1", 110},
		"invocation": {"invocation-skeleton-v1", 4}, "parameter": {"parameter-transfer-v1", 176},
		"section-addresses": {"fb-section-addresses-v1", 74}, "header-memory": {"program-object-v1", 12},
		"fb-memory-file": {"program-object-v1", 12}, "header-options-bcd": {"program-object-v1", 32},
	}
	seenRuns := map[string]bool{}
	rows, images, definitions, rejected := 0, 0, 0, 0
	for _, run := range corpus.Runs {
		spec, ok := specs[run.Group]
		key := run.Group + "/" + run.LocalCurrentVersion
		if !ok || seenRuns[key] || (run.LocalCurrentVersion != "3.0" && run.LocalCurrentVersion != "4.0") {
			t.Fatal("run identity")
		}
		seenRuns[key] = true
		baseline := map[string]nativeResult{}
		for _, row := range loadLibraryProbeResults(t, run.Group+"-manifest.json", spec.count, spec.probe) {
			baseline[row.Device+"/"+row.ID] = row
		}
		inputs := map[string]*codec.N1FBMemorySettings{}
		for _, request := range run.Request {
			inputs[request.ID] = request.MemorySettings
		}
		seen := map[string]bool{}
		for _, raw := range run.Events {
			var row nativeResult
			if err = json.Unmarshal(raw, &row); err != nil {
				t.Fatal(err)
			}
			if row.Kind != "case" {
				continue
			}
			var context struct{ LocalCurrentVersionBytes []int }
			if err = json.Unmarshal(raw, &context); err != nil {
				t.Fatal(err)
			}
			major := 3
			if run.LocalCurrentVersion == "4.0" {
				major = 4
			}
			if !reflect.DeepEqual(context.LocalCurrentVersionBytes, []int{major, 0, 0}) {
				t.Fatal("current version drift")
			}
			id := row.Device + "/" + row.ID
			want, ok := baseline[id]
			if !ok || seen[id] {
				t.Fatal("case identity")
			}
			seen[id] = true
			// Sequence numbers shift when the explicit-context event is added;
			// the owned output directory is deliberately new for every run.
			row.Sequence, want.Sequence = 0, 0
			if row.ProgramObject != nil {
				row.ProgramObject.File.Path = ""
				want.ProgramObject.File.Path = ""
			}
			if !reflect.DeepEqual(row, want) {
				t.Fatalf("%s %s differs from qualified baseline", key, id)
			}
			rows++
			if !row.Accepted {
				rejected++
			}
			if row.ProgramObject != nil {
				if memory := inputs[row.ID]; memory != nil {
					checkMemoryReadback(t, *memory, row.ProgramObject.Memory)
				}
				options := codec.N1TaskOptions{ExtendedTIMHold: true}
				if row.ProgramObject.HeaderOptions != nil {
					options = *row.ProgramObject.HeaderOptions
				}
				model := codec.CJ1MCPU21
				if row.Device == "CJ1H-H" {
					model = codec.CJ1HCPU65
				}
				flags, err := codec.EncodeN1TaskFlags(codec.Profile{Model: model, TimerCounterMode: codec.TimerModeBCD}, options)
				if err != nil {
					t.Fatal(err)
				}
				// Independently reconstruct every word, not just compare two DLL runs.
				checkOwnedProgramImage(t, row, codec.TimerModeBCD, flags)
				images++
				if row.ProgramObject.Definition != nil {
					definitions++
				}
			}
		}
		if len(seen) != spec.count {
			t.Fatal("incomplete run")
		}
	}
	if rows != 1532 || images != 112 || definitions != 24 || rejected != 100 {
		t.Fatalf("coverage %d/%d/%d/rejected%d", rows, images, definitions, rejected)
	}
	// Neither equal native output nor a positive FB predicate qualifies the
	// complete explicit production profile or binary FB descriptor.
	for _, model := range []string{codec.CJ1MCPU21, codec.CJ1HCPU65} {
		if _, err := codec.EncodeN1TaskFlags(codec.Profile{Model: model, TimerCounterMode: codec.TimerModeBCD, UnitVersion: "3.0"}, codec.N1TaskOptions{}); err == nil {
			t.Fatal("explicit full-profile barrier removed")
		}
	}
}
