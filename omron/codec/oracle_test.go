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
	TaskConfiguration    *taskConfigurationInput    `json:"taskConfiguration,omitempty"`
	TaskProperties       *taskPropertiesInput       `json:"taskProperties,omitempty"`
	NextEnable           *nextEnableInput           `json:"nextEnable,omitempty"`
	DefinitionProperties *definitionPropertiesInput `json:"definitionProperties,omitempty"`
	Code                 []uint32                   `json:"code,omitempty"`
	SourceSHA256         string                     `json:"sourceSHA256,omitempty"`
	SourceOffsetWords    int                        `json:"sourceOffsetWords,omitempty"`
	Mutation             *decodeMutation            `json:"mutation,omitempty"`
	Parameter            *parameterInput            `json:"parameter,omitempty"`
	InvocationOption     *bool                      `json:"invocationOption,omitempty"`
	ID                   string                     `json:"id"`
	Mnemonic             string                     `json:"mnemonic"`
	Operands             []string                   `json:"operands"`
}
type nativeResult struct {
	TaskConfiguration  *taskConfigurationResult    `json:"taskConfiguration"`
	TaskProperties     *taskPropertiesResult       `json:"taskProperties"`
	NextEnable         *nextEnableResult           `json:"nextEnable"`
	InstanceStages     []controlStage              `json:"instanceStages"`
	Properties         *definitionPropertiesResult `json:"properties"`
	CompileResult      *int                        `json:"compileResult"`
	CompiledFlag       *int                        `json:"compiledFlag"`
	UseFunctionBlocks  *int                        `json:"useFunctionBlocks"`
	Decoded            []decodedInstruction        `json:"decoded"`
	SourceSHA256       string                      `json:"sourceSHA256"`
	SourceOffsetWords  int                         `json:"sourceOffsetWords"`
	ObjectSize         int                         `json:"objectSize"`
	Bases              []nativeBase                `json:"bases"`
	Parameter          *parameterResult            `json:"parameter"`
	Invocation         *invocationResult           `json:"invocation"`
	Probe              string                      `json:"probe"`
	FlagsBeforeControl uint32                      `json:"flagsBeforeControl"`
	ControlStages      []controlStage              `json:"controlStages"`
	Schema             int                         `json:"schemaVersion"`
	Kind               string                      `json:"kind"`
	Sequence           int                         `json:"sequence"`
	ID                 string                      `json:"id"`
	Device             string                      `json:"device"`
	CPU                string                      `json:"cpu"`
	Mnemonic           string                      `json:"mnemonic"`
	Operands           []string                    `json:"operands"`
	Accepted           bool                        `json:"accepted"`
	Initialise         int                         `json:"initialiseResult"`
	OperandResults     []uint32                    `json:"operandResults"`
	EncodeResult       *int                        `json:"encodeResult"`
	Words              []uint32                    `json:"words"`
	Selector           uint32                      `json:"selector"`
	Low                uint32                      `json:"familyMaskLow"`
	High               uint32                      `json:"familyMaskHigh"`
	Compiler           string                      `json:"compilerSHA256"`
	Request            string                      `json:"requestSHA256"`
	Architecture       string                      `json:"architecture"`
	ProgramType        *int                        `json:"programType"`
	UnitVersion        *string                     `json:"unitVersion"`
	TimerMode          string                      `json:"timerCounterMode"`
	FullProgram        *bool                       `json:"fullProgramCompiled"`
	CaseCount          int                         `json:"caseCount"`
	ModeBCD            *int                        `json:"timerModeBcd"`
	ModeBinary         *int                        `json:"timerModeBinary"`
}

type definitionPropertiesInput struct {
	Number      int    `json:"number"`
	Name        string `json:"name"`
	InputCount  int    `json:"inputCount"`
	OutputCount int    `json:"outputCount"`
}
type definitionPropertiesResult struct {
	Number        int    `json:"number"`
	Name          string `json:"name"`
	InputsResult  int    `json:"inputsResult"`
	OutputsResult int    `json:"outputsResult"`
	InputCount    int    `json:"inputCount"`
	OutputCount   int    `json:"outputCount"`
	Flags         uint32 `json:"flags"`
}

