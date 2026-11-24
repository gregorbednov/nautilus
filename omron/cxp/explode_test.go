package cxp

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestDCLReference(t *testing.T) {
	// Public PKWARE DCL vector documented by zlib's blast.
	input := []byte{0, 4, 0x82, 0x24, 0x25, 0x8f, 0x80, 0x7f}
	out, err := Explode(input, 100)
	if err != nil || string(out) != "AIAIAIAIAIAIA" {
		t.Fatalf("%q %v", out, err)
	}
	for n := 0; n < len(input); n++ {
		if _, err := Explode(input[:n], 100); err == nil {
			t.Fatalf("accepted truncation %d", n)
		}
	}
	if _, err := Explode(input, 12); err == nil {
		t.Fatal("accepted output exceeding limit")
	}
	if _, err := Explode(append(bytes.Clone(input), 0), 100); err == nil {
		t.Fatal("accepted trailing data")
	}
}
func TestProjects(t *testing.T) {
	files, err := filepath.Glob("testdata/*.cxp")
	if err != nil || len(files) != 2 {
		t.Fatalf("fixtures %v %v", files, err)
	}
	for _, name := range files {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			d, err := Read(data)
			if err != nil {
				t.Fatal(err)
			}
			s := d.Summary()
			if len(s.CPUs) != 1 || len(s.Rungs) == 0 || s.Mnemonics["END(001)"] != 2 {
				t.Fatalf("CPUs=%d rungs=%d END=%d", len(s.CPUs), len(s.Rungs), s.Mnemonics["END(001)"])
			}
			expected := "CPU21"
			if filepath.Base(name) == "cj1h.cxp" {
				expected = "CPU65"
			}
			if s.CPUs[0].Model != expected {
				t.Fatalf("model %q", s.CPUs[0].Model)
			}
			again, err := Read(d.Source)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(again.Source, d.Source) {
				t.Fatal("CXT source changed")
			}
			if len(again.Summary().Rungs) != len(s.Rungs) {
				t.Fatal("rungs lost")
			}
		})
	}
}
func TestCXTMalformed(t *testing.T) {
	for _, src := range []string{"", "CXProgVer:=\"1.8\";\nEND;", "CXProgVer:=\"1.8\";\nA:=\nBEGIN\n", "CXProgVer:=\"1.8\";\nSL:=\n$?St$Bk?_#[1]\nLD 0.00", "CXProgVer:=\"unterminated;"} {
		if _, err := Read([]byte(src)); err == nil {
			t.Fatalf("accepted %q", src)
		}
	}
}
func FuzzRead(f *testing.F) {
	f.Add([]byte("CXProgVer:=\"1.8\";\n"))
	f.Add([]byte{0, 4, 0x82, 0x24, 0x25, 0x8f, 0x80, 0x7f})
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 4096 {
			t.Skip()
		}
		_, _ = Read(b)
	})
}
