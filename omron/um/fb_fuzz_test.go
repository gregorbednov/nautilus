package um

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
)

// Mutate the FB prefix while keeping the outer N1/task structure intact, so
// malformed records actually reach the new reader instead of the outer guard.
func FuzzN1FBPrefix(f *testing.F) {
	baseline, err := os.ReadFile("testdata/cj1h-baseline.n1")
	if err != nil {
		f.Fatal(err)
	}
	const start, end = 330 * 4, 496 * 4
	f.Add(append([]byte{}, baseline[start:end]...))
	f.Add([]byte{0xff, 0xff, 0xff, 0xff})
	f.Fuzz(func(t *testing.T, prefix []byte) {
		if len(prefix) > end-start {
			return
		}
		data := bytes.Clone(baseline)
		copy(data[start:end], prefix)
		image, err := ParseN1(data)
		if err != nil {
			t.Fatal("prefix mutation invalidated outer fixture", err)
		}
		fb, err := image.FunctionBlocks()
		if err != nil {
			if fb != nil {
				t.Fatal("partial prefix inventory")
			}
			return
		}
		check := func(offset uint32, header, code []uint32) {
			if offset < 330 || uint64(offset)+uint64(len(header))+uint64(len(code)) > 496 {
				t.Fatal("record escaped prefix")
			}
			for i, w := range append(append([]uint32{}, header...), code...) {
				if binary.BigEndian.Uint32(data[(int(offset)+i)*4:]) != w {
					t.Fatal("FB record bytes changed")
				}
			}
		}
		for _, def := range fb.Definitions {
			check(def.OffsetWords, def.Header, def.Code)
		}
		for _, instance := range fb.Instances {
			check(instance.OffsetWords, instance.Header, instance.Code)
		}
		if !bytes.Equal(image.Original(), data) {
			t.Fatal("prefix inspection changed opaque bytes")
		}
	})
}