type decodeMutation struct {
	Operation string `json:"operation"`
	Value     uint32 `json:"value"`
	Indices   []int  `json:"indices"`
}

type decodedInstruction struct {
	Text        string   `json:"text"`
	OffsetWords int      `json:"offsetWords"`
	Words       []uint32 `json:"words"`
	Known       bool     `json:"known"`
}

type nativeBase struct {
	Index   int    `json:"index"`
	Name    string `json:"name"`
	Address int    `json:"address"`
	Size    int    `json:"size"`
}

type parameterInput struct {
	Type   string `json:"type"`
	Usage  int    `json:"usage"`
	Source string `json:"source"`
}

type parameterResult struct {
	Input        parameterInput          `json:"input"`
	DataType     uint32                  `json:"dataType"`
	SourceAfter  string                  `json:"sourceAfter"`
	Instructions []invocationInstruction `json:"instructions"`
}

type controlStage struct {
	Operation string   `json:"operation"`
	Words     []uint32 `json:"words"`
	Flags     uint32   `json:"flags"`
}

type invocationResult struct {
	Option         bool                    `json:"option"`
	GenerateResult int                     `json:"generateResult"`
	FinishResult   int                     `json:"finishResult"`
	Before         controlStage            `json:"before"`
	After          controlStage            `json:"after"`
	Preamble       []invocationInstruction `json:"preamble"`
	Epilogue       []invocationInstruction `json:"epilogue"`
}

type invocationInstruction struct {
	Mnemonic string   `json:"mnemonic"`
	Words    []uint32 `json:"words"`
	Flags    uint32   `json:"flags"`
}
type oracleArtifact struct {
	Path     string `json:"path"`
	SHA256   string `json:"sha256"`
	Bytes    int    `json:"bytes"`
	Encoding string `json:"encoding"`
	Decoded  string `json:"decodedSHA256"`
}
type oracleManifest struct {
	Probe       string                    `json:"probe"`
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
	return loadLibraryProbeResults(t, manifestName, expected, "")
}

