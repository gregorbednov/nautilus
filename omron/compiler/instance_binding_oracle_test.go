package compiler_test

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"slices"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/il"
	"github.com/joyautomation/nautilus/omron/internal/fbcode"
)

func TestNativeBoundInstanceAndRejectedInvocation(t *testing.T) {
	f, err := os.Open("testdata/native-instance-binding.json.gz")
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
		SchemaVersion                                                                                 int
		Method, CompilerSHA256                                                                        string
		VendorCodeExecuted, NetworkAccess, HardwareQualified, WriteQualified                          bool
		SourceInvocationQualified, ParameterStackContractQualified, ExplicitCodecUnitVersionQualified bool
		Runs                                                                                          []struct {
			LocalCurrentVersion, ProgramCheckVersion string
			Events                                   []struct {
				Stage                                               string
				Header                                              struct{ Words []uint32 }
				Preamble                                            []struct{ Words []uint32 }
				Kind, Device, CPU, ID, Role                         string
				Accepted                                            bool
				UseCurrentVersion, AfterPackedGuardVersion          int
				SelectedGuardVersionBytes, LocalCurrentVersionBytes []int
				InstanceBinding                                     struct {
					Returned, ErrorCount                                       int
					FileHex                                                    string
					SourceInvocationQualified, ParameterStackContractQualified bool
					Programs                                                   []struct {
						Role                     string
						ErrorCount, WarningCount int
						Words                    []uint32
					}
				}
			}
		}
	}
	if err = json.NewDecoder(io.LimitReader(z, 8<<20)).Decode(&corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.SchemaVersion != 1 || corpus.Method != "native-instance-binding-with-negative-invocation-v1" || corpus.CompilerSHA256 != "a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141" || !corpus.VendorCodeExecuted || corpus.NetworkAccess || corpus.HardwareQualified || corpus.WriteQualified || corpus.SourceInvocationQualified || corpus.ParameterStackContractQualified || corpus.ExplicitCodecUnitVersionQualified || len(corpus.Runs) != 2 {
		t.Fatal("scope")
	}
	accepted, rejected, guards, frames := 0, 0, 0, 0
	var golden []byte
	for _, run := range corpus.Runs {
		major := 3
		if run.LocalCurrentVersion == "4.0" {
			major = 4
		} else if run.LocalCurrentVersion != "3.0" {
			t.Fatal("current")
		}
		if run.ProgramCheckVersion != run.LocalCurrentVersion {
			t.Fatal("stored check version")
		}
		for _, event := range run.Events {
			if event.Kind == "instance-binding-stage" && event.Stage == "invocation" {
				model, source := codec.CJ1MCPU21, "cj1m.cxp"
				if event.Device == "CJ1H-H" {
					model, source = codec.CJ1HCPU65, "cj1h.cxp"
				}
				body, err := compiler.CompileCXPFBCallBody(codec.Profile{Model: model, TimerCounterMode: codec.TimerModeBCD}, callDocument(t, source), symbolProgram, callRung)
				if err != nil {
					t.Fatal(err)
				}
				frame := slices.Clone(event.Header.Words)
				for _, child := range event.Preamble {
					frame = append(frame, child.Words...)
				}
				if !slices.Equal(frame, body.Words[1:]) {
					t.Fatalf("generated invocation differs from source before full Compile: %x != %x", frame, body.Words[1:])
				}
				frames++
			}
			if event.Kind == "program-compile-version" {
				if event.UseCurrentVersion != 0 || !slices.Equal(event.SelectedGuardVersionBytes, []int{major, 0}) || event.AfterPackedGuardVersion != major<<8 {
					t.Fatal("guard")
				}
				guards++
				continue
			}
			if event.Kind != "case" {
				continue
			}
			r := event.InstanceBinding
			if r.SourceInvocationQualified || r.ParameterStackContractQualified || !slices.Equal(event.LocalCurrentVersionBytes, []int{major, 0, 0}) || len(r.Programs) != 3 {
				t.Fatal("case scope")
			}
			if event.ID == "invocation-link" {
				if event.Accepted || r.Returned != 0 || r.FileHex != "" || r.Programs[0].ErrorCount != 9 || r.Programs[1].ErrorCount != 0 || r.Programs[2].ErrorCount != 0 {
					t.Fatal("rejected native caller silently qualified")
				}
				rejected++
				continue
			}
			if event.ID != "direct-link" || !event.Accepted || r.Returned != 1 || r.ErrorCount != 0 {
				t.Fatal("direct binding")
			}
			model, source := codec.CJ1MCPU21, "cj1m.cxp"
			if event.Device == "CJ1H-H" {
				model, source = codec.CJ1HCPU65, "cj1h.cxp"
			} else if event.Device != "CJ1M" {
				t.Fatal("CPU")
			}
			profile := codec.Profile{Model: model, TimerCounterMode: codec.TimerModeBCD}
			doc := callDocument(t, source)
			definition, err := compiler.CompileCXPFBDefinition(profile, doc, "Resource[0].FunctionBlocks.FunctionBlockWrapper[0].FunctionBlock", 0)
			if err != nil {
				t.Fatal(err)
			}
			instance, err := compiler.CompileCXPFBInstance(profile, doc, "Resource[0].GlobalVariables.POUInstances.Instance[0]")
			if err != nil {
				t.Fatal(err)
			}
			ld, _ := il.Parse("LD W1.00")
			end, _ := il.Parse("END")
			task, _ := codec.EncodeInstruction(profile, ld)
			call, _ := fbcode.InstanceCall(8, 0, 1024)
			tail, _ := codec.EncodeInstruction(profile, end)
			task = append(append(task, call...), tail...)
			for i, want := range [][]uint32{task, definition.Body.Words, instance.Body.Words} {
				if r.Programs[i].ErrorCount != 0 || r.Programs[i].WarningCount != 0 || !slices.Equal(r.Programs[i].Words, want) {
					t.Fatalf("native program %d differs from independent source", i)
				}
			}
			settings := imageSettings(t)
			header, err := codec.EncodeN1Header(profile, codec.N1TaskOptions{ExtendedTIMHold: true}, codec.N1HeaderLayout{DefinitionWords: 50, InstanceWords: 62, TaskBlockWords: 331}, &settings.Memory)
			if err != nil {
				t.Fatal(err)
			}
			words := make([]uint32, 773)
			copy(words, header)
			for i := 0; i < 288; i++ {
				flags := uint32(0x2000)
				if i >= 256 {
					flags = 0x1000
				}
				words[42+i] = uint32(i) | flags
			}
			words[42+256] = 0xd100
			copy(words[330:], []uint32{1, 0, 0, 0})
			copy(words[334:], definition.RecordWords)
			words[380] = 1
			copy(words[388:], instance.RecordWords)
			words[442+256] = 288
			words[730], words[731], words[732] = 256, 0x50726f, 0x626500
			words[730+26], words[730+27] = 43, 32
			copy(words[730+32:], []uint32{0, 0xbbc00f43, 0x61440100, 0x65440000})
			copy(words[730+36:], task)
			words[772] = 0xbb800f48
			want := make([]byte, 3092)
			for i, w := range words {
				binary.BigEndian.PutUint32(want[4*i:], w)
			}
			raw, err := hex.DecodeString(r.FileHex)
			if err != nil || !bytes.Equal(raw, want) {
				t.Fatal("whole bound file differs", err)
			}
			if golden != nil && !bytes.Equal(raw, golden) {
				t.Fatal("native model/version difference")
			}
			golden = raw
			accepted++
		}
	}
	if accepted != 4 || rejected != 4 || guards != 24 || frames != 4 {
		t.Fatalf("coverage %d/%d/%d/%d", accepted, rejected, guards, frames)
	}
}
