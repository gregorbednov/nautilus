package compiler_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestSectionSeparatorStaticEvidence(t *testing.T) {
	for _, spec := range []struct{ file, hash string }{
		{"section-marker-ghidra.json", "e60d084e77feabaf265d8e2d3c4ad8ad36f13d548a4a05936ea893a5dbf32e3a"},
		{"section-recognition-ghidra.json", "c7518a192b8bbbeaf3a95136e9cf5bb307e1370b940305debd7165fbdb3d838a"},
	} {
		data, err := os.ReadFile("testdata/" + spec.file)
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != spec.hash {
			t.Fatal("static evidence hash changed", spec.file)
		}
		var proof struct {
			Engine, EngineVersion, SourceSHA256, AnalysisMode string
			Functions                                         []struct {
				RequestedAddress, Name, Pseudocode string
				DecompileCompleted                 bool
			}
		}
		if err := json.Unmarshal(data, &proof); err != nil {
			t.Fatal(err)
		}
		if proof.Engine != "ghidra" || proof.EngineVersion != "12.1.4" || proof.SourceSHA256 != "a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141" || proof.AnalysisMode != "selected-functions; no whole-program auto-analysis" || len(proof.Functions) != 2 {
			t.Fatal("unreviewed evidence provenance")
		}
		for _, f := range proof.Functions {
			if !f.DecompileCompleted {
				t.Fatal("incomplete static evidence")
			}
		}
		if spec.file == "section-marker-ghidra.json" && (proof.Functions[0].RequestedAddress != "121cb520" || !strings.Contains(proof.Functions[0].Name, "GetSLForSectionSeperator")) {
			t.Fatal("wrong section factory evidence")
		}
	}
}
