package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"testing"
)

func TestCompileReadbackSTDefinitions(t *testing.T) {
	// Hashes of the big-endian records at byte offsets 1336 and 1636
	// in the independent 4672-byte CJ1M CPU21 program-area backup.
	for i, tc := range []struct{ name, number, hash string }{
		{"Humidity", "1", "875fe2c819e3ff4778806a5b5cad0d91e5cfa9c7bf4dd6ee5a7b8fa8af47551b"},
		{"Temperature", "0", "716aed0400daa3284d082ae0d218ff1f15ef68b7069e66fe6bb03495b85376fc"},
	} {
		path := fmt.Sprintf("Resource[0].FunctionBlocks.FunctionBlockWrapper[%d].FunctionBlock", i)
		data, err := captureOmronBodyCommand(t, omronCompileFBDefinitionRecord, []string{"-model", "CJ1M CPU21", "-definition", path, "-number", tc.number, "-timer-mode", "bcd", "../../omron/cxp/testdata/cpu21-upload-st.cxt"})
		if err != nil {
			t.Fatal(err)
		}
		var report struct {
			Result struct {
				Name        string
				RecordWords []uint32
			}
		}
		if err := json.Unmarshal(data, &report); err != nil {
			t.Fatal(err)
		}
		hash := sha256.New()
		if err := binary.Write(hash, binary.BigEndian, report.Result.RecordWords); err != nil {
			t.Fatal(err)
		}
		if report.Result.Name != tc.name || fmt.Sprintf("%x", hash.Sum(nil)) != tc.hash {
			t.Errorf("record mismatch for %s", tc.name)
		}
	}
}
