package compiler_test

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/cxp"
)

func TestSourceInvocationWholeImageMatchesExplicitNativeGuards(t *testing.T) {
	f, err := os.Open("testdata/native-source-invocation.json.gz")
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
		SchemaVersion                                                                                                        int
		Method, CompilerSHA256, SourceSHA256                                                                                 string
		VendorCodeExecuted, SourceInvocationQualified                                                                        bool
		NetworkAccess, HardwareQualified, WriteQualified, ParameterStackContractQualified, ExplicitCodecUnitVersionQualified bool
		Runs                                                                                                                 []struct {
			LocalCurrentVersion, ProgramCheckVersion string
			Events                                   []struct {
				Kind, Device, CPU, ID, Role                string
				Accepted                                   bool
				UseCurrentVersion, AfterPackedGuardVersion int
				SelectedGuardVersionBytes                  []int
				InstanceBinding                            struct {
					Returned                                                   int
					FileHex                                                    string
					SourceInvocationQualified, ParameterStackContractQualified bool
					Programs                                                   []struct {
						ErrorCount, WarningCount int
						Words                    []uint32
					}
				}
			}
		}
	}
	if err = json.NewDecoder(z).Decode(&corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.SchemaVersion != 1 || corpus.Method != "native-source-invocation-lifecycle-v1" || corpus.CompilerSHA256 != "a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141" || !corpus.VendorCodeExecuted || !corpus.SourceInvocationQualified || corpus.NetworkAccess || corpus.HardwareQualified || corpus.WriteQualified || corpus.ParameterStackContractQualified || corpus.ExplicitCodecUnitVersionQualified || len(corpus.Runs) != 2 {
		t.Fatal("scope")
	}
	source, err := os.ReadFile("testdata/n1-source-invocation.cxt")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(source)
	if hex.EncodeToString(sum[:]) != corpus.SourceSHA256 {
		t.Fatal("source changed")
	}
	seen := map[string]bool{}
	guards, rejected := 0, 0
	for _, run := range corpus.Runs {
		major := 3
		if run.LocalCurrentVersion == "4.0" {
			major = 4
		} else if run.LocalCurrentVersion != "3.0" {
			t.Fatal("current version")
		}
		if run.ProgramCheckVersion != run.LocalCurrentVersion {
			t.Fatal("guard version")
		}
		for _, e := range run.Events {
			if e.Kind == "program-compile-version" {
				if e.UseCurrentVersion != 0 || !slices.Equal(e.SelectedGuardVersionBytes, []int{major, 0}) || e.AfterPackedGuardVersion != major<<8 {
					t.Fatal("native guard")
				}
				guards++
			}
			if e.Kind != "case" {
				continue
			}
			r := e.InstanceBinding
			if len(r.Programs) != 3 || r.ParameterStackContractQualified {
				t.Fatal("program scope")
			}
			if e.ID == "invocation-no-enable" {
				if e.Accepted || r.Returned != 0 || r.FileHex != "" || r.SourceInvocationQualified || r.Programs[0].ErrorCount != 9 || r.Programs[1].ErrorCount != 0 || r.Programs[2].ErrorCount != 0 {
					t.Fatal("negative control")
				}
				rejected++
				continue
			}
			if e.ID != "invocation-link" {
				continue
			}
			if !e.Accepted || r.Returned != 1 || !r.SourceInvocationQualified {
				t.Fatal("native source invocation rejected")
			}
			for _, p := range r.Programs {
				if p.ErrorCount != 0 || p.WarningCount != 0 {
					t.Fatal("native diagnostics")
				}
			}
			model := codec.CJ1MCPU21
			if e.Device == "CJ1H-H" && e.CPU == "CPU65" {
				model = codec.CJ1HCPU65
			} else if e.Device != "CJ1M" || e.CPU != "CPU21" {
				t.Fatal("CPU")
			}
			key := model + run.LocalCurrentVersion
			if seen[key] {
				t.Fatal("duplicate native context")
			}
			seen[key] = true
			want, err := hex.DecodeString(r.FileHex)
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(want)
			if len(want) != 3180 || hex.EncodeToString(sum[:]) != "0345bfbcfcedbc88cd90759cf30e7948ecad94d805cb1e5e040acd101dece502" {
				t.Fatal("native image")
			}
			doc, err := cxp.Read(source)
			if err != nil {
				t.Fatal(err)
			}
			profile := codec.Profile{Model: model, TimerCounterMode: codec.TimerModeBCD}
			settings := imageSettings(t)
			image, err := compiler.CompileCXPImage(profile, doc, "Resource[0]", settings)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(image.Bytes(), want) || image.Summary().HardwareQualified {
				t.Fatal("independent source image differs from native full compilation")
			}
			proof, err := compiler.VerifyCXPImageBuild(profile, source, "auto", "Resource[0]", settings, want)
			if err != nil || !proof.Matches(model, want) {
				t.Fatal("source reproduction", err)
			}
			// This bounded recipe does not qualify all version-sensitive instructions.
			profile.UnitVersion = "3.0"
			if _, err = compiler.CompileCXPImage(profile, doc, "Resource[0]", settings); !errors.Is(err, codec.ErrUnqualifiedProfile) {
				t.Fatal("explicit profile gate lifted", err)
			}
		}
	}
	if len(seen) != 4 || guards != 36 || rejected != 4 {
		t.Fatalf("coverage %d/%d/%d", len(seen), guards, rejected)
	}
}
