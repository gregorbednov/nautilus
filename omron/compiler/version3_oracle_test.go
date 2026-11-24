package compiler_test

import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/il"
)

type versionCompileEvent struct {
	Kind, ID, Device, CPU, Mnemonic, UnitVersion, UnitVersionSource string
	Operands                                                        []string
	Words                                                           []uint32
	Accepted                                                        bool
	LocalCurrentVersionBytes                                        []int
	ProgramObject                                                   struct {
		CompileResult int
		File          struct {
			TaskSelector, Returned, ErrorCount, ByteLength int
			FileHex                                        string
		}
	}
}

func TestExplicitLocalVersionInstructionAndImageOracle(t *testing.T) {
	f, err := os.Open("testdata/native-version3-compile.json.gz")
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
		SchemaVersion                                                                  int
		Method, CompilerSHA256                                                         string
		VendorCodeExecuted, NetworkAccess, HardwareQualified, WriteQualified           bool
		ExplicitCodecUnitVersionQualified, FullUserProjectsCompiled, BinaryFBQualified bool
		Runs                                                                           []struct {
			Kind, TimerCounterMode, LocalCurrentVersion string
			Events                                      []versionCompileEvent
		}
	}
	if err = json.NewDecoder(io.LimitReader(z, 8<<20)).Decode(&corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.SchemaVersion != 1 || corpus.Method != "explicit-local-current-instruction-image-oracle-v1" || corpus.CompilerSHA256 != "a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141" || !corpus.VendorCodeExecuted || corpus.NetworkAccess || corpus.HardwareQualified || corpus.WriteQualified || corpus.ExplicitCodecUnitVersionQualified || corpus.FullUserProjectsCompiled || corpus.BinaryFBQualified || len(corpus.Runs) != 12 {
		t.Fatal("invalid corpus scope")
	}
	settingsRaw, err := os.ReadFile("testdata/n1-finalization-probe-settings.json")
	if err != nil {
		t.Fatal(err)
	}
	var settings compiler.N1ImageSettings
	if err = json.Unmarshal(settingsRaw, &settings); err != nil {
		t.Fatal(err)
	}
	instructions, images := 0, 0
	seenImages := map[string][]byte{}
	for _, run := range corpus.Runs {
		mode := codec.TimerCounterMode(run.TimerCounterMode)
		if mode != codec.TimerModeBCD && mode != codec.TimerModeBinary {
			t.Fatal("timer mode")
		}
		major := 3
		if run.LocalCurrentVersion == "4.0" {
			major = 4
		} else if run.LocalCurrentVersion != "3.0" {
			t.Fatal("local version")
		}
		if run.Events[0].Kind != "begin" || run.Events[0].UnitVersion != run.LocalCurrentVersion || run.Events[0].UnitVersionSource != "explicit-local-current-setter" {
			t.Fatal("native context provenance")
		}
		for _, event := range run.Events {
			if event.Kind != "case" {
				continue
			}
			model := map[string]string{"CJ1M/CPU21": codec.CJ1MCPU21, "CJ1H-H/CPU65": codec.CJ1HCPU65}[event.Device+"/"+event.CPU]
			if model == "" || !event.Accepted || !slices.Equal(event.LocalCurrentVersionBytes, []int{major, 0, 0}) {
				t.Fatal("native case/context")
			}
			// This is the current independently implemented recipe. It is
			// compared against explicit native3/4, without qualifying every
			// production API for an explicit unit version.
			profile := codec.Profile{Model: model, TimerCounterMode: mode}
			if run.Kind == "instructions" {
				instruction, err := il.Parse(event.Mnemonic + " " + strings.Join(event.Operands, " "))
				if err != nil {
					t.Fatal(err)
				}
				words, err := codec.EncodeInstruction(profile, instruction)
				if err != nil || !slices.Equal(words, event.Words) {
					t.Fatalf("%s %s %s: %v", run.LocalCurrentVersion, mode, event.ID, err)
				}
				instructions++
				continue
			}
			if run.Kind != "images" {
				t.Fatal("unknown run kind")
			}
			file := event.ProgramObject.File
			want, err := hex.DecodeString(file.FileHex)
			if err != nil || event.ProgramObject.CompileResult != 1 || file.Returned != 1 || file.ErrorCount != 0 || file.TaskSelector != 65535 || file.ByteLength != len(want) {
				t.Fatal("native complete image")
			}
			sourcePath := map[string]string{"bit-full": "testdata/n1-finalization-probe.cxt", "boundary-2824": "testdata/n1-finalization-boundary-probe.cxt"}[event.ID]
			if sourcePath == "" {
				t.Fatal("unknown image")
			}
			source, err := os.ReadFile(sourcePath)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := cxp.Read(source)
			if err != nil {
				t.Fatal(err)
			}
			built, err := compiler.CompileCXPImage(profile, doc, "Resource[0]", settings)
			if err != nil || !bytes.Equal(built.Bytes(), want) {
				t.Fatalf("%s %s %s independent image: %v", run.LocalCurrentVersion, mode, event.ID, err)
			}
			if len(built.ResourceBody.FBPrefix.Words) != 0 || built.Summary().HardwareQualified {
				t.Fatal("image scope")
			}
			for _, word := range built.HeaderWords[15:32] {
				if word != 0 {
					t.Fatal("emitted absent FB descriptor")
				}
			}
			verified, err := compiler.VerifyCXPImageBuild(profile, source, "auto", "Resource[0]", settings, want)
			if err != nil || !verified.Matches(model, want) {
				t.Fatalf("image reproduction: %v", err)
			}
			profile.UnitVersion = "3.0"
			if _, err = compiler.CompileCXPImage(profile, doc, "Resource[0]", settings); !errors.Is(err, codec.ErrUnqualifiedProfile) {
				t.Fatal("promoted explicit full profile")
			}
			key := string(mode) + "/" + model + "/" + event.ID
			if prior, ok := seenImages[key]; ok && !bytes.Equal(prior, want) {
				t.Fatal("native3/4 image difference")
			}
			seenImages[key] = want
			images++
		}
	}
	if instructions != 5560 || images != 16 || len(seenImages) != 8 {
		t.Fatalf("incomplete corpus %d/%d/%d", instructions, images, len(seenImages))
	}
}
