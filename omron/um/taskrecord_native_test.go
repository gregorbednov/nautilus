package um

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

// Static writer evidence does not qualify an extracted record as a task file.
func TestTaskFileWriterNativeEvidence(t *testing.T) {
	packed, err := os.ReadFile("testdata/task-file-writer-evidence.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(packed)
	if hex.EncodeToString(sum[:]) != "bf40c7b2e125e0630ccf6609f00fbfbcf9e4c02cb19e6a002884945314e6e6c5" {
		t.Fatal("task writer evidence hash changed")
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
			t.Fatal("unknown source binary")
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
	if len(evidence.Reports) != 5 || count != 10 {
		t.Fatal("incomplete native writer provenance")
	}
	for address, want := range map[string]string{
		// Worker constructs an explicit file path and passes a selected task ID.
		"124b8ee1": "ADD EAX,-0x3", "124b8ee6": "CALL 0x1285b24c", "124b8efe": "CALL 0x1285b23a",
		"124b8f1c": "PUSH EAX", "124b8f1e": "CALL 0x1280edf0", "124b8f32": "PUSH EAX", "124b8f47": "CALL 0x1285ab02",
		"1280edf0": "MOV EAX,dword ptr [ECX + 0x4]",
		// Full N1 header and directory are gated on selectorffff.
		"121c5760": "CMP dword ptr [EBP + 0x18],0xffff", "121c5767": "JNZ 0x121c580a",
		"121c576d": "PUSH 0x528", "121c5786": "CALL 0x121c632d",
		"121c5cb9": "CMP dword ptr [EBP + 0x18],0xffff", "121c5cc0": "JNZ 0x121c5ce0", "121c5cc2": "PUSH 0x480",
		// Task record header/wrapper are gated on the selected descriptor+204 ID.
		"121c5fec": "CMP dword ptr [EBP + 0x18],0xffff", "121c5ffe": "CMP EAX,dword ptr [EDX + 0x204]",
		"121c6004": "JNZ 0x121c6026", "121c6021": "CALL 0x121c632d",
		"121c6042": "CMP EDX,dword ptr [ECX + 0x204]", "121c6048": "JNZ 0x121c6189",
		// The pre-transfer query constructs read selectors, not task-file bytes.
		"1263fb14": "MOV word ptr [ECX],0x316", "1263fb1f": "MOV word ptr [EDX],0x306",
		"12668773": "TEST AL,0x1c", "12668777": "MOV EAX,0x8000", "12668797": "MOV EAX,0x4000", "126687b7": "MOV EAX,0xc00",
	} {
		if instructions[address] != want {
			t.Errorf("native task writer at %s: got %q, want %q", address, instructions[address], want)
		}
	}
}
