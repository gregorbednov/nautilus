package um_test

import (
	"encoding/binary"
	"os"
	"slices"
	"testing"

	"github.com/joyautomation/nautilus/omron/um"
)

func TestObservedServiceSumAndUninterpretedFields(t *testing.T) {
	raw, err := os.ReadFile("../compiler/testdata/n1-no-fb-cj1h-h.n1")
	if err != nil {
		t.Fatal(err)
	}
	image, err := um.ParseN1(raw)
	if err != nil {
		t.Fatal(err)
	}
	// Independent Python sum over this unchanged native fixture.
	r := image.ServiceFields()
	if !r.Available || r.Sum32FromWord6 != 0xfb1f5fd5 || r.StoredPairMatches || r.Blocks64FromImageLength != 41 || r.StoredBlockCountMatches || r.NativeAlgorithmConfirmed || r.WriteQualified {
		t.Fatal("incorrect observed relation", r)
	}
	// Controlled fixture stamps are for the diagnostic test only. They do not
	// implement or qualify the native writer's header preparation.
	binary.BigEndian.PutUint32(raw, 0x5fd5)
	binary.BigEndian.PutUint32(raw[4:], 0xfb1f)
	binary.BigEndian.PutUint32(raw[4*4:], 0x12345678)
	image, err = um.ParseN1(raw)
	if err != nil {
		t.Fatal(err)
	}
	before := slices.Clone(raw)
	r = image.ServiceFields()
	if !r.StoredPairMatches || r.Words[4] != 0x12345678 || !slices.Equal(image.Original(), before) {
		t.Fatal("third field interpreted or input changed", r)
	}
	if r.StoredBlockCountMatches {
		t.Fatal("arbitrary word4 accepted")
	}
	withCount := slices.Clone(raw)
	binary.BigEndian.PutUint32(withCount[16:], 41)
	countImage, err := um.ParseN1(withCount)
	if err != nil {
		t.Fatal(err)
	}
	if got := countImage.ServiceFields(); !got.StoredPairMatches || !got.StoredBlockCountMatches || got.NativeAlgorithmConfirmed || got.WriteQualified {
		t.Fatal("wrong observed count relation", got)
	}
	// A body change is structurally valid but must break the observed pair.
	binary.BigEndian.PutUint32(raw[654*4:], 0xa0000000)
	image, err = um.ParseN1(raw)
	if err != nil {
		t.Fatal(err)
	}
	if image.ServiceFields().StoredPairMatches {
		t.Fatal("missed body change")
	}
	// Stored fields are32-bit containers, not silently masked16-bit inputs.
	binary.BigEndian.PutUint32(before, 0x10005fd5)
	image, err = um.ParseN1(before)
	if err != nil {
		t.Fatal(err)
	}
	if image.ServiceFields().StoredPairMatches {
		t.Fatal("ignored unexpected upper bits")
	}
	var nilImage *um.N1Image
	if nilImage.ServiceFields().Available || (&um.N1Image{}).ServiceFields().Available {
		t.Fatal("empty image has service evidence")
	}
}
