package fbcode_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestInvocationStaticEvidence(t *testing.T) {
	for _, spec := range []struct {
		file, hash string
		entries    []string
	}{
		{"postamble-caller-ghidra.json", "a0444eeb24cacdf4699298d093c6329b2bc81122397a4351b59b7d81f02ed27a", []string{"121dd444", "1217170b", "1216f826"}},
		{"control-primitives-ghidra.json", "aa07db7103205fa7421acb537ccf1cff96cc2e59713b3eaf93e92e78f0f2f981", []string{"12175eb3", "12175e8a", "12175e61", "12171499", "1217166f"}},
		{"scalar-transfer-ghidra.json", "dfbc7f04c6174f512c92f83b45573f8a5474e7e00acb19422a747ac480300fdf", []string{"12175d65", "121749ea", "12173d43", "12176744", "1216b1be"}},
		{"scalar-types-ghidra.json", "46e4a7f08230cf40107144b5c3768d6b5197641e0cb74d640f5f574b17b018a8", []string{"12175f7c", "1217185d"}},
	} {
		data, err := os.ReadFile("testdata/" + spec.file)
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != spec.hash {
			t.Fatal("native invocation evidence hash changed")
		}
		var proof struct {
			Engine, EngineVersion, SourceSHA256 string
			Functions                           []struct {
				Entry, Name, Pseudocode string
				DecompileCompleted      bool
			}
		}
		if err := json.Unmarshal(data, &proof); err != nil {
			t.Fatal(err)
		}
		if proof.Engine != "ghidra" || proof.EngineVersion != "12.1.4" || proof.SourceSHA256 != "a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141" || len(proof.Functions) != len(spec.entries) {
			t.Fatal("unreviewed invocation provenance")
		}
		for i, f := range proof.Functions {
			if f.Entry != spec.entries[i] || !f.DecompileCompleted {
				t.Fatal("incomplete invocation evidence")
			}
			if f.Entry == "1217170b" && (!strings.Contains(f.Name, "AddtoInvoke_Postamble") || !strings.Contains(f.Pseudocode, "0x9000080e") || !strings.Contains(f.Pseudocode, "0xc0000000")) {
				t.Fatal("wrong invocation control branch")
			}
		}
	}
}
