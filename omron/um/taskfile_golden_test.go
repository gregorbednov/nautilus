package um

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

// These files were emitted separately by the pinned native compiler, rather
// than constructed by slicing the full-image fixtures in this test.
func TestExtractN1TaskRecordNativeFiles(t *testing.T) {
	f, err := os.Open("testdata/native-task-file-cases.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	var fixture struct {
		CompilerSHA256                                                       string
		VendorCodeExecuted, NetworkAccess, HardwareQualified, WriteQualified bool
		Runs                                                                 []struct {
			Cases []struct {
				ID, Device, CPU string
				ProgramObject   struct {
					CompileResult int
					Words         []uint32
					File          struct {
						Returned, ErrorCount, TaskSelector, ByteLength int
						FileHex, SHA256                                string
					}
				}
			}
		}
	}
	if err := json.NewDecoder(z).Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	if fixture.CompilerSHA256 != "a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141" || !fixture.VendorCodeExecuted || fixture.NetworkAccess || fixture.HardwareQualified || fixture.WriteQualified {
		t.Fatal("unexpected evidence provenance")
	}
	full := make(map[string][]byte)
	tasks := make(map[string][]byte)
	absent := make(map[string]bool)
	count := 0
	for _, run := range fixture.Runs {
		for _, c := range run.Cases {
			count++
			file := c.ProgramObject.File
			b, err := hex.DecodeString(file.FileHex)
			if err != nil {
				t.Fatal(err)
			}
			if c.ProgramObject.CompileResult != 1 || file.Returned != 1 || file.ErrorCount != 0 || len(b) != file.ByteLength || hex.EncodeToString(sha256Sum(b)) != file.SHA256 {
				t.Fatalf("invalid native result %s/%s", c.Device, c.ID)
			}
			model := c.Device + "/" + c.CPU
			if model != "CJ1M/CPU21" && model != "CJ1H-H/CPU65" {
				t.Fatalf("unexpected profile %s", model)
			}
			if c.ID == "absent-task" {
				if file.TaskSelector != 257 || len(b) != 0 || absent[model] {
					t.Fatal("absent-task control changed")
				}
				absent[model] = true
				continue
			}
			stem, kind, ok := strings.Cut(c.ID, "-")
			if !ok || (stem != "end" && stem != "bit") {
				t.Fatal("unknown native recipe")
			}
			key := model + "/" + stem
			switch kind {
			case "full":
				if file.TaskSelector != 65535 || full[key] != nil {
					t.Fatal("unexpected full selector or duplicate")
				}
				image, err := ParseN1(b)
				if err != nil {
					t.Fatal(err)
				}
				if len(image.Tasks) != 1 || image.Tasks[0].Number != 256 || image.Tasks[0].NameASCII != "Probe" {
					t.Fatal("unexpected native task")
				}
				want := []uint32{0xbb800001}
				if stem == "bit" {
					want = []uint32{0xe205e000, 0xfa05e001, 0xbb800001}
				}
				got := image.Tasks[0].Code
				if len(got) != len(want) {
					t.Fatal("unexpected native instruction count")
				}
				for i := range want {
					if got[i] != want[i] {
						t.Fatal("unexpected native instruction")
					}
				}
				full[key] = b
			case "task":
				if file.TaskSelector != 256 || len(b) == 0 || tasks[key] != nil {
					t.Fatal("unexpected task selector or duplicate")
				}
				tasks[key] = b
			default:
				t.Fatal("unknown native output kind")
			}
		}
	}
	if count != 10 || len(full) != 4 || len(tasks) != 4 || len(absent) != 2 {
		t.Fatal("incomplete native corpus")
	}
	for key, source := range full {
		t.Run(key, func(t *testing.T) {
			got, err := ExtractN1TaskRecord(source, 256)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, tasks[key]) {
				t.Fatal("extraction differs from separately emitted native task file")
			}
			if _, err := ParseN1(tasks[key]); !errors.Is(err, ErrN1Layout) {
				t.Fatal("standalone record accepted as full image")
			}
			if _, err := ExtractN1TaskRecord(source, 257); !errors.Is(err, ErrN1TaskNotFound) {
				t.Fatal("missing task must fail despite native writer success with empty output")
			}
		})
	}
	if _, err := ParseN1(nil); !errors.Is(err, ErrN1Layout) {
		t.Fatal("empty native output accepted")
	}
}

func sha256Sum(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}
