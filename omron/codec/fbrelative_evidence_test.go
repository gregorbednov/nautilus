package codec_test

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestFBRelativeNativeAPIAndSignedOffsetEvidence(t *testing.T) {
	for _, spec := range []struct {
		artifact oracleArtifact
		entries  []string
	}{
		{oracleArtifact{Path: "ghidra-fb-decode-01.json.gz", SHA256: "ed7190e0367d39c9a8666839807d6c5f1cc1d8ddac43489dbade98b979c98d4c", Bytes: 5446, Encoding: "gzip", Decoded: "c8353dd73d0118c7c3ec9cb643943d0c59c1adebd988b40401663882003c3ce2"}, []string{"121e8b11", "121e8bd0", "1216f154", "121d6216"}},
		{oracleArtifact{Path: "ghidra-program-decode-02.json.gz", SHA256: "29ca76fbbb8e12a23439dc72879e11dab2493d946b2a537bf97cb14e8dda7559", Bytes: 18006, Encoding: "gzip", Decoded: "98d10a83377c6e1d4205b280610ca1cb064b019947d2b4db8a8875ed87820f7c"}, []string{"121d5677", "121d5c77", "1202284a"}},
		{oracleArtifact{Path: "ghidra-program-text-03.json.gz", SHA256: "07e4be0a1a8568c94b6fcb9c1a53fc7a5d80d808dedb041c2486d0de930ad4b1", Bytes: 4389, Encoding: "gzip", Decoded: "f403b88d84f40be44296482d173a9452ca75873970f5cbe609169930581d4758"}, []string{"121d62a8", "1216c2ae", "121d544a", "12013d74"}},
		{oracleArtifact{Path: "ghidra-br-parser-04.json.gz", SHA256: "016000ca384950698145406b4d1eaffa3c9879fc56d0a93b637e4517a32b38c6", Bytes: 8432, Encoding: "gzip", Decoded: "99503b26bc69fad24ae0b4b90ecae2be494e209922f34c024b94884a6c3030ad"}, []string{"1203140f"}},
	} {
		var proof struct {
			Engine, EngineVersion, SourceSHA256 string
			Functions                           []struct {
				Entry, Name, Pseudocode string
				DecompileCompleted      bool
				Instructions            []struct{ Address, Text string }
			}
		}
		if err := json.Unmarshal(loadOracleArtifact(t, spec.artifact), &proof); err != nil {
			t.Fatal(err)
		}
		if proof.Engine != "ghidra" || proof.EngineVersion != "12.1.4" || proof.SourceSHA256 != "a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141" {
			t.Fatal("static provenance changed")
		}
		var entries []string
		for _, function := range proof.Functions {
			if !function.DecompileCompleted || function.Name == "" || function.Pseudocode == "" || len(function.Instructions) == 0 {
				t.Fatal("incomplete API evidence")
			}
			entries = append(entries, function.Entry)
		}
		if !reflect.DeepEqual(entries, spec.entries) {
			t.Fatal("unexpected function exports", entries)
		}
	}
}

func TestRenderedBRTextDoesNotQualifyNativeEncoding(t *testing.T) {
	for _, spec := range []struct {
		name  string
		count int
	}{{"br-rendered-manifest.json", 72}, {"br-typed-manifest.json", 30}} {
		for _, row := range loadLibraryProbeResults(t, spec.name, spec.count, "fb-body-operands-v1") {
			if row.Accepted || len(row.Words) != 0 || row.EncodeResult != nil || len(row.OperandResults) == 0 {
				t.Fatal("unexpected BR encoding success", row.ID)
			}
			failed := false
			for _, result := range row.OperandResults {
				failed = failed || result != 0
			}
			if !failed {
				t.Fatal("BR form rejected without a failed operand", row.ID)
			}
		}
	}
}
