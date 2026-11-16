package codec_test

import (
	"encoding/binary"
	"os"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
)

func settingsFromReadback(memory *memoryReadback) codec.N1FBMemorySettings {
	settings := codec.N1FBMemorySettings{Stack: codec.N1MemoryStack{Address: memory.Stack.Name, FirstSize: memory.Stack.FirstSize, TotalSize: memory.Stack.TotalSize}}
	for _, base := range memory.Bases[:5] {
		settings.Bases = append(settings.Bases, codec.N1MemoryBase{Address: base.Name, Size: base.Size})
	}
	return settings
}

func TestN1FixedHeaderMatchesControlledFBFiles(t *testing.T) {
	for _, row := range loadLibraryProbeResults(t, "fb-memory-file-manifest.json", 12, "program-object-v1") {
		t.Run(row.Device+"/"+row.ID, func(t *testing.T) { checkOwnedProgramImage(t, row, codec.TimerModeBCD, 0) })
	}
}

func TestN1FixedHeaderMatchesSavedProjectCaptures(t *testing.T) {
	memory := codec.N1FBMemorySettings{Bases: []codec.N1MemoryBase{{Address: "T3072", Size: 1024}, {Address: "C3072", Size: 1024}, {Address: "DIFF8191"}, {Address: "H512", Size: 896}, {Address: "H1408", Size: 128}}, Stack: codec.N1MemoryStack{Address: "AC0", FirstSize: 64, TotalSize: 1536}}
	for _, spec := range []struct{ model, file string }{{codec.CJ1MCPU21, "cj1m-input-bit.n1"}, {codec.CJ1HCPU65, "cj1h-baseline.n1"}} {
		words, err := codec.EncodeN1Header(codec.Profile{Model: spec.model, TimerCounterMode: codec.TimerModeBCD}, codec.N1TaskOptions{}, codec.N1HeaderLayout{DefinitionWords: 50, InstanceWords: 116, TaskBlockWords: 652}, &memory)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile("../um/testdata/" + spec.file)
		if err != nil {
			t.Fatal(err)
		}
		for i, w := range words {
			if binary.BigEndian.Uint32(raw[i*4:]) != w {
				t.Fatalf("%s header word %d differs", spec.file, i)
			}
		}
	}
}

func TestN1FixedHeaderRejectsUnsupportedLayout(t *testing.T) {
	profile := codec.Profile{Model: codec.CJ1MCPU21, TimerCounterMode: codec.TimerModeBCD}
	for _, layout := range []codec.N1HeaderLayout{{}, {TaskBlockWords: 325}, {TaskBlockWords: 326, DefinitionWords: -1}, {TaskBlockWords: 326, DefinitionWords: 1}, {TaskBlockWords: 326, InstanceWords: 62}, {TaskBlockWords: 326, DefinitionWords: 33}, {TaskBlockWords: 326, DefinitionWords: 33, InstanceWords: 1}, {TaskBlockWords: int(^uint(0) >> 1)}, {TaskBlockWords: 4 << 20}} {
		if words, err := codec.EncodeN1Header(profile, codec.N1TaskOptions{}, layout, nil); words != nil || err == nil {
			t.Fatal("accepted unsupported layout", layout, err)
		}
	}
	if words, err := codec.EncodeN1Header(codec.Profile{}, codec.N1TaskOptions{}, codec.N1HeaderLayout{TaskBlockWords: 326}, nil); words != nil || err == nil {
		t.Fatal("unqualified profile")
	}
	if words, err := codec.EncodeN1Header(profile, codec.N1TaskOptions{}, codec.N1HeaderLayout{TaskBlockWords: 326}, &codec.N1FBMemorySettings{}); words != nil || err == nil {
		t.Fatal("unqualified memory")
	}
}
