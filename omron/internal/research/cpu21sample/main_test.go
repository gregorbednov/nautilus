package main

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/joyautomation/nautilus/omron/um"
)

func TestRebuildAgainstArchivedCPU21(t *testing.T) {
	verifyArchivedCPU21(t, build())
}

func TestRebuildFromCXUpload(t *testing.T) {
	image, err := buildReadback("testdata/cpu21-upload.cxp")
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(image)); got != "3b70cab1f76a274072b0b82cc88dfa9cda37ffdd39fdc3ff4855b1b8794aa6d7" {
		t.Fatalf("CXP-derived compiler image hash %s", got)
	}
	verifyArchivedCPU21(t, image)
}

func verifyArchivedCPU21(t *testing.T, image []byte) {
	t.Helper()
	if len(image) != 4672 {
		t.Fatalf("rebuilt %d bytes", len(image))
	}
	parsed, err := um.ParseN1(image)
	if err != nil {
		t.Fatal(err)
	}
	fields := parsed.ServiceFields()
	// Calculate from the generated file. Both 64-byte hypotheses predict
	// the same count at this length. The archive hash is the independent oracle.
	for word, predicted := range map[int]uint32{0: fields.Sum32FromWord6 & 0xffff, 1: fields.Sum32FromWord6 >> 16, 4: fields.Blocks64FromImageLength} {
		slot := image[word*4:]
		if binary.BigEndian.Uint32(slot) != 0 {
			t.Fatalf("compiler set service word %d", word)
		}
		binary.BigEndian.PutUint32(slot, predicted)
	}
	const archiveSHA = "a4c0b45ba391fcd0cefb49bce52058168971e4aca1e6b29fe05937f80961c8cf"
	if got := fmt.Sprintf("%x", sha256.Sum256(image)); got != archiveSHA {
		t.Fatalf("reconstructed bytes with predicted service fields hash to %s", got)
	}
}