func loadLibraryProbeResults(t *testing.T, manifestName string, expected int, probe string) []nativeResult {
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
	programType := 0
	decodeProbe := probe == "fb-native-decode-v1" || probe == "fb-instance-decode-v1"
	configurationProbe := probe == "task-configuration-v1"
	if probe == "fb-body-operands-v1" || probe == "fb-native-decode-v1" || probe == "fb-body-context-v1" || probe == "fb-instruction-compile-v1" || probe == "fb-definition-properties-v1" {
		programType = 32
	}
	if probe == "fb-instance-compile-v1" || probe == "fb-instance-decode-v1" {
		programType = 16
	}
	if manifest.Probe != probe || (probe != "" && probe != "control-bits-v1" && probe != "invocation-skeleton-v1" && probe != "parameter-transfer-v1" && probe != "fb-body-operands-v1" && !decodeProbe && !configurationProbe && probe != "fb-body-context-v1" && probe != "fb-instruction-compile-v1" && probe != "fb-definition-properties-v1" && probe != "fb-instance-compile-v1" && probe != "fb-section-addresses-v1" && probe != "task-properties-v1") || manifest.Schema != 1 || manifest.Method != "native-library-calls-in-disposable-worker" || manifest.Compiler != compiler || manifest.UnitVersion != nil || manifest.ProgramType != programType || (manifest.TimerMode != "library-default" && manifest.TimerMode != "bcd" && manifest.TimerMode != "binary") || manifest.FullProgram || manifest.Hardware {
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
		if !decodeProbe && (len(test.Code) != 0 || test.SourceSHA256 != "" || test.SourceOffsetWords != 0 || test.Mutation != nil) {
			t.Fatal("decode inputs presented as encode evidence")
		}
	}
	// Verify the actual injected script contains precisely these input literals
	// and the archived template, rather than only trusting a declared hash.
	parts := bytes.SplitN(assets["script"], []byte("\n"), 2)
	if len(parts) != 2 || !bytes.Equal(parts[1], assets["template"]) {
		t.Fatal("script template mismatch")
	}
	requestText := strings.TrimSuffix(strings.TrimPrefix(string(parts[0]), "const request = "), ";")
	var request struct {
		ProgramType *int        `json:"programType"`
		Probe       string      `json:"probe"`
		Binary      string      `json:"binary"`
		Compiler    string      `json:"compilerSHA256"`
		Hash        string      `json:"requestSHA256"`
		Cases       []inputCase `json:"cases"`
		TimerMode   string      `json:"timerCounterMode"`
	}
	if err := json.Unmarshal([]byte(requestText), &request); err != nil || request.Binary != `C:\Program Files (x86)\OMRON\CX-One\CX-Programmer\CX-Pdata.dll` || request.Compiler != compiler || request.Hash != manifest.Request || !reflect.DeepEqual(request.Cases, cases) {
		t.Fatal("script input mismatch", err)
	}
	if request.TimerMode != manifest.TimerMode && !(request.TimerMode == "" && manifest.TimerMode == "library-default") {
		t.Fatal("script timer mode mismatch")
	}
	if request.Probe != probe {
		t.Fatal("script probe mode mismatch")
	}
	if (request.ProgramType == nil && programType != 0) || (request.ProgramType != nil && *request.ProgramType != programType) {
		t.Fatal("script program type mismatch")
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
			if (decodeProbe && row.ObjectSize != 752) || (!decodeProbe && row.ObjectSize != 0) {
				t.Fatal("native object size mismatch")
			}
			if row.Probe != probe {
				t.Fatal("library probe mode mismatch")
			}
			if sequence != 0 || row.Compiler != compiler || row.Request != manifest.Request || row.Architecture != "ia32" || row.ProgramType == nil || *row.ProgramType != programType || row.UnitVersion != nil || row.TimerMode != manifest.TimerMode || row.FullProgram == nil || *row.FullProgram {
				t.Fatal("library configuration mismatch")
			}
		case "profile":
			if probe == "fb-body-context-v1" || probe == "fb-instruction-compile-v1" || probe == "fb-definition-properties-v1" || probe == "fb-instance-compile-v1" {
				if row.UseFunctionBlocks == nil || *row.UseFunctionBlocks != 1 {
					t.Fatal("FB mode readback mismatch")
				}
			} else if row.UseFunctionBlocks != nil {
				t.Fatal("unexpected FB mode readback")
			}
			if probe == "fb-body-operands-v1" || probe == "fb-body-context-v1" || probe == "fb-instruction-compile-v1" || probe == "fb-definition-properties-v1" || probe == "fb-instance-compile-v1" {
				wantBases := []nativeBase{{0, "T3072", 60416, 1024}, {1, "C3072", 64512, 1024}, {2, "DIFF8191", 40959, 0}, {3, "H512", 55808, 896}, {4, "H1408", 56704, 128}}
				if !reflect.DeepEqual(row.Bases, wantBases) {
					t.Fatal("base register readback differs", row.Bases)
				}
			} else if len(row.Bases) != 0 {
				t.Fatal("unexpected base register readback")
			}
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
			if !decodeProbe && (len(row.Decoded) != 0 || row.SourceSHA256 != "" || row.SourceOffsetWords != 0) {
				t.Fatal("decode results presented as compile evidence")
			}
			if probe != "fb-instruction-compile-v1" && probe != "fb-definition-properties-v1" && probe != "fb-instance-compile-v1" && (row.CompileResult != nil || row.CompiledFlag != nil) {
				t.Fatal("instruction compile presented as encode-only evidence")
			}
			if probe != "control-bits-v1" && len(row.ControlStages) != 0 {
				t.Fatal("unexpected control probe data")
			}
			if probe == "" && (len(row.ControlStages) != 0 || row.Invocation != nil || row.Parameter != nil) {
				t.Fatal("control probe presented as encode-only evidence")
			}
			if profileIndex < 0 || caseIndex >= len(cases) {
				t.Fatal("orphan case")
			}
			want := cases[caseIndex]
			if configurationProbe {
				p, r := want.TaskConfiguration, row.TaskConfiguration
				if p == nil || r == nil || r.Number != p.Number || r.Startup != p.Startup&1 || r.Protection != p.Protection || r.StartupResult != 1 || r.ProtectionResult != 1 || row.Mnemonic != "NOP" || len(row.Operands) != 0 {
					t.Fatal("task configuration request/readback mismatch")
				}
			} else if want.TaskConfiguration != nil || row.TaskConfiguration != nil {
				t.Fatal("unexpected task configuration")
			}
			if probe == "task-properties-v1" {
				if want.TaskProperties == nil || row.TaskProperties == nil || row.Mnemonic != "NOP" || len(row.Operands) != 0 {
					t.Fatal("missing task properties")
				}
				p, r := want.TaskProperties, row.TaskProperties
				number, returned := 256, 0
				if p.Number >= -1 && p.Number <= 289 {
					number, returned = p.Number, 1
				}
				protection, protectionResult := p.Protection, 1
				if number < 0 {
					protection, protectionResult = 0, 0
				}
				if r.Number != number || r.NumberResult != returned || r.Name != p.Name || r.Protection != protection || r.ProtectionResult != protectionResult {
					t.Fatal("task setter input/readback correlation mismatch")
				}
			} else if want.TaskProperties != nil || row.TaskProperties != nil {
				t.Fatal("unexpected task properties")
			}
			if probe == "fb-section-addresses-v1" {
				if want.NextEnable == nil || row.NextEnable == nil || row.NextEnable.Index != want.NextEnable.Index || row.NextEnable.Value != want.NextEnable.Value || row.Mnemonic != "NOP" || len(row.Operands) != 0 {
					t.Fatal("next-enable request/result mismatch")
				}
			} else if want.NextEnable != nil || row.NextEnable != nil {
				t.Fatal("unexpected next-enable observation")
			}
			if probe != "fb-instance-compile-v1" && len(row.InstanceStages) != 0 {
				t.Fatal("unexpected instance postamble stages")
			}
			if probe == "fb-instance-compile-v1" && row.Accepted {
				expected := 1
				if row.Mnemonic == "FBBCALL" {
					expected = 2
				}
				if len(row.InstanceStages) != expected || row.InstanceStages[0].Operation != "SetCIIFlag(true)" {
					t.Fatal("missing instance CII stages")
				}
				stage := append([]uint32(nil), row.Words...)
				stage[0] |= 0x20000000
				if !reflect.DeepEqual(row.InstanceStages[0].Words, stage) {
					t.Fatal("instance CII mutation differs")
				}
				if expected == 2 {
					stage[0] |= 0x023e0000
					if row.InstanceStages[1].Operation != "SetCCFlags" || !reflect.DeepEqual(row.InstanceStages[1].Words, stage) {
						t.Fatal("instance call CC mutation differs")
					}
				}
			}
			if probe == "fb-definition-properties-v1" {
				if want.DefinitionProperties == nil || row.Properties == nil {
					t.Fatal("missing definition properties")
				}
				p, r := want.DefinitionProperties, row.Properties
				if p.Number != r.Number || p.Name != r.Name || r.Flags != 0 {
					t.Fatal("definition identity/flags mismatch")
				}
				for _, pair := range [][3]int{{p.InputCount, r.InputsResult, r.InputCount}, {p.OutputCount, r.OutputsResult, r.OutputCount}} {
					if pair[0] < 66 {
						if pair[1] != 1 || pair[2] != pair[0] {
							t.Fatal("accepted count readback differs")
						}
					} else if pair[1] != 0 || pair[2] != 0 {
						t.Fatal("rejected count changed default")
					}
				}
			} else if want.DefinitionProperties != nil || row.Properties != nil {
				t.Fatal("unexpected definition properties")
			}
			if probe == "parameter-transfer-v1" {
				if want.Parameter == nil || row.Parameter == nil || !reflect.DeepEqual(row.Parameter.Input, *want.Parameter) {
					t.Fatal("parameter input correlation mismatch")
				}
			} else if row.Parameter != nil || want.Parameter != nil {
				t.Fatal("unexpected parameter probe data")
			}
			if probe == "invocation-skeleton-v1" {
				if want.InvocationOption == nil || row.Invocation == nil || row.Invocation.Option != *want.InvocationOption {
					t.Fatal("invocation option correlation mismatch")
				}
			} else if row.Invocation != nil || want.InvocationOption != nil {
				t.Fatal("unexpected invocation probe data")
			}
			profile := profiles[profileIndex]
			if row.ID != want.ID || row.Mnemonic != want.Mnemonic || !reflect.DeepEqual(row.Operands, want.Operands) || row.Device != profile.device || row.CPU != profile.cpu {
				t.Fatal("case input correlation mismatch")
			}
			if decodeProbe {
				if !row.Accepted || row.Initialise != 0 || row.EncodeResult != nil || len(row.OperandResults) != 0 || row.SourceSHA256 != want.SourceSHA256 || row.SourceOffsetWords != want.SourceOffsetWords || !reflect.DeepEqual(row.Words, want.Code) || len(row.Decoded) == 0 {
					t.Fatal("incomplete or changed native decode result")
				}
				offset := 0
				for _, instruction := range row.Decoded {
					if !instruction.Known || instruction.OffsetWords != offset || len(instruction.Words) == 0 || len(instruction.Words) > len(want.Code)-offset || !reflect.DeepEqual(instruction.Words, want.Code[offset:offset+len(instruction.Words)]) {
						t.Fatal("invalid native decoded span")
					}
					offset += len(instruction.Words)
				}
				if offset != len(want.Code) {
					t.Fatal("incomplete decoded word coverage")
				}
			} else if probe == "fb-instruction-compile-v1" || probe == "fb-definition-properties-v1" || probe == "fb-instance-compile-v1" {
				if row.CompileResult == nil || row.CompiledFlag == nil || row.EncodeResult != nil || row.Initialise != 1 || len(row.OperandResults) != len(want.Operands) {
					t.Fatal("incomplete instruction compile result")
				}
				if row.Accepted {
					if *row.CompileResult != 1 || *row.CompiledFlag != 1 || len(row.Words) == 0 {
						t.Fatal("failed native compile admitted")
					}
					for _, code := range row.OperandResults {
						if code != 0 {
							t.Fatal("compiled invalid operand")
						}
					}
				} else if *row.CompileResult != 0 || *row.CompiledFlag != 0 || len(row.Words) != 0 {
					t.Fatal("inconsistent rejected compile")
				}
			} else if len(row.Decoded) != 0 || row.SourceSHA256 != "" {
				t.Fatal("unexpected decode results")
			} else if row.Accepted {
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
	case "LD", "LDNOT", "AND", "ANDNOT", "OR", "ORNOT", "OUT", "OUTNOT", "SET", "RSET", "LD(000)", "END", "END(001)", "END(1)", "NOP", "NOP(000)", "MOV(021)", "@LD", "%LD", "@SET", "%SET":
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
		dmBitExcluded := instruction.Mnemonic != "MOV" && len(instruction.Operands) == 1 && strings.HasPrefix(strings.ToUpper(instruction.Operands[0]), "D")
		if !qualifiedForm(row.Mnemonic) || dmBitExcluded {
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
	if encoded < 250 || rejected < 30 || unsupported < 10 {
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
