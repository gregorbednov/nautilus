package codec_test

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/il"
	"github.com/joyautomation/nautilus/omron/um"
)

type inputCase struct {
	ID       string   `json:"id"`
	Mnemonic string   `json:"mnemonic"`
	Operands []string `json:"operands"`
}
type nativeResult struct {
	Schema         int      `json:"schemaVersion"`
	Kind           string   `json:"kind"`
	Sequence       int      `json:"sequence"`
	ID             string   `json:"id"`
	Device         string   `json:"device"`
	CPU            string   `json:"cpu"`
	Mnemonic       string   `json:"mnemonic"`
	Operands       []string `json:"operands"`
	Accepted       bool     `json:"accepted"`
	Initialise     int      `json:"initialiseResult"`
	OperandResults []uint32 `json:"operandResults"`
	EncodeResult   *int     `json:"encodeResult"`
	Words          []uint32 `json:"words"`
	Selector       uint32   `json:"selector"`
	Low            uint32   `json:"familyMaskLow"`
	High           uint32   `json:"familyMaskHigh"`
	Compiler       string   `json:"compilerSHA256"`
	Request        string   `json:"requestSHA256"`
	Architecture   string   `json:"architecture"`
	ProgramType    *int     `json:"programType"`
	UnitVersion    *string  `json:"unitVersion"`
	TimerMode      string   `json:"timerCounterMode"`
	FullProgram    *bool    `json:"fullProgramCompiled"`
	CaseCount      int      `json:"caseCount"`
	ModeBCD        *int     `json:"timerModeBcd"`
	ModeBinary     *int     `json:"timerModeBinary"`
}
type oracleArtifact struct {
	Path     string `json:"path"`
	SHA256   string `json:"sha256"`
	Bytes    int    `json:"bytes"`
	Encoding string `json:"encoding"`
	Decoded  string `json:"decodedSHA256"`
}
type oracleManifest struct {
	Schema      int                       `json:"schemaVersion"`
	Method      string                    `json:"method"`
	Compiler    string                    `json:"compilerSHA256"`
	Request     string                    `json:"requestSHA256"`
	ProgramType int                       `json:"programType"`
	UnitVersion *string                   `json:"unitVersion"`
	TimerMode   string                    `json:"timerCounterMode"`
	FullProgram bool                      `json:"fullProgramCompiled"`
	Hardware    bool                      `json:"hardwareQualified"`
	Artifacts   map[string]oracleArtifact `json:"artifacts"`
}

func sha(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }
func loadOracleArtifact(t *testing.T, entry oracleArtifact) []byte {
	t.Helper()
	if entry.Path == "" || filepath.Base(entry.Path) != entry.Path || strings.ContainsAny(entry.Path, `/\:`) {
		t.Fatal("invalid fixture artifact path")
	}
	data, err := os.ReadFile(filepath.Join("testdata", entry.Path))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != entry.Bytes || sha(data) != entry.SHA256 {
		t.Fatal("artifact hash/size differs:", entry.Path)
	}
	if entry.Encoding == "gzip" {
		zip, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		defer zip.Close()
		data, err = io.ReadAll(io.LimitReader(zip, (16<<20)+1))
		if err != nil || len(data) > 16<<20 {
			t.Fatal("invalid compressed fixture", err)
		}
		if sha(data) != entry.Decoded {
			t.Fatal("decoded artifact hash differs:", entry.Path)
		}
	} else if entry.Encoding != "" {
		t.Fatal("unknown fixture encoding")
	}
	return data
}

func libraryResults(t *testing.T) []nativeResult {
	return loadLibraryResults(t, "library-manifest.json", 430)
}

