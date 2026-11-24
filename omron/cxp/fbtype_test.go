package cxp

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

func fbProof(t *testing.T, name, hash string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil || fmt.Sprintf("%x", sha256.Sum256(data)) != hash {
		t.Fatal("changed FB type evidence", name, err)
	}
	return data
}

func TestFBTypeNamesFromPHeadTypeLibrary(t *testing.T) {
	data := fbProof(t, "phead-typelib.json", "37599f0dbd67c32b10d6b59ba2b60d3daa35fd6fcd0594098c99fb182a76d04f")
	var library struct {
		SchemaVersion, PointerBytes      int
		Method, SourceSHA256, SourceName string
		VendorObjectCreated              bool
		Functions                        []string
		Enums                            []struct {
			Name    string
			Members []struct {
				Name  string
				Value int
			}
		}
	}
	if err := json.Unmarshal(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}), &library); err != nil {
		t.Fatal(err)
	}
	if library.SchemaVersion != 1 || library.PointerBytes != 4 || library.Method != "oleaut32-LoadTypeLibEx-REGKIND_NONE" || library.SourceSHA256 != "463eb4402bb6fa00453a5d7a745d77ced6be454d77aa2a683eb67de7e38ba71c" || library.SourceName != "PHeadDoc.dll" || library.VendorObjectCreated {
		t.Fatal("unreviewed PH_TYPE provenance")
	}
	count := 0
	for _, enum := range library.Enums {
		if enum.Name != "PH_TYPE" {
			continue
		}
		for _, member := range enum.Members {
			if member.Name == "PH_TYPE_END" {
				if FBTypeName(member.Value) != "" {
					t.Fatal("accepted sentinel")
				}
				continue
			}
			if FBTypeName(member.Value) != strings.TrimPrefix(member.Name, "PH_") {
				t.Fatal("PH_TYPE identity differs", member)
			}
			count++
		}
	}
	if count != 17 || FBTypeName(-1) != "" || FBTypeName(99) != "" {
		t.Fatal("type identity coverage", count)
	}
	found := false
	for _, fn := range library.Functions {
		if strings.HasPrefix(fn, "ICXP_POUHeadDoc2.GetItem3(nIndex:3,pbstrName:26,pnUsage:26,penumType:26,") && strings.Contains(fn, ";vtable=432;") {
			found = true
		}
	}
	if !found {
		t.Fatal("GetItem3 does not match serialization vtable slot0x1b0")
	}
	fbProof(t, "typelib-reader.ps1", "c8ebdf7a1b4ef7f20cfd4b603d2f505142cbc44f35560999bf7f9e692de698d8")
	data = fbProof(t, "kbase-typelib.json", "1026e16cc975db4e59eb9196cef8b034adc9c2bbe83a7f718e0511500ff047a1")
	if err := json.Unmarshal(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}), &library); err != nil {
		t.Fatal(err)
	}
	if library.SourceSHA256 != "8ab6abdd01a4928a3ec0a5f7d235e1b2a658a47a17e8407fe53ba538554b261f" || library.SourceName != "KBaseComp.dll" || library.PointerBytes != 4 || library.VendorObjectCreated {
		t.Fatal("unreviewed native type library")
	}
	native := map[string]int{}
	for _, enum := range library.Enums {
		if enum.Name == "nDATA_TYPE" {
			for _, member := range enum.Members {
				native[member.Name] = member.Value
			}
		}
	}
	if native["nDT_BOOL"] != 1 || native["nDT_INT"] != 2 || native["nDT_UINT"] != 4 || native["nDT_WORD"] != 8 || native["nDT_TIMER"] != 65536 {
		t.Fatal("native bitmask changed")
	}
}

func TestFBTypeSerializationChain(t *testing.T) {
	for _, spec := range []struct {
		file, compressed, decoded string
		entries                   []string
	}{
		{"pou-serialization-ghidra.json.gz", "93e722212de28e813ba51268799c9b499292c750e72dbe279894ef07ebac0a73", "29239476bf9df4fa364a85619bab37b248ca28be92a72e32264908f0477f6df8", []string{"124895a0", "12489b40"}},
		{"pou-row-ghidra.json.gz", "cd5255e9a458ebc6756519ea7ae7981f66992f90c0a5a22f149a9967f31da6de", "585eeb6077cbe38c719f433ee1da32a5670fa4ad1b176a192bda1896fa493f93", []string{"1248a3b0", "1248ab80", "12489a90"}},
		{"pou-write-ghidra.json.gz", "7fd809a7b82b275437c6c13b240d5d6d95e119abb068251ed9242655fd162c4c", "1e555d35592235b98364058832919a2b5e88b9415ff9d4fb0a980d06d055e4c7", []string{"1248a560"}},
	} {
		data := fbProof(t, spec.file, spec.compressed)
		zip, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		data, err = io.ReadAll(io.LimitReader(zip, (16<<20)+1))
		zip.Close()
		if err != nil || len(data) > 16<<20 || fmt.Sprintf("%x", sha256.Sum256(data)) != spec.decoded {
			t.Fatal("invalid compressed type proof", err)
		}
		var proof struct {
			Engine, EngineVersion, SourceSHA256 string
			Functions                           []struct {
				Entry              string
				DecompileCompleted bool
			}
		}
		if err := json.Unmarshal(data, &proof); err != nil {
			t.Fatal(err)
		}
		if proof.Engine != "ghidra" || proof.EngineVersion != "12.1.4" || proof.SourceSHA256 != "e91f15b8613e56c8f326bc83ce07f5a24b177f5f70a4a445e4eadfa34b0a9eb4" || len(proof.Functions) != len(spec.entries) {
			t.Fatal("unreviewed source serialization proof")
		}
		for i, fn := range proof.Functions {
			if fn.Entry != spec.entries[i] || !fn.DecompileCompleted {
				t.Fatal("incomplete serialization evidence")
			}
		}
	}
}
