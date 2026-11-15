package codec_test

import (
	"encoding/binary"
	"os"
	"testing"
)

type taskConfigurationInput struct{ Number, Startup, Protection int }
type headerBase struct{ Address, Size uint32 }
type headerStack struct {
	Name                    string
	FirstSize, SecondSize   int
	Start, OtherStart, Size uint32
}
type taskConfigurationResult struct {
	Number, StartupResult, Startup, ProtectionResult, Protection int
	Word, Absent                                                 uint32
	Bases                                                        []headerBase
	Stack                                                        headerStack
}

func TestTaskConfigurationAndHeaderMemoryNativeEvidence(t *testing.T) {
	data, err := os.ReadFile("../um/testdata/cj1h-baseline.n1")
	if err != nil {
		t.Fatal(err)
	}
	word := func(i int) uint32 { return binary.BigEndian.Uint32(data[i*4:]) }
	for _, row := range loadLibraryProbeResults(t, "task-config-manifest.json", 48, "task-configuration-v1") {
		r := row.TaskConfiguration
		expected := uint32(r.Number) | 0x9000 | uint32(r.Startup)*0x4000 | uint32(r.Protection)*0xc00
		if !row.Accepted || r.Word != expected || r.Absent != 0 || len(r.Bases) != 7 {
			t.Fatal("native configuration word differs", row.ID, r)
		}
		for i, base := range r.Bases {
			if base.Address != word(15+i*2) || base.Size != word(16+i*2) {
				t.Fatal("normal default memory state differs from captured header", row.ID, i)
			}
		}
		if r.Stack.Name != "AC0" || r.Stack.FirstSize != 64 || r.Stack.SecondSize != 1536 || r.Stack.Start != word(29) || r.Stack.OtherStart != word(30) || r.Stack.Size != word(31) {
			t.Fatal("stack getter/readback differs from captured header", row.ID, r.Stack)
		}
	}
}
