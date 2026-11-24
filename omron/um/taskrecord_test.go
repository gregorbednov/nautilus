package um

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"testing"
)

func TestExtractN1TaskRecord(t *testing.T) {
	for _, file := range []string{"testdata/cj1h-baseline.n1", "testdata/cj1m-input-bit.n1"} {
		t.Run(file, func(t *testing.T) {
			source, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			image, err := ParseN1(source)
			if err != nil {
				t.Fatal(err)
			}
			if len(image.Tasks) == 0 {
				t.Fatal("fixture has no tasks")
			}
			saved := bytes.Clone(source)
			for _, task := range image.Tasks {
				record, err := ExtractN1TaskRecord(source, task.Number)
				if err != nil {
					t.Fatal(err)
				}
				start := int(task.OffsetWords) * 4
				want := saved[start : start+int(task.LengthWords)*4]
				if !bytes.Equal(record, want) {
					t.Fatal("record was re-encoded or has wrong boundaries")
				}
				if binary.BigEndian.Uint32(record)&0x3ff != uint32(task.Number) || binary.BigEndian.Uint32(record[0x1a*4:])*4 != uint32(len(record)) || binary.BigEndian.Uint32(record[len(record)-4:]) != 0xbb800f48 {
					t.Fatal("record wrapper lost")
				}
				record[0] ^= 0xff
				if !bytes.Equal(source, saved) {
					t.Fatal("record aliases source")
				}
				record, err = ExtractN1TaskRecord(source, task.Number)
				if err != nil {
					t.Fatal(err)
				}
				source[start] ^= 0xff
				if !bytes.Equal(record, want) {
					t.Fatal("source aliases returned record")
				}
				copy(source, saved)
				if _, err = ParseN1(record); !errors.Is(err, ErrN1Layout) {
					t.Fatal("task record incorrectly accepted as full N1")
				}
			}
		})
	}
}

func TestExtractN1TaskRecordPreservesOpaqueData(t *testing.T) {
	source := oracle(t)
	image, err := ParseN1(source)
	if err != nil {
		t.Fatal(err)
	}
	task := image.Tasks[0]
	start := int(task.OffsetWords) * 4
	// An opaque header word, a non-ASCII name byte and an unknown instruction
	// must survive extraction; this operation does not interpret or normalize IL.
	binary.BigEndian.PutUint32(source[start+12*4:], 0xdeadbeef)
	source[start+4+1] = 0xe9
	codeStart := start + 36*4
	if task.IndependentIRDR {
		codeStart += 8
	}
	binary.BigEndian.PutUint32(source[codeStart:], 0xfefefefe)
	saved := bytes.Clone(source)
	record, err := ExtractN1TaskRecord(source, task.Number)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(record, source[start:start+int(task.LengthWords)*4]) || !bytes.Equal(source, saved) {
		t.Fatal("opaque data changed")
	}
}

func TestExtractN1TaskRecordErrors(t *testing.T) {
	source := oracle(t)
	for _, number := range []uint16{0, 288, 65535} {
		if record, err := ExtractN1TaskRecord(source, number); record != nil || !errors.Is(err, ErrN1TaskNotFound) {
			t.Fatalf("task %d: got %v", number, err)
		}
	}
	bad := bytes.Clone(source)
	binary.BigEndian.PutUint32(bad[0x20*4:], 0)
	for _, data := range [][]byte{nil, source[:1320], bad} {
		if record, err := ExtractN1TaskRecord(data, 256); record != nil || !errors.Is(err, ErrN1Layout) || errors.Is(err, ErrN1TaskNotFound) {
			t.Fatalf("invalid image lost validation error: %v", err)
		}
	}
}