func loadLibraryResults(t *testing.T, manifestName string, expected int) []nativeResult {
	t.Helper()
	metadata, err := os.ReadFile(filepath.Join("testdata", manifestName))
	if err != nil {
		t.Fatal(err)
	}
	var manifest oracleManifest
	if err := json.Unmarshal(metadata, &manifest); err != nil {
		t.Fatal(err)
	}
	const compiler = "a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141"
	if manifest.Schema != 1 || manifest.Method != "native-library-calls-in-disposable-worker" || manifest.Compiler != compiler || manifest.UnitVersion != nil || manifest.ProgramType != 0 || (manifest.TimerMode != "library-default" && manifest.TimerMode != "bcd" && manifest.TimerMode != "binary") || manifest.FullProgram || manifest.Hardware {
		t.Fatal("unsupported oracle provenance")
	}
	assets := map[string][]byte{}
	if len(manifest.Artifacts) != 5 {
		t.Fatal("missing or unexpected oracle artifacts")
	}
	for name, entry := range manifest.Artifacts {
		assets[name] = loadOracleArtifact(t, entry)
	}
	for _, name := range []string{"cases", "events", "script", "template", "worker"} {
		if len(assets[name]) == 0 {
			t.Fatal("missing oracle artifact:", name)
		}
	}
	if sha(assets["cases"]) != manifest.Request {
		t.Fatal("request hash mismatch")
	}
	var cases []inputCase
	if err := json.Unmarshal(assets["cases"], &cases); err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, test := range cases {
		if test.ID == "" || ids[test.ID] {
			t.Fatal("duplicate fixture case")
		}
		ids[test.ID] = true
	}
	// Verify the actual injected script contains precisely these input literals
	// and the archived template, rather than only trusting a declared hash.
	parts := bytes.SplitN(assets["script"], []byte("\n"), 2)
	if len(parts) != 2 || !bytes.Equal(parts[1], assets["template"]) {
		t.Fatal("script template mismatch")
	}
	requestText := strings.TrimSuffix(strings.TrimPrefix(string(parts[0]), "const request = "), ";")
	var request struct {
		Binary    string      `json:"binary"`
		Compiler  string      `json:"compilerSHA256"`
		Hash      string      `json:"requestSHA256"`
		Cases     []inputCase `json:"cases"`
		TimerMode string      `json:"timerCounterMode"`
	}
	if err := json.Unmarshal([]byte(requestText), &request); err != nil || request.Binary != `C:\Program Files (x86)\OMRON\CX-One\CX-Programmer\CX-Pdata.dll` || request.Compiler != compiler || request.Hash != manifest.Request || !reflect.DeepEqual(request.Cases, cases) {
		t.Fatal("script input mismatch", err)
	}
	if request.TimerMode != manifest.TimerMode && !(request.TimerMode == "" && manifest.TimerMode == "library-default") {
		t.Fatal("script timer mode mismatch")
	}
	rows := strings.Split(strings.TrimSpace(string(assets["events"])), "\n")
	results := []nativeResult{}
	profileIndex := -1
	caseIndex := 0
	done := false
	profiles := []struct {
		device, cpu string
		mask        uint32
	}{{"CJ1M", "CPU21", 1024}, {"CJ1H-H", "CPU65", 524288}}
	for sequence, line := range rows {
		if !strings.HasPrefix(line, "CJ1_INSTRUCTION ") {
			t.Fatal("unexpected library output")
		}
		var row nativeResult
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "CJ1_INSTRUCTION ")), &row); err != nil {
			t.Fatal(err)
		}
		if row.Schema != 1 || row.Sequence != sequence || done {
			t.Fatal("library sequence/lifecycle mismatch")
		}
		if sequence == 0 && row.Kind != "begin" {
			t.Fatal("missing library begin event")
		}
		switch row.Kind {
		case "begin":
			if sequence != 0 || row.Compiler != compiler || row.Request != manifest.Request || row.Architecture != "ia32" || row.ProgramType == nil || *row.ProgramType != 0 || row.UnitVersion != nil || row.TimerMode != manifest.TimerMode || row.FullProgram == nil || *row.FullProgram {
				t.Fatal("library configuration mismatch")
			}
		case "profile":
			if profileIndex >= 0 && caseIndex != len(cases) {
				t.Fatal("incomplete preceding profile")
			}
			profileIndex++
			caseIndex = 0
			if profileIndex >= len(profiles) {
				t.Fatal("unexpected profile")
			}
			want := profiles[profileIndex]
			if row.Device != want.device || row.CPU != want.cpu || row.Selector != 8 || row.Low != want.mask || row.High != 0 {
				t.Fatal("profile mismatch")
			}
			if manifest.TimerMode != "library-default" || row.ModeBCD != nil || row.ModeBinary != nil {
				bcd := 0
				if manifest.TimerMode == "bcd" {
					bcd = 1
				}
				if row.ModeBCD == nil || row.ModeBinary == nil || *row.ModeBCD != bcd || *row.ModeBinary != 1-bcd {
					t.Fatal("timer mode readback mismatch")
				}
			}
		case "case":
			if profileIndex < 0 || caseIndex >= len(cases) {
				t.Fatal("orphan case")
			}
			want := cases[caseIndex]
			profile := profiles[profileIndex]
			if row.ID != want.ID || row.Mnemonic != want.Mnemonic || !reflect.DeepEqual(row.Operands, want.Operands) || row.Device != profile.device || row.CPU != profile.cpu {
				t.Fatal("case input correlation mismatch")
			}
			if row.Accepted {
				if row.Initialise != 1 || row.EncodeResult == nil || *row.EncodeResult != 0 || len(row.Words) == 0 || len(row.OperandResults) != len(want.Operands) {
					t.Fatal("incomplete encoded result")
				}
				for _, code := range row.OperandResults {
					if code != 0 {
						t.Fatal("encoded invalid operand")
					}
				}
			} else if len(row.Words) != 0 {
				t.Fatal("rejected case contains machine code")
			}
			results = append(results, row)
			caseIndex++
		case "done":
			if profileIndex != 1 || caseIndex != len(cases) || row.CaseCount != 2*len(cases) {
				t.Fatal("incomplete library run")
			}
			done = true
		default:
			t.Fatal("library error/unknown event:", row.Kind)
		}
	}
	if !done || len(results) != expected {
		t.Fatal("library fixture coverage changed")
	}
	return results
}

