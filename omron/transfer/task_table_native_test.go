package transfer_test

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"testing"
)

// Native task records contain CString fields. They are not a wire format.
func TestTaskTableNativeEvidence(t *testing.T) {
	packed, err := os.ReadFile("testdata/task-table-evidence.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(packed)
	if hex.EncodeToString(sum[:]) != "d7cb3d054d7f4588703278b0ef9a249e6c2d6f45f186c8c2491b0bd9dc0ab232" {
		t.Fatal("task evidence hash changed")
	}
	reader, err := gzip.NewReader(bytes.NewReader(packed))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	var evidence struct {
		Reports []struct {
			SourceSHA256 string
			Functions    []struct {
				Entry              string
				DecompileCompleted bool
				Instructions       []struct{ Address, Text string }
			}
		}
		Literals map[string]string
	}
	if err := json.Unmarshal(data, &evidence); err != nil {
		t.Fatal(err)
	}
	instructions := map[string]string{}
	count := 0
	for _, report := range evidence.Reports {
		switch report.SourceSHA256 {
		case "e91f15b8613e56c8f326bc83ce07f5a24b177f5f70a4a445e4eadfa34b0a9eb4", "a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141":
		default:
			t.Fatal("unrecognized native source")
		}
		for _, f := range report.Functions {
			count++
			if !f.DecompileCompleted {
				t.Fatal("incomplete listing", f.Entry)
			}
			for _, i := range f.Instructions {
				instructions[i.Address] = i.Text
			}
		}
	}
	if len(evidence.Reports) != 7 || count != 17 {
		t.Fatal("incomplete task provenance")
	}
	for address, want := range map[string]string{"129c09d8": "SEC", "12a2ebd0": "PRG", "129ed298": "%03X"} {
		if evidence.Literals[address] != want {
			t.Fatal("filename literal changed", address)
		}
	}
	for address, want := range map[string]string{
		// Dynamic array allocation: (n + 2*(5*n))*4 = 44*n bytes.
		"126523f0": "LEA EAX,[EBP + EBP*0x4]", "126523f4": "LEA ECX,[EBP + EAX*0x2]",
		"126523f8": "SHL ECX,0x2", "126523fc": "CALL 0x1285af94", "126523c7": "ADD ESI,0x2c",
		// Record copies assign CString fields at4,8,20 instead of raw pointer copies.
		"1265253e": "LEA ECX,[ESI + 0x4]", "12652543": "CALL 0x1285b132",
		"1265254b": "LEA ECX,[ESI + 0x8]", "1265254f": "CALL 0x1285b132",
		"12652576": "LEA ECX,[ESI + 0x20]", "12652579": "CALL 0x1285b132",
		// Separate temporary-name and base-name-derived paths.
		"12651af8": "PUSH 0x129c09d8", "12651b0a": "CALL 0x1285a988",
		"12651c67": "PUSH 0x12a2ebd0", "12651c6d": "CALL 0x1285a988",
		"12651caa": "PUSH 0x129ed298", "12651cb8": "CALL 0x1285af1c",
		"12651cd1": "ADD EAX,-0x3", "12651cda": "CALL 0x1285b24c", "12651cf2": "CALL 0x1285b23a",
		"1285a988": "JMP dword ptr [0x12b224fc]",
		// Metadata path assignments: no claim of a serialized N1 layout.
		"126489f8": "LEA ECX,[EDX + 0x3c]", "126489fb": "CALL 0x1285b132",
		"12648af3": "LEA ECX,[EAX + 0x44]", "12648af6": "CALL 0x1285b132",
		// Consumer indexes the same array, then passes record+4 and record+c.
		"12610b20": "MOV ECX,dword ptr [EBP + 0xd4]",
		"12610b66": "LEA ECX,[EAX + EAX*0x4]", "12610b69": "LEA EDX,[EAX + ECX*0x2]",
		"12610b6c": "MOV ECX,dword ptr [EBP + 0xd0]", "12610b72": "LEA EBX,[ECX + EDX*0x4]",
		"12610b7a": "JNZ 0x12611ca4", "12610bb3": "MOV EDX,dword ptr [EBX + 0xc]",
		"12610bb6": "MOV EAX,dword ptr [EBX + 0x4]", "12610bbd": "CALL 0x12611fd0",
		// Whole-program dispatch uses the distinct filename at this+78.
		"12610a99": "MOV EAX,dword ptr [EBP + 0x78]", "12610ab3": "CALL 0x12859810",
		// Temporary-path helper calls GetTempFileNameA with unique=0.
		"12177f70": "PUSH 0x0", "12177f7d": "CALL dword ptr [0x122cb540]",
	} {
		if instructions[address] != want {
			t.Errorf("task path at%s: got%q want%q", address, instructions[address], want)
		}
	}
}
