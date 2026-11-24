package um

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"
)

func nativeSumAsset(t *testing.T, name string, out any) {
	t.Helper()
	manifestBytes, err := os.ReadFile("testdata/native-um-sum-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		SourceSHA256, CodeSHA256, PlainCodeSHA256                string
		FunctionBytes, NativeCases, SyntheticCases, LiveSumCases int
		HardwareQualified, TargetFinalizationQualified           bool
		Artifacts                                                map[string]struct{ SHA256, DecodedSHA256 string }
	}
	if err = json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.SourceSHA256 != "be53c4711cd58094919fffa058de90b6f223357572a54b410a28f61abc7c589a" || manifest.CodeSHA256 != "0d993a5edc6d751a13abdc80dc6440992ee1811365d5af873d1b7fb11e3e3f99" || manifest.PlainCodeSHA256 != manifest.CodeSHA256 || manifest.FunctionBytes != 100 || manifest.NativeCases != 80 || manifest.SyntheticCases != 76 || manifest.LiveSumCases != 4 || manifest.HardwareQualified || manifest.TargetFinalizationQualified {
		t.Fatal("native capsule provenance/scope")
	}
	asset, ok := manifest.Artifacts[name]
	if !ok {
		t.Fatal("missing native asset")
	}
	packed, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(packed)) != asset.SHA256 {
		t.Fatal("native asset hash")
	}
	z, err := gzip.NewReader(bytes.NewReader(packed))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	decoded, err := io.ReadAll(io.LimitReader(z, 4<<20))
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded) >= 4<<20 || fmt.Sprintf("%x", sha256.Sum256(decoded)) != asset.DecodedSHA256 {
		t.Fatal("native decoded asset hash/size")
	}
	if err = json.Unmarshal(decoded, out); err != nil {
		t.Fatal(err)
	}
}

func TestUMSumMatchesIsolatedNativeArithmetic(t *testing.T) {
	var cases struct {
		Cases []struct {
			ID, DataHex string
			Mode, Count int
		}
	}
	var results struct {
		Architecture                   string
		NativeDLLLoaded, NetworkAccess bool
		Cases                          []struct {
			ID               string
			Mode, Count      int
			Actual, Expected uint32
			Matches          bool
		}
	}
	nativeSumAsset(t, "native-um-sum-cases.json.gz", &cases)
	nativeSumAsset(t, "native-um-sum-results.json.gz", &results)
	if len(cases.Cases) != 76 || len(results.Cases) != 80 || results.Architecture != "x86" || results.NativeDLLLoaded || results.NetworkAccess {
		t.Fatal("native worker scope")
	}
	byID := map[string]uint32{}
	for _, r := range results.Cases {
		if !r.Matches || r.Actual != r.Expected {
			t.Fatal("native mismatch")
		}
		if _, exists := byID[r.ID]; exists {
			t.Fatal("duplicate result")
		}
		byID[r.ID] = r.Actual
	}
	checked := 0
	for _, c := range cases.Cases {
		if c.Mode != 2 && c.Mode != 3 {
			continue
		}
		data, err := hex.DecodeString(c.DataHex)
		if err != nil {
			t.Fatal(err)
		}
		n := max(0, c.Count)
		if n > len(data) {
			t.Fatal("native case bound")
		}
		// Native x86 reads LE words; N1 uses BE containers. The arithmetic
		// comparison changes representation only, not the numeric values.
		be := make([]byte, n)
		for offset := 0; offset+4 <= n; offset += 4 {
			binary.BigEndian.PutUint32(be[offset:], binary.LittleEndian.Uint32(data[offset:]))
		}
		copy(be[n&^3:], data[n&^3:n])
		want, exists := byID[c.ID]
		if !exists {
			t.Fatal("missing native result")
		}
		if got := sum32BE(be); got != want {
			t.Fatalf("%s: %08x != native %08x", c.ID, got, want)
		}
		checked++
	}
	if checked != 38 {
		t.Fatal("incomplete native32-bit replay", checked)
	}
}

func TestObservedUMBlockCountAndNativeSums(t *testing.T) {
	var rows []struct {
		SourceSHA256                  string
		ImageBytes                    int
		ServiceWords                  [6]uint32
		NativeSum32, ExpectedBlocks64 uint32
	}
	nativeSumAsset(t, "native-um-service-observations.json.gz", &rows)
	if len(rows) != 4 {
		t.Fatal("missing hardware observations")
	}
	for _, r := range rows {
		if len(r.SourceSHA256) != 64 || r.ServiceWords[0] != r.NativeSum32&0xffff || r.ServiceWords[1] != r.NativeSum32>>16 || r.ExpectedBlocks64 != uint32((r.ImageBytes+63)/64) || r.ServiceWords[4] != r.ExpectedBlocks64 {
			t.Fatal("observed relation failed", r)
		}
		if r.ServiceWords[4] == uint32((r.ImageBytes+255)/256) {
			t.Fatal("observed64-byte relation conflated with simulator256-byte quantum")
		}
	}
}

func TestNativeUMSumStaticEvidenceProvenance(t *testing.T) {
	var roots []struct {
		Engine, EngineVersion, SourceSHA256 string
		Functions                           []struct {
			Entry              string
			DecompileCompleted bool
		}
	}
	nativeSumAsset(t, "native-um-sum-ghidra.json.gz", &roots)
	if len(roots) != 9 {
		t.Fatal("incomplete static capsule")
	}
	allowed := map[string]bool{
		"be53c4711cd58094919fffa058de90b6f223357572a54b410a28f61abc7c589a": true,
		"397a44d32ffd57cb926c854eac445c471665d2e73e4ad1abfa55c7657f0f1d84": true,
		"2c16a7c364017c85327876ae9d1156878673e6c7d5e6e5b651120724b9ffbca2": true,
		"21430e768190361b7dc137ff968e2839ae269d087823f2ca16270076caa9402b": true,
	}
	for _, r := range roots {
		if r.Engine != "ghidra" || r.EngineVersion != "12.1.4" || !allowed[r.SourceSHA256] || len(r.Functions) == 0 {
			t.Fatal("unknown static provenance")
		}
		for _, f := range r.Functions {
			if f.Entry == "" || !f.DecompileCompleted {
				t.Fatal("incomplete function export")
			}
		}
	}
}
