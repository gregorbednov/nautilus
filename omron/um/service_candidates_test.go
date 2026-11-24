package um

import (
	"encoding/binary"
	"os"
	"slices"
	"testing"
)

func TestReadbackBlockCountEvidenceDoesNotSelectPrefixRule(t *testing.T) {
	var rows []struct {
		ImageBytes   int
		ServiceWords [6]uint32
	}
	nativeSumAsset(t, "native-um-service-observations.json.gz", &rows)
	if len(rows) != 4 {
		t.Fatal("missing historical readbacks")
	}
	for _, row := range rows {
		candidates := blockCountCandidates(row.ImageBytes, row.ServiceWords[4])
		if len(candidates) != 4 {
			t.Fatal("missing hypotheses")
		}
		// All four observations fit both64-byte rules. They cannot establish
		// whether word4 counts the first24 service bytes, despite exact matches.
		for i, c := range candidates {
			if c.StoredMatches != (i < 2) || (i < 2 && c.Count != row.ServiceWords[4]) {
				t.Fatalf("historical observation unexpectedly distinguishes prefix rules: %+v", candidates)
			}
		}
	}
}

func TestServiceBlockCountHypothesesPreserveRawImage(t *testing.T) {
	raw, err := os.ReadFile("../compiler/testdata/n1-no-fb-cj1h-h.n1")
	if err != nil {
		t.Fatal(err)
	}
	binary.BigEndian.PutUint32(raw[16:], 41)
	image, err := ParseN1(raw)
	if err != nil {
		t.Fatal(err)
	}
	saved := slices.Clone(raw)
	got := image.ServiceFields()
	want := []N1BlockCountCandidate{{64, 0, 41, true}, {64, 24, 41, true}, {256, 0, 11, false}, {256, 24, 11, false}}
	if !slices.Equal(got.BlockCountCandidates, want) || !got.StoredBlockCountMatches || got.NativeAlgorithmConfirmed || got.WriteQualified {
		t.Fatalf("wrong hypothesis scope: %+v", got)
	}
	got.BlockCountCandidates[0].Count = 0
	if !slices.Equal(image.ServiceFields().BlockCountCandidates, want) || !slices.Equal(image.Original(), saved) || !slices.Equal(raw, saved) {
		t.Fatal("diagnostics mutated or aliased image state")
	}
	var empty *N1Image
	if empty.ServiceFields().BlockCountCandidates != nil {
		t.Fatal("empty image has hypotheses")
	}
}
