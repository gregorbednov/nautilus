package fbcode_test

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"
)

func TestSourceCallerSelectsSavedPF0Invocation(t *testing.T) {
	data, err := os.ReadFile("testdata/source-invocation-ghidra.json.gz")
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(data)) != "869699e8cc99b139b36c03ab4d955953f7a69d8d5c4e57dfcff2d45692ed66ad" {
		t.Fatal("changed invocation caller evidence", err)
	}
	zip, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	data, err = io.ReadAll(io.LimitReader(zip, (16<<20)+1))
	zip.Close()
	if err != nil || len(data) > 16<<20 || fmt.Sprintf("%x", sha256.Sum256(data)) != "c9bdbf5f3c0e69d021b818d6951e62555c6bf977e5ddbf10d0b35851a249538c" {
		t.Fatal("invalid compressed caller proof", err)
	}
	var proof struct {
		Engine, EngineVersion, SourceSHA256 string
		Functions                           []struct {
			Entry              string
			DecompileCompleted bool
			Instructions       []struct{ Address, Text string }
		}
	}
	if err := json.Unmarshal(data, &proof); err != nil {
		t.Fatal(err)
	}
	if proof.Engine != "ghidra" || proof.EngineVersion != "12.1.4" || proof.SourceSHA256 != "e91f15b8613e56c8f326bc83ce07f5a24b177f5f70a4a445e4eadfa34b0a9eb4" || len(proof.Functions) != 1 || proof.Functions[0].Entry != "12713de0" || !proof.Functions[0].DecompileCompleted {
		t.Fatal("unreviewed source caller")
	}
	want := map[string]string{"12713e07": "XOR EBX,EBX", "12713e40": "PUSH EBX", "12713e4c": "CALL 0x1285adf6"}
	for _, instruction := range proof.Functions[0].Instructions {
		if expected, ok := want[instruction.Address]; ok {
			if instruction.Text != expected {
				t.Fatal("changed invocation-option argument")
			}
			delete(want, instruction.Address)
		}
	}
	if len(want) != 0 {
		t.Fatal("incomplete invocation-option argument proof")
	}
}
