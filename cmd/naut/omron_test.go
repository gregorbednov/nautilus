package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/cxp"
)

func TestOmronImportCLI(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "lift")
	if err := omronSource("import", []string{"-o", dest, "../../omron/cxp/testdata/cj1m.cxp"}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dest, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(b) || !strings.Contains(string(b), "CPU21") || !strings.Contains(string(b), "FBR[5]") {
		t.Fatal("incomplete import report")
	}
	if err := omronSource("import", []string{"-o", dest, "../../omron/cxp/testdata/cj1m.cxp"}); err == nil {
		t.Fatal("overwrote import")
	}
}

func TestOmronRewriteCLI(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "rung.txt")
	out := filepath.Join(dir, "edited.cxp")
	if err := os.WriteFile(source, []byte("LD L1_Key_stop\nSET ON\n"), 0600); err != nil {
		t.Fatal(err)
	}
	path := "Resource[0].Programs.Program[0].Sections.Sec[0].ProgramData.R[0]"
	args := []string{"-rung", path, "-source", source, "-o", out, "../../omron/cxp/testdata/cj1m.cxp"}
	if err := omronSource("rewrite", args); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	d, err := cxp.Read(data)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range d.Summary().Rungs {
		if r.Path == path {
			found = true
			if len(r.Instructions) != 2 || r.Instructions[0].Text != "LD L1_Key_stop" {
				t.Fatal("edit not applied")
			}
		}
	}
	if !found {
		t.Fatal("rung lost")
	}
	if err := omronSource("rewrite", args); err == nil {
		t.Fatal("overwrote existing artifact")
	}
	if err := writeNewOmronFile(out, []byte("replacement")); err == nil {
		t.Fatal("atomic writer overwrote existing file")
	}
	again, _ := os.ReadFile(out)
	if string(again) != string(data) {
		t.Fatal("failed rewrite changed artifact")
	}
}
func TestOmronInvalidConnectionBeforeDial(t *testing.T) {
	for _, args := range [][]string{
		{"-source-node", "256"}, {"-node", "-1"}, {"-network", "128"}, {"-timeout", "0"}, {"-transport", "unknown"}, {"-transport", "hostlink"}, {"-transport", "udp"},
	} {
		if err := omronRead("status", args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	if err := omronRead("read", []string{"D10", "0"}); err == nil {
		t.Fatal("accepted zero read")
	}
	if err := omronRead("read", []string{"D10", "1garbage"}); err == nil {
		t.Fatal("accepted malformed count")
	}
}
func TestOmronDoesNotAdvertiseExecutableImport(t *testing.T) {
	if !strings.Contains(omronUsage, "not implemented") || !strings.Contains(omronUsage, "not yet an executable") {
		t.Fatal("missing capability boundary")
	}
}
