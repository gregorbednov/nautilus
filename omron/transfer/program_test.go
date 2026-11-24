package transfer_test

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"slices"
	"testing"

	"github.com/joyautomation/nautilus/omron/transfer"
)

func nativeImage(t *testing.T) []byte {
	t.Helper()
	b, e := os.ReadFile("../compiler/testdata/n1-no-fb-cj1h-h.n1")
	if e != nil {
		t.Fatal(e)
	}
	return b
}

func TestProgramPlanReconstructsExactOpaqueImage(t *testing.T) {
	for _, chunk := range []int{4, 8, 992, 996} {
		raw := nativeImage(t)
		// Unknown fields must survive planning unchanged.
		binary.BigEndian.PutUint32(raw, 0xdead)
		binary.BigEndian.PutUint32(raw[16:], 0x12345678)
		before := slices.Clone(raw)
		p, err := transfer.PlanProgramWrite(raw, chunk)
		if err != nil {
			t.Fatal(err)
		}
		if p.NetworkAccess || p.ImageModified || p.WireImageMappingQualified || p.WriteQualified || !p.RequiresProgramMode || p.Command != "0307" || len(p.Unresolved) == 0 || p.ImageSHA256 != fmt.Sprintf("%x", sha256.Sum256(before)) {
			t.Fatal("incorrect qualification")
		}
		var joined []byte
		for i, block := range p.Blocks {
			payload, err := hex.DecodeString(block.PayloadHex)
			if err != nil {
				t.Fatal(err)
			}
			last := i == len(p.Blocks)-1
			n := len(payload) - 8
			if binary.BigEndian.Uint16(payload) != 0xffff || binary.BigEndian.Uint32(payload[2:]) != uint32(len(joined)) || block.OffsetBytes != uint32(len(joined)) || n < 4 || n > chunk || n%4 != 0 || int(binary.BigEndian.Uint16(payload[6:])&0x7fff) != n || (binary.BigEndian.Uint16(payload[6:])&0x8000 != 0) != last || block.Final != last || block.DataBytes != n || block.DataSHA256 != fmt.Sprintf("%x", sha256.Sum256(payload[8:])) {
				t.Fatal("wrong block layout", i)
			}
			joined = append(joined, payload[8:]...)
		}
		if !slices.Equal(joined, before) || !slices.Equal(raw, before) {
			t.Fatal("image normalized or truncated")
		}
		if chunk == 996 && (len(p.Blocks) != 3 || p.Blocks[2].DataBytes != 632) {
			t.Fatal("write limit confused with992-byte read limit")
		}
		raw[0] ^= 255
		first, _ := hex.DecodeString(p.Blocks[0].PayloadHex)
		if !slices.Equal(first[8:], before[:p.Blocks[0].DataBytes]) {
			t.Fatal("plan shares caller memory")
		}
	}
}

func TestProgramPlanRejectsInvalidOrExcessiveInput(t *testing.T) {
	for _, n := range []int{-1, 0, 1, 3, 998, 1000} {
		if p, e := transfer.PlanProgramWrite(nativeImage(t), n); p != nil || e == nil {
			t.Fatal("invalid block size", n)
		}
	}
	if p, e := transfer.PlanProgramWrite([]byte("not N1"), 996); p != nil || e == nil {
		t.Fatal("invalid image")
	}
	base := nativeImage(t)
	large := make([]byte, 65537*4)
	copy(large, base)
	for i := 654; i < 65536; i++ {
		binary.BigEndian.PutUint32(large[i*4:], 0xa0000000)
	}
	binary.BigEndian.PutUint32(large[65535*4:], 0xbb800001)
	binary.BigEndian.PutUint32(large[65536*4:], 0xbb800f48)
	binary.BigEndian.PutUint32(large[32*4:], 65537)
	binary.BigEndian.PutUint32(large[(618+26)*4:], 65537-618)
	if p, e := transfer.PlanProgramWrite(large, 4); p != nil || e == nil {
		t.Fatal("unbounded block report")
	}
}
