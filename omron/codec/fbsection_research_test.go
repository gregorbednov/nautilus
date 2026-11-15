package codec_test

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestSectionFieldLiteralReferenceCandidates(t *testing.T) {
	a := oracleArtifact{Path: "ghidra-section-fields-11.json.gz", SHA256: "14662fbd55b34129c6be436bb173dd3364f68767b21a40ffbae9993e49957b50", Bytes: 460, Encoding: "gzip", Decoded: "4672a5c88d10c896551bcdf3a2489548b49bae3ce9d7b364a42034b0e858f949"}
	var proof struct {
		Engine, EngineVersion, SourceSHA256, Limitations string
		Matches                                          []struct{ Displacement, MatchAddress string }
	}
	if err := json.Unmarshal(loadOracleArtifact(t, a), &proof); err != nil {
		t.Fatal(err)
	}
	if proof.Engine != "ghidra" || proof.EngineVersion != "12.1.4" || proof.SourceSHA256 != "a32ada50d87bc39697ca7c57337b8e64b14e59bca54d54791929cbe7db678141" || proof.Limitations == "" || len(proof.Matches) != 3 {
		t.Fatal("unqualified literal search provenance")
	}
	for i, want := range []string{"121b9ace", "121c5b07", "121cf706"} {
		if proof.Matches[i].Displacement != "2d7c" || proof.Matches[i].MatchAddress != want {
			t.Fatal("changed literal candidates")
		}
	}
}

type nextEnableInput struct {
	Index int   `json:"index"`
	Value int32 `json:"value"`
}
type nextEnableResult struct {
	Index    int      `json:"index"`
	Value    int32    `json:"value"`
	Returned int      `json:"returned"`
	Before   []uint32 `json:"before"`
	After    []uint32 `json:"after"`
	Cleared  []uint32 `json:"cleared"`
}

func TestNextEnableSectionStateAgainstNativeSetter(t *testing.T) {
	accepted, rejected := 0, 0
	for _, row := range loadLibraryProbeResults(t, "section-addresses-manifest.json", 74, "fb-section-addresses-v1") {
		p := row.NextEnable
		zeros := make([]uint32, 7)
		if !row.Accepted || !reflect.DeepEqual(p.Before, zeros) || !reflect.DeepEqual(p.Cleared, zeros) {
			t.Fatal("nondefault/uncleared state", row.ID)
		}
		expected := make([]uint32, 7)
		if p.Index >= 0 && p.Index <= 6 {
			accepted++
			expected[p.Index] = uint32(p.Value)
			if p.Returned != 1 {
				t.Fatal("valid setter rejected", row.ID)
			}
		} else {
			rejected++
			if p.Returned != 0 {
				t.Fatal("invalid index accepted", row.ID)
			}
		}
		if !reflect.DeepEqual(p.After, expected) {
			t.Fatal("wrong setter offset/readback", row.ID, p.After, expected)
		}
	}
	if accepted != 70 || rejected != 4 {
		t.Fatal("setter coverage changed", accepted, rejected)
	}
}
