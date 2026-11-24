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

func TestProgramCheckVersionGuardAndCompleteFiles(t *testing.T) {
	f, err := os.Open("testdata/native-program-guard-version.json.gz")
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
		SchemaVersion                                                                       int
		Method, CompilerSHA256                                                              string
		VendorCodeExecuted, NetworkAccess, HardwareQualified, WriteQualified                bool
		ExplicitCodecUnitVersionQualified, FullReferencedFBQualified, CurrentBranchExecuted bool
		Runs                                                                                []struct {
			LocalCurrentVersion, ProgramCheckVersion string
			Events                                   []json.RawMessage
		}
	}
	if err = json.NewDecoder(io.LimitReader(z, 8<<20)).Decode(&corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.SchemaVersion != 1 || corpus.Method != "program-check-version-guard-observer-v1" || corpus.CompilerSHA256 != "a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141" || !corpus.VendorCodeExecuted || corpus.NetworkAccess || corpus.HardwareQualified || corpus.WriteQualified || corpus.ExplicitCodecUnitVersionQualified || corpus.FullReferencedFBQualified || corpus.CurrentBranchExecuted || len(corpus.Runs) != 6 {
		t.Fatal("corpus scope")
	}
	seen := map[string]bool{}
	images, registrations, compiles := 0, 0, 0
	fileHex := map[string]string{}
	for _, run := range corpus.Runs {
		key := run.LocalCurrentVersion + "/" + run.ProgramCheckVersion
		if seen[key] || (run.LocalCurrentVersion != "3.0" && run.LocalCurrentVersion != "4.0") {
			t.Fatal("run identity")
		}
		seen[key] = true
		major := 3
		if run.LocalCurrentVersion == "4.0" {
			major = 4
		}
		current := []int{major, 0, 0}
		guard := []int{255, 255}
		if run.ProgramCheckVersion == "3.0" {
			guard = []int{3, 0}
		} else if run.ProgramCheckVersion == "4.0" {
			guard = []int{4, 0}
		} else if run.ProgramCheckVersion != "constructor-default" {
			t.Fatal("program version")
		}
		for _, raw := range run.Events {
			var event struct {
				Kind, Role                                                                                                                                                                        string
				InitialVersionBytes, SelectedVersionBytes, LocalCurrentVersionBytes, ProgramCheckVersionBytes, SelectedGuardVersionBytes, AfterCurrentVersionBytes, AfterProgramCheckVersionBytes []int
				UseCurrentVersion, AfterPackedGuardVersion, Returned                                                                                                                              int
				SetterUsed                                                                                                                                                                        bool
			}
			if err = json.Unmarshal(raw, &event); err != nil {
				t.Fatal(err)
			}
			switch event.Kind {
			case "program-check-version":
				if !reflect.DeepEqual(event.InitialVersionBytes, []int{255, 255}) || !reflect.DeepEqual(event.SelectedVersionBytes, guard) || !reflect.DeepEqual(event.LocalCurrentVersionBytes, current) || event.SetterUsed != (run.ProgramCheckVersion != "constructor-default") {
					t.Fatal("constructor/setter state")
				}
				registrations++
			case "program-compile-version":
				if event.UseCurrentVersion != 0 || !reflect.DeepEqual(event.LocalCurrentVersionBytes, current) || !reflect.DeepEqual(event.AfterCurrentVersionBytes, current) || !reflect.DeepEqual(event.ProgramCheckVersionBytes, guard) || !reflect.DeepEqual(event.SelectedGuardVersionBytes, guard) || !reflect.DeepEqual(event.AfterProgramCheckVersionBytes, guard) || event.AfterPackedGuardVersion != guard[0]<<8|guard[1] {
					t.Fatal("guard branch/version drift")
				}
				wantReturn := 1
				if event.Role == "definition" {
					wantReturn = 0
				} else if event.Role != "task" {
					t.Fatal("role")
				}
				if event.Returned != wantReturn {
					t.Fatal("compile result")
				}
				compiles++
			case "case":
				var row nativeResult
				if err = json.Unmarshal(raw, &row); err != nil {
					t.Fatal(err)
				}
				if !row.Accepted || !reflect.DeepEqual(event.LocalCurrentVersionBytes, current) || row.ProgramObject == nil || row.ProgramObject.File == nil {
					t.Fatal("complete case")
				}
				identity := row.Device + "/" + row.ID
				if prior, ok := fileHex[identity]; ok && prior != row.ProgramObject.File.Hex {
					t.Fatal("file differs between guard/current settings")
				}
				fileHex[identity] = row.ProgramObject.File.Hex
				if row.ID == "no-fb-bit" {
					// The existing whole-file helper's direct recipe is exactly
					// this owned LD W0.00 / OUT W0.01 / END source.
					row.ID = "direct-program-guard"
				} else if row.ID != "fb-default" {
					t.Fatal("unknown source")
				}
				checkOwnedProgramImage(t, row, codec.TimerModeBCD, 0)
				images++
			}
		}
	}
	if images != 24 || registrations != 36 || compiles != 60 || len(fileHex) != 4 {
		t.Fatalf("coverage %d/%d/%d", images, registrations, compiles)
	}
}
