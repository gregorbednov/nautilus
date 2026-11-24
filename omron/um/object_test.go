package um

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"
)

func TestPlainN1ObjectNativeLayout(t *testing.T) {
	// The prefix/copy recipe is static native evidence, not a captured OBJ.
	for _, name := range []string{"cj1h-baseline.n1", "cj1m-input-bit.n1"} {
		t.Run(name, func(t *testing.T) {
			n1, err := os.ReadFile("testdata/" + name)
			if err != nil {
				t.Fatal(err)
			}
			saved := bytes.Clone(n1)
			object, err := WrapPlainN1Object(n1)
			if err != nil {
				t.Fatal(err)
			}
			if len(object) != len(saved)+48 || !bytes.Equal(object[:48], make([]byte, 48)) || !bytes.Equal(object[48:], saved) {
				t.Fatal("native zero-prefix / complete-body copy layout differs")
			}
			n1[0] ^= 1
			if !bytes.Equal(object[48:], saved) {
				t.Fatal("wrapper aliases source")
			}
			image, err := ParsePlainN1Object(object)
			if err != nil {
				t.Fatal(err)
			}
			object[48] ^= 1
			if !bytes.Equal(image.Original(), saved) || image.Summary().HardwareQualified {
				t.Fatal("parser aliases container or claims hardware qualification")
			}
		})
	}
}

func TestPlainN1ObjectRejectsOtherContainers(t *testing.T) {
	n1 := oracle(t)
	plain, err := WrapPlainN1Object(n1)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 48; i++ {
		data := bytes.Clone(plain)
		data[i] = 1
		if image, err := ParsePlainN1Object(data); image != nil || !errors.Is(err, ErrPlainN1ObjectLayout) {
			t.Fatalf("accepted nonzero prefix byte %d", i)
		}
	}
	protected := make([]byte, 48+380+len(n1))
	binary.BigEndian.PutUint32(protected[4:], 0x80)
	copy(protected[48+380:], n1)
	damaged := bytes.Clone(plain)
	binary.BigEndian.PutUint32(damaged[48+0x22*4:], 0xffffffff)
	for name, data := range map[string][]byte{
		"raw N1": n1, "protected": protected, "CJ2 magic": append([]byte("CJ2X"), plain[4:]...),
		"short": plain[:47], "empty body": plain[:48], "metadata tail": append(bytes.Clone(plain), 0, 0, 0, 0),
		"damaged N1": damaged, "oversized": make([]byte, MaxSize+48+4),
	} {
		t.Run(name, func(t *testing.T) {
			if image, err := ParsePlainN1Object(data); image != nil || !errors.Is(err, ErrPlainN1ObjectLayout) {
				t.Fatal("accepted unsupported/malformed object")
			}
		})
	}
	if data, err := WrapPlainN1Object(nil); data != nil || !errors.Is(err, ErrPlainN1ObjectLayout) || !errors.Is(err, ErrN1Layout) {
		t.Fatal("wrapper accepted invalid N1 or lost error cause")
	}
	if _, err := ParsePlainN1Object(damaged); !errors.Is(err, ErrN1Layout) {
		t.Fatal("parser lost body error cause")
	}
}

func TestPlainN1ObjectNativeEvidence(t *testing.T) {
	packed, err := os.Open("testdata/plain-object-writer-ghidra.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer packed.Close()
	reader, err := gzip.NewReader(packed)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	var report struct {
		SourceSHA256 string
		Functions    []struct {
			Entry              string
			DecompileCompleted bool
			Instructions       []struct{ Address, Text string }
		}
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if report.SourceSHA256 != "e91f15b8613e56c8f326bc83ce07f5a24b177f5f70a4a445e4eadfa34b0a9eb4" || len(report.Functions) != 1 || report.Functions[0].Entry != "12685840" || !report.Functions[0].DecompileCompleted {
		t.Fatal("native writer provenance changed")
	}
	instructions := map[string]string{}
	for _, ins := range report.Functions[0].Instructions {
		instructions[ins.Address] = ins.Text
	}
	for address, expected := range map[string]string{
		"126859f8": "MOV ECX,0xc", "12685a05": "STOSD.REP ES:EDI",
		"12685a4a": "PUSH 0x30", "12685a50": "CALL 0x1285b9ea",
		"12685a2b": "MOV dword ptr [EBP + 0xc],0x800000",
		"126859c2": "MOV dword ptr [EBP + 0xc],0x810000",
		"12685b5e": "PUSH 0x17c", "12685b67": "CALL 0x1285b9ea",
		"12685bc5": "CALL 0x1285bbd6", "12685bd3": "CALL 0x1285b9ea",
	} {
		if instructions[address] != expected {
			t.Fatalf("native writer instruction %s changed", address)
		}
	}
}