func qualifiedForm(mnemonic string) bool {
	switch mnemonic {
	case "LD", "LDNOT", "AND", "ANDNOT", "OR", "ORNOT", "OUT", "OUTNOT", "SET", "RSET", "LD(000)", "END", "END(001)", "END(1)", "NOP", "NOP(000)", "MOV(021)":
		return true
	}
	return false
}

func TestCodecAgainstLibraryOracle(t *testing.T) {
	encoded, rejected, unsupported := 0, 0, 0
	for _, row := range libraryResults(t) {
		source := row.Mnemonic
		if len(row.Operands) > 0 {
			source += " " + strings.Join(row.Operands, " ")
		}
		instruction, err := il.Parse(source)
		if err != nil {
			t.Fatal(err)
		}
		profile := codec.Profile{Model: row.Device + " " + row.CPU}
		words, err := codec.EncodeInstruction(profile, instruction)
		if !qualifiedForm(row.Mnemonic) {
			if err == nil {
				t.Fatalf("unqualified form emitted code: %s", source)
			}
			unsupported++
			continue
		}
		if !row.Accepted {
			if err == nil {
				t.Fatalf("invalid native operand emitted code: %s", source)
			}
			rejected++
			continue
		}
		if err != nil || !reflect.DeepEqual(words, row.Words) {
			t.Fatalf("%s %s: got %x/%v, oracle %x", profile.Model, source, words, err, row.Words)
		}
		decoded, consumed, err := codec.DecodeInstruction(profile, row.Words)
		if err != nil || consumed != len(row.Words) {
			t.Fatalf("decode %s: %v", source, err)
		}
		roundTrip, err := codec.EncodeInstruction(profile, decoded)
		if err != nil || !reflect.DeepEqual(roundTrip, row.Words) {
			t.Fatal("decoded canonical source differs from oracle", err)
		}
		encoded++
	}
	if encoded < 300 || rejected < 30 || unsupported < 10 {
		t.Fatalf("insufficient corpus coverage: %d/%d/%d", encoded, rejected, unsupported)
	}
	t.Logf("oracle cases: encoded=%d rejected=%d outside current codec=%d", encoded, rejected, unsupported)
}

func TestCodecAnchoredToCompiledProjects(t *testing.T) {
	for _, file := range []struct{ name, first string }{{"cj1h-baseline.n1", "LD W0.00"}, {"cj1m-input-bit.n1", "LD W0.15"}} {
		data, err := os.ReadFile(filepath.Join("..", "um", "testdata", file.name))
		if err != nil {
			t.Fatal(err)
		}
		image, err := um.ParseN1(data)
		if err != nil {
			t.Fatal(err)
		}
		body := image.Tasks[0].Code
		for _, model := range []string{codec.CJ1MCPU21, codec.CJ1HCPU65} {
			for _, test := range []struct {
				source string
				word   uint32
			}{{file.first, body[0]}, {"OUT W0.01", body[1]}, {"END", body[len(body)-1]}} {
				instruction, err := il.Parse(test.source)
				if err != nil {
					t.Fatal(err)
				}
				words, err := codec.EncodeInstruction(codec.Profile{Model: model}, instruction)
				if err != nil || len(words) != 1 || words[0] != test.word {
					t.Fatalf("project anchor %s: %x/%v", test.source, words, err)
				}
			}
		}
	}
}
