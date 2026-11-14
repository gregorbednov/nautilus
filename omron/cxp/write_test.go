package cxp

import (
	"bytes"
	"os"
	"reflect"
	"testing"
)

func withoutLines(fields []Field) []Field {
	out := make([]Field, len(fields))
	copy(out, fields)
	for i := range out {
		out[i].Line = 0
		if out[i].Fields != nil {
			out[i].Fields = withoutLines(out[i].Fields)
		}
	}
	return out
}
func TestCXTCompleteTreeRoundTrip(t *testing.T) {
	for _, name := range []string{"cj1m", "cj1h"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile("testdata/" + name + ".cxp")
			if err != nil {
				t.Fatal(err)
			}
			d, err := Read(data)
			if err != nil {
				t.Fatal(err)
			}
			text, err := d.MarshalCXT()
			if err != nil {
				t.Fatal(err)
			}
			d2, err := ReadEncoding(text, d.Encoding)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(withoutLines(d.Fields), withoutLines(d2.Fields)) {
				t.Fatal("canonical CXT lost project fields")
			}
			packed, err := Pack(text)
			if err != nil {
				t.Fatal(err)
			}
			d3, err := ReadEncoding(packed, d.Encoding)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(text, d3.Source) {
				t.Fatal("CXP container altered text")
			}
		})
	}
}
func TestRungEditPreservesGraphicalFBs(t *testing.T) {
	data, _ := os.ReadFile("testdata/cj1m.cxp")
	d, err := Read(data)
	if err != nil {
		t.Fatal(err)
	}
	s := d.Summary()
	var path string
	for _, r := range s.Rungs {
		if len(r.Instructions) > 0 {
			path = r.Path
			break
		}
	}
	if err := d.SetRung(path, "LD L1_Key_start\nOR L1_Key_stop\nSET ON\n"); err != nil {
		t.Fatal(err)
	}
	text, err := d.MarshalCXT()
	if err != nil {
		t.Fatal(err)
	}
	edited, err := ReadEncoding(text, d.Encoding)
	if err != nil {
		t.Fatal(err)
	}
	for i, r := range edited.Summary().Rungs {
		if r.Path == path {
			if len(r.Instructions) != 3 || r.Instructions[1].Text != "OR L1_Key_stop" {
				t.Fatalf("bad edited rung %+v", r)
			}
			continue
		}
		old := s.Rungs[i]
		if len(old.Graphic) != len(r.Graphic) || len(old.Instructions) != len(r.Instructions) {
			t.Fatal("unrelated rung changed")
		}
	}
	for _, r := range s.Rungs {
		if len(r.Graphic) > 0 {
			if err := d.SetRung(r.Path, "LD 0.00"); err == nil {
				t.Fatal("silently replaced graphical rung")
			}
			break
		}
	}
	if err := d.SetRung("missing", "LD 0.00"); err == nil {
		t.Fatal("accepted missing rung")
	}
}
func TestPackAllBytesAndOverlapDecode(t *testing.T) {
	data := make([]byte, 256)
	for i := range data {
		data[i] = byte(i)
	}
	data = bytes.Repeat(data, 20)
	packed, err := Pack(data)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := Explode(packed, MaxSize)
	if err != nil || !bytes.Equal(plain, data) {
		t.Fatalf("pack/decode %v", err)
	}
	packed, err = Pack(nil)
	if err != nil {
		t.Fatal(err)
	}
	plain, err = Explode(packed, 10)
	if err != nil || len(plain) != 0 {
		t.Fatalf("empty %v", err)
	}
}
func TestDelimiterInjectionIsRefused(t *testing.T) {
	d, err := Read([]byte("CXProgVer:=\"1.8\";\nR[0]:=\nBEGIN\nSL:=\n$?St$Bk?_#[1]\nLD 0.00\n$?St$Bk?_#[1]\nEND;\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SetRung("R[0]", "LD 0.00\n$?St$Bk?_#[1]\nName:=\"changed\";"); err == nil {
		t.Fatal("accepted payload injection")
	}
}

func TestCXTSizeLimitIncludesCRLF(t *testing.T) {
	d := &Document{Encoding: "utf-8", Fields: []Field{{Name: "CXProgVer", Kind: "scalar", Value: `"1"`}, {Name: "A", Kind: "scalar", Value: `"2"`}}}
	unbounded, err := d.MarshalCXT()
	if err != nil {
		t.Fatal(err)
	}
	// LF-formatted content fits, but the emitted CRLF bytes exceed the limit.
	limit := len(unbounded) - 1
	if _, err := d.marshalCXT(limit); err == nil {
		t.Fatal("emitted oversized encoded CXT")
	}
	if data, err := d.marshalCXT(len(unbounded)); err != nil || !bytes.Equal(data, unbounded) {
		t.Fatal("rejected exact output limit", err)
	}
}
