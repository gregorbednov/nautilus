package cxp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportPreservesFBAndOriginal(t *testing.T) {
	b, err := os.ReadFile("testdata/cj1m.cxp")
	if err != nil {
		t.Fatal(err)
	}
	d, err := Read(b)
	if err != nil {
		t.Fatal(err)
	}
	s := d.Summary()
	graphic, fb, code := 0, 0, 0
	for _, r := range s.Rungs {
		code += len(r.Instructions)
		if len(r.Graphic) > 0 {
			graphic++
			if strings.Contains(r.Path, "FunctionBlocks") {
				fb++
			}
		}
	}
	if graphic != 7 || fb != 6 || code != 231 {
		t.Fatalf("unexpected fixture inventory: graphic=%d FB=%d instructions=%d", graphic, fb, code)
	}
	if len(s.Warnings) != 2 || !strings.Contains(s.Warnings[1], "CPU65") {
		t.Fatalf("settings conflict lost: %v", s.Warnings)
	}
	dest := filepath.Join(t.TempDir(), "lift")
	if err := Export(d, b, dest); err != nil {
		t.Fatal(err)
	}
	original, _ := os.ReadFile(filepath.Join(dest, "original.cxp"))
	text, _ := os.ReadFile(filepath.Join(dest, "source.cxt"))
	if !bytes.Equal(original, b) || !bytes.Equal(text, d.Source) {
		t.Fatal("source bytes changed")
	}
	tree, _ := os.ReadFile(filepath.Join(dest, "project.json"))
	if !json.Valid(tree) {
		t.Fatal("invalid JSON")
	}
	if !bytes.Contains(tree, []byte("Лифт")) {
		t.Fatal("Cyrillic comments were lost")
	}
	if err := Export(d, b, dest); err == nil {
		t.Fatal("overwrote existing import")
	}
	if _, err := os.Stat(filepath.Join(dest, "nautilus.yaml")); !os.IsNotExist(err) {
		t.Fatal("uncompiled project advertised as runnable")
	}
}

func TestRawListsAndMultilineStrings(t *testing.T) {
	src := []byte("CXProgVer:=\"1.8\";\nName:=\"first;\nsecond \"\"quote\"\"\";\nVariables:=\nBEGIN_LIST_$#[1]\n,Name,D1,BOOL,,,text;\nEND_LIST_$#[1];\n")
	d, err := Read(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Fields) != 3 || d.Fields[1].Text() != "first;\nsecond \"quote\"" || !strings.Contains(d.Fields[2].Raw, "D1") {
		t.Fatalf("%+v", d.Fields)
	}
}

func TestEncodingChoice(t *testing.T) {
	if _, err := Read(append([]byte{0xef, 0xbb, 0xbf}, []byte("CXProgVer:=\"1.8\";\n")...)); err != nil {
		t.Fatal(err)
	}
	src := append([]byte("CXProgVer:=\"1.8\";\nName:=\""), 0xC0)
	src = append(src, []byte("\";\n")...)
	d, err := ReadEncoding(src, "windows-1251")
	if err != nil || d.Fields[1].Text() != "А" {
		t.Fatalf("%+v %v", d, err)
	}
	d, err = ReadEncoding(src, "windows-1252")
	if err != nil || d.Fields[1].Text() != "À" {
		t.Fatalf("%+v %v", d, err)
	}
	if _, err := ReadEncoding(src, "utf-8"); err == nil {
		t.Fatal("accepted invalid UTF-8")
	}
}
