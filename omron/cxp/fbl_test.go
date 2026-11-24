package cxp

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestNew4FBLMatchesCX(t *testing.T) {
	source, err := os.ReadFile("testdata/new4-fbl-source.cxt")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/new4-cx.fbl")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Read(source)
	if err != nil {
		t.Fatal(err)
	}
	got, err := doc.BuildFBL("23 11 2026 2 2 13")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("FBL differs from the %d-byte CX file (generated %d bytes)", len(want), len(got))
	}
	withSourceTimestamp, err := doc.BuildFBL("")
	if err != nil {
		t.Fatal(err)
	}
	text, err := Explode(withSourceTimestamp, MaxSize)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(text, []byte(`Modified:="16 11 2026 15 12 36";`)) || bytes.Contains(text, []byte("TEST,W10.00")) {
		t.Fatal("source timestamp or symbol filtering differs")
	}
	programsOnly, err := doc.BuildProgramsOnlyFBL("23 11 2026 2 2 13")
	if err != nil {
		t.Fatal(err)
	}
	programsOnlyText, err := Explode(programsOnly, MaxSize)
	if err != nil {
		t.Fatal(err)
	}
	standardText, err := Explode(want, MaxSize)
	if err != nil {
		t.Fatal(err)
	}
	listNumbers := strings.NewReplacer("LIST_$#[3]", "LIST_$#[1]", "LIST_$#[4]", "LIST_$#[2]")
	if string(programsOnlyText) != listNumbers.Replace(string(standardText)) {
		t.Fatal("Programs-only archive must differ in list numbers only")
	}
	if _, err := doc.BuildFBL("invalid timestamp"); err == nil || !strings.Contains(err.Error(), "Modified timestamp") {
		t.Fatalf("invalid timestamp: %v", err)
	}
}
