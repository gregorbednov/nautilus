package um

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"testing"
)

func oracle(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/cj1h-baseline.n1")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestN1CompilerOracle(t *testing.T) {
	data := oracle(t)
	image, err := ParseN1(data)
	if err != nil {
		t.Fatal(err)
	}
	report := image.Summary()
	if report.SHA256 != "8a88c6e726542537787cea1419ccc42a1d203a37efb2304218c2a4e1687ce4f3" || report.ByteLength != 4592 {
		t.Fatal("compiler fixture provenance changed")
	}
	if report.DirectoryOffsetWords != 496 || report.UninterpretedPrefixBytes != 664 || report.HardwareQualified {
		t.Fatalf("unexpected image summary: %+v", report)
	}
	if len(image.Tasks) != 2 || image.Tasks[0].Number != 256 || image.Tasks[1].Number != 257 ||
		image.Tasks[0].NameASCII != "MotorDrive_LIFT_1" || image.Tasks[1].NameASCII != "NewProgram1" ||
		len(image.Tasks[0].Code) != 151 || len(image.Tasks[1].Code) != 139 {
		t.Fatalf("task inventory differs: %+v", report.Tasks)
	}
	// Numeric values established by the actual binary, not a serializer round trip.
	if image.Tasks[0].Code[0] != 0xe205e000 || image.Tasks[0].Code[1] != 0xfa05e001 {
		t.Fatal("wrong container byte order or body boundary")
	}
	for _, task := range image.Tasks {
		if task.Code[len(task.Code)-1] != 0xbb800001 {
			t.Fatal("END instruction lost")
		}
	}
	if !bytes.Equal(data, image.Original()) {
		t.Fatal("opaque prefix or other input data lost")
	}
	data[0] ^= 0xff
	copy := image.Original()
	copy[1] ^= 0xff
	if image.Summary().SHA256 != report.SHA256 {
		t.Fatal("caller can mutate retained image")
	}
}

func TestN1RejectsDamagedLayouts(t *testing.T) {
	tests := []struct {
		name         string
		index, value uint32
	}{
		{"length", 0x20, 0xffffffff}, {"marker", 0x21, 0},
		{"directory overflow", 0x22, 0xffffffff}, {"directory in header", 0x22, 1},
		{"auxiliary outside prefix", 0x23, 496},
		{"task pointer overflow", 496 + 256, 0xffffffff}, {"task in directory", 496 + 256, 1},
		{"task length overflow", 784 + 26, 0xffffffff}, {"task too short", 784 + 26, 36},
		{"wrong task", 784, 1}, {"wrong wrapper", 784 + 33, 0},
		{"wrong header size", 784 + 27, 0}, {"exit missing", 971, 0},
		{"missing task", 496 + 256, 0}, {"overlap", 496 + 257, 0x120},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := oracle(t)
			binary.BigEndian.PutUint32(data[test.index*4:], test.value)
			if _, err := ParseN1(data); !errors.Is(err, ErrN1Layout) {
				t.Fatalf("accepted damaged layout: %v", err)
			}
		})
	}
	data := oracle(t)
	for _, length := range []int{0, 1319, len(data) - 1, len(data) - 4} {
		if _, err := ParseN1(data[:length]); !errors.Is(err, ErrN1Layout) {
			t.Fatalf("accepted truncation %d", length)
		}
	}
}

func TestN1DifferentialCompilerOracle(t *testing.T) {
	baseline, err := ParseN1(oracle(t))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("testdata/cj1m-input-bit.n1")
	if err != nil {
		t.Fatal(err)
	}
	variant, err := ParseN1(data)
	if err != nil {
		t.Fatal(err)
	}
	if variant.Summary().SHA256 != "52d783d09159eb2be3a76aa56dcf899bfd7d313da727107bcabe0a416b28eeb5" {
		t.Fatal("variant provenance changed")
	}
	if len(variant.Tasks) != len(baseline.Tasks) {
		t.Fatal("variant lost tasks")
	}
	changed := 0
	for taskIndex, task := range variant.Tasks {
		if len(task.Code) != len(baseline.Tasks[taskIndex].Code) {
			t.Fatal("variant changed code size")
		}
		for codeIndex, value := range task.Code {
			if value == baseline.Tasks[taskIndex].Code[codeIndex] {
				continue
			}
			changed++
			if task.Number != 256 || codeIndex != 0 || value != 0xe205e00f {
				t.Fatal("unexpected changed instruction")
			}
		}
	}
	if changed != 1 {
		t.Fatalf("expected one changed container, got %d", changed)
	}
}

func TestN1CyclicWrapperAndCodePageName(t *testing.T) {
	const start = n1HeaderWords + n1DirectoryWords
	data := make([]byte, (start+40)*4)
	put := func(index int, value uint32) { binary.BigEndian.PutUint32(data[index*4:], value) }
	put(0x20, uint32(len(data)/4))
	put(0x21, 0x2a)
	put(0x22, n1HeaderWords)
	put(n1HeaderWords+5, n1DirectoryWords)
	put(start, 5)
	put(start+1, 0x00d2e5f1)
	put(start+26, 40)
	put(start+27, 32)
	put(start+33, 0xbbc00f43)
	put(start+34, 0x61440005)
	put(start+35, 0x65440000)
	put(start+36, 0x80400820)
	put(start+37, 0x653c0000)
	put(start+38, 0xbb800001)
	put(start+39, 0xbb800f48)
	image, err := ParseN1(data)
	if err != nil {
		t.Fatal(err)
	}
	task := image.Tasks[0]
	if !task.IndependentIRDR || len(task.Code) != 1 || task.NameASCII != "" || !bytes.Equal(task.NameBytes, []byte{0xd2, 0xe5, 0xf1}) {
		t.Fatalf("wrapper or code-page bytes lost: %+v", task)
	}
	if fb, err := image.FunctionBlocks(); err != nil || len(fb.Definitions) != 0 || len(fb.Instances) != 0 {
		t.Fatal("empty prefix misclassified", err)
	}
}

func FuzzParseN1(f *testing.F) {
	data, err := os.ReadFile("testdata/cj1h-baseline.n1")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(data)
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		image, err := ParseN1(data)
		if err == nil && !bytes.Equal(image.Original(), data) {
			t.Fatal("accepted image not preserved")
		}
		if err == nil {
			blocks, fbErr := image.FunctionBlocks()
			if fbErr != nil {
				if blocks != nil {
					t.Fatal("partial FB inventory on failure")
				}
				return
			}
			for _, def := range blocks.Definitions {
				if uint64(def.OffsetWords)+uint64(def.LengthWords) > uint64(image.DirectoryOffsetWords) || len(def.Header) != 23 || uint32(len(def.Code))+23 != def.LengthWords {
					t.Fatal("unbounded definition range")
				}
			}
			for _, instance := range blocks.Instances {
				if uint64(instance.OffsetWords)+uint64(instance.LengthWords) > uint64(image.DirectoryOffsetWords) || len(instance.Header) != 10 || uint32(len(instance.Code))+10 != instance.LengthWords {
					t.Fatal("unbounded instance range")
				}
			}
		}
	})
}
