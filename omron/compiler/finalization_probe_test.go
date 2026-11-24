package compiler_test

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/um"
)

func TestFinalizationProbeMatchesBothNativeProfiles(t *testing.T) {
	checkFinalizationProbe(t, "testdata/n1-finalization-probe.cxt", "../um/testdata/native-task-file-cases.json.gz", "bit-full", "798dca90199c45dee2c55711d6d9685fba826ae7cb1c66fa98e2bbcb6a76b0f8", 2632, [4]uint32{42, 41, 11, 11})
}

func TestFinalizationBoundaryProbeDistinguishesAllFourRules(t *testing.T) {
	checkFinalizationProbe(t, "testdata/n1-finalization-boundary-probe.cxt", "testdata/finalization-probe-native.json.gz", "boundary-2824", "ce500168349fadf708c2d07dad3c4eb32dde99d710220c920190cfc1154de7be", 2824, [4]uint32{45, 44, 12, 11})
}

func checkFinalizationProbe(t *testing.T, sourceFile, corpusFile, caseID, wantSHA string, imageBytes int, counts [4]uint32) {
	t.Helper()
	raw, err := os.ReadFile(sourceFile)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := cxp.Read(raw)
	if err != nil {
		t.Fatal(err)
	}
	settingsBytes, err := os.ReadFile("testdata/n1-finalization-probe-settings.json")
	if err != nil {
		t.Fatal(err)
	}
	var settings compiler.N1ImageSettings
	if err = json.Unmarshal(settingsBytes, &settings); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(corpusFile)
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
		CompilerSHA256                                                       string
		VendorCodeExecuted, NetworkAccess, HardwareQualified, WriteQualified bool
		Runs                                                                 []struct {
			Cases []struct {
				ID, Device, CPU string
				ProgramObject   struct {
					CompileResult int
					File          struct {
						TaskSelector, Returned, ErrorCount, ByteLength int
						FileHex                                        string
					}
				}
			}
		}
	}
	if err = json.NewDecoder(z).Decode(&corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.CompilerSHA256 != "a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141" || !corpus.VendorCodeExecuted || corpus.NetworkAccess || corpus.HardwareQualified || corpus.WriteQualified {
		t.Fatal("invalid native compiler scope")
	}
	checked := map[string]bool{}
	for _, run := range corpus.Runs {
		for _, c := range run.Cases {
			if c.ID != caseID {
				continue
			}
			model := ""
			switch c.Device + "/" + c.CPU {
			case "CJ1M/CPU21":
				model = codec.CJ1MCPU21
			case "CJ1H-H/CPU65":
				model = codec.CJ1HCPU65
			default:
				t.Fatal("unknown native profile")
			}
			if checked[model] {
				t.Fatal("duplicate native profile")
			}
			checked[model] = true
			want, err := hex.DecodeString(c.ProgramObject.File.FileHex)
			if err != nil {
				t.Fatal(err)
			}
			file := c.ProgramObject.File
			if c.ProgramObject.CompileResult != 1 || file.TaskSelector != 65535 || file.Returned != 1 || file.ErrorCount != 0 || file.ByteLength != imageBytes || len(want) != imageBytes {
				t.Fatal("invalid native full-image evidence")
			}
			sum := sha256.Sum256(want)
			if hex.EncodeToString(sum[:]) != wantSHA {
				t.Fatal("native probe changed")
			}
			built, err := compiler.CompileCXPImage(codec.Profile{Model: model, TimerCounterMode: codec.TimerModeBCD}, doc, "Resource[0]", settings)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(built.Bytes(), want) || built.Summary().HardwareQualified {
				t.Fatal("independent full image differs from native probe")
			}
			parsed, err := um.ParseN1(built.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			// This is a discriminating test recipe, not a predicted hardware result.
			service := parsed.ServiceFields()
			expected := []um.N1BlockCountCandidate{{QuantumBytes: 64, ExcludedPrefixBytes: 0, Count: counts[0]}, {QuantumBytes: 64, ExcludedPrefixBytes: 24, Count: counts[1]}, {QuantumBytes: 256, ExcludedPrefixBytes: 0, Count: counts[2]}, {QuantumBytes: 256, ExcludedPrefixBytes: 24, Count: counts[3]}}
			if !slices.Equal(service.BlockCountCandidates, expected) || service.NativeAlgorithmConfirmed || service.WriteQualified {
				t.Fatalf("probe lost discrimination or gained qualification: %+v", service)
			}
		}
	}
	if len(checked) != 2 {
		t.Fatal("both native profiles required")
	}
}
