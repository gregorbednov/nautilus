package codec_test

import (
	"reflect"
	"strings"
	"testing"
)

// This qualifies the research mode selection, not a production timer codec.
func TestTimerModeOracle(t *testing.T) {
	defaults := loadLibraryResults(t, "timer-library-default-manifest.json", 28)
	binary := loadLibraryResults(t, "timer-binary-manifest.json", 28)
	bcd := loadLibraryResults(t, "timer-bcd-manifest.json", 28)
	accepted := 0
	for i, row := range defaults {
		if row.ID != binary[i].ID || row.ID != bcd[i].ID || row.Device != binary[i].Device || row.Device != bcd[i].Device {
			t.Fatal("uncorrelated mode cases")
		}
		if row.Accepted != binary[i].Accepted || !reflect.DeepEqual(row.Words, binary[i].Words) || !reflect.DeepEqual(row.OperandResults, binary[i].OperandResults) || row.Initialise != binary[i].Initialise {
			t.Fatal("default differs from measured binary mode")
		}
		if strings.HasPrefix(row.Mnemonic, "MOV") {
			if !row.Accepted || !bcd[i].Accepted || !reflect.DeepEqual(row.Words, bcd[i].Words) {
				t.Fatal("MOV control changed with timer mode")
			}
			continue
		}
		x := strings.Contains(row.Mnemonic, "X")
		if row.Accepted != x || bcd[i].Accepted != (!x && row.Operands[1] == "#0010") {
			t.Fatalf("unexpected mode acceptance: %s", row.Mnemonic)
		}
		if row.Accepted {
			accepted++
		}
		if bcd[i].Accepted {
			accepted++
		}
	}
	if accepted != 18 {
		t.Fatal("timer probe coverage changed", accepted)
	}
}
