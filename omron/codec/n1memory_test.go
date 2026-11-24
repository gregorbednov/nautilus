package codec_test

import (
	"encoding/binary"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
)

type memoryReadback struct {
	BaseResults []int `json:"baseResults"`
	StackResult int   `json:"stackResult"`
	Bases       []struct {
		Name    string `json:"name"`
		Address uint32 `json:"address"`
		Size    int    `json:"size"`
	} `json:"bases"`
	Stack struct {
		Name       string `json:"name"`
		FirstSize  int    `json:"firstSize"`
		TotalSize  int    `json:"totalSize"`
		Start      uint32 `json:"start"`
		OtherStart uint32 `json:"otherStart"`
		Size       int    `json:"size"`
	} `json:"stack"`
}

func checkMemoryReadback(t *testing.T, input codec.N1FBMemorySettings, result *memoryReadback) {
	t.Helper()
	if result == nil || len(input.Bases) != 5 || len(result.BaseResults) != 5 || len(result.Bases) != 7 {
		t.Fatal("missing memory settings/readbacks")
	}
	for i, base := range input.Bases {
		if result.BaseResults[i] != 1 || result.Bases[i].Name != base.Address || result.Bases[i].Size != base.Size {
			t.Fatal("base input/readback mismatch")
		}
	}
	stack := input.Stack
	returned := 1
	if stack.FirstSize > stack.TotalSize {
		returned = 0
		stack = codec.N1MemoryStack{Address: "AC0", FirstSize: 64, TotalSize: 1536}
	}
	if result.StackResult != returned || result.Stack.Name != stack.Address || result.Stack.FirstSize != stack.FirstSize || result.Stack.TotalSize != stack.TotalSize || result.Stack.Size != stack.TotalSize || result.Stack.OtherStart-result.Stack.Start != uint32(stack.FirstSize) {
		t.Fatal("stack input/readback mismatch")
	}
}

func TestN1FBMemoryMatchesNativeSettings(t *testing.T) {
	for _, row := range loadLibraryProbeResults(t, "header-memory-manifest.json", 12, "program-object-v1") {
		t.Run(row.Device+"/"+row.ID, func(t *testing.T) {
			memory := row.ProgramObject.Memory
			model := codec.CJ1MCPU21
			if row.Device == "CJ1H-H" {
				model = codec.CJ1HCPU65
			}
			profile := codec.Profile{Model: model, TimerCounterMode: codec.TimerModeBCD}
			settings := codec.N1FBMemorySettings{Stack: codec.N1MemoryStack{Address: memory.Stack.Name, FirstSize: memory.Stack.FirstSize, TotalSize: memory.Stack.TotalSize}}
			var expected []uint32
			for i, base := range memory.Bases {
				if i < 5 {
					settings.Bases = append(settings.Bases, codec.N1MemoryBase{Address: base.Name, Size: base.Size})
				} else if base.Name != "" || base.Address != 0 || base.Size != 0 {
					t.Fatal("unexpected reserved bank")
				}
				expected = append(expected, base.Address, uint32(base.Size))
			}
			expected = append(expected, memory.Stack.Start, memory.Stack.OtherStart, uint32(memory.Stack.Size))
			if row.ID == "invalid-split" {
				settings.Stack.FirstSize, settings.Stack.TotalSize = 81, 80
				if words, err := codec.EncodeN1FBMemory(profile, settings); words != nil || !errors.Is(err, codec.ErrN1FBMemory) {
					t.Fatal("accepted invalid stack split", err)
				}
			} else {
				words, err := codec.EncodeN1FBMemory(profile, settings)
				if err != nil || !reflect.DeepEqual(words, expected) {
					t.Fatalf("memory descriptor %x != %x: %v", words, expected, err)
				}
				if row.ID == "default" {
					image, err := os.ReadFile("../um/testdata/cj1m-input-bit.n1")
					if err != nil {
						t.Fatal(err)
					}
					for i, word := range words {
						if binary.BigEndian.Uint32(image[(15+i)*4:]) != word {
							t.Fatalf("captured FB header word %d differs", 15+i)
						}
					}
				}
			}
			// No definitions: all17 memory header words are zero despite the
			// configured getters above. Verify the whole 656-word file.
			checkOwnedProgramImage(t, row, codec.TimerModeBCD, 0)
		})
	}
}

func TestN1FBMemoryRejectsUnqualifiedSettingsAtomically(t *testing.T) {
	valid := func() codec.N1FBMemorySettings {
		return codec.N1FBMemorySettings{Bases: []codec.N1MemoryBase{{Address: "T3072", Size: 1024}, {Address: "C3072", Size: 1024}, {Address: "DIFF8191"}, {Address: "H512", Size: 896}, {Address: "H1408", Size: 128}}, Stack: codec.N1MemoryStack{Address: "AC0", FirstSize: 64, TotalSize: 1536}}
	}
	profile := codec.Profile{Model: codec.CJ1MCPU21, TimerCounterMode: codec.TimerModeBCD}
	for _, change := range []func(*codec.N1FBMemorySettings){
		func(s *codec.N1FBMemorySettings) { s.Bases = nil },
		func(s *codec.N1FBMemorySettings) { s.Bases = append(s.Bases, codec.N1MemoryBase{}) },
		func(s *codec.N1FBMemorySettings) { s.Bases[0].Address = "C3072" },
		func(s *codec.N1FBMemorySettings) { s.Bases[0].Size = 1025 },
		func(s *codec.N1FBMemorySettings) { s.Bases[1].Size = -1 },
		func(s *codec.N1FBMemorySettings) { s.Bases[2].Size = 1 },
		func(s *codec.N1FBMemorySettings) { s.Bases[3].Address = "H0512" },
		func(s *codec.N1FBMemorySettings) { s.Bases[4].Address = "H512" },
		func(s *codec.N1FBMemorySettings) { s.Stack.Address = "AC1536" },
		func(s *codec.N1FBMemorySettings) { s.Stack.Address = "D32767"; s.Stack.TotalSize = 2 },
		func(s *codec.N1FBMemorySettings) { s.Stack.FirstSize = -1 },
		func(s *codec.N1FBMemorySettings) { s.Stack.TotalSize = 0 },
		func(s *codec.N1FBMemorySettings) { s.Stack.FirstSize = 1537 },
		func(s *codec.N1FBMemorySettings) { s.Stack.TotalSize = int(^uint(0) >> 1) },
	} {
		settings := valid()
		change(&settings)
		if words, err := codec.EncodeN1FBMemory(profile, settings); words != nil || !errors.Is(err, codec.ErrN1FBMemory) {
			t.Fatal("accepted unsupported memory settings", settings, err)
		}
	}
	for _, p := range []codec.Profile{{}, {Model: codec.CJ1MCPU21}, {Model: codec.CJ1MCPU21, TimerCounterMode: codec.TimerModeBinary}, {Model: codec.CJ1HCPU65, UnitVersion: "4.0", TimerCounterMode: codec.TimerModeBCD}} {
		if words, err := codec.EncodeN1FBMemory(p, valid()); words != nil || !errors.Is(err, codec.ErrUnqualifiedProfile) {
			t.Fatal("accepted unsupported memory profile", p, err)
		}
	}
}
