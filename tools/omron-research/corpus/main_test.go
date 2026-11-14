package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/joyautomation/nautilus/omron/cxp"
)

func TestPreparePreservesBothProjects(t *testing.T) {
	const rung = "Resource[0].Programs.Program[0].Sections.Sec[0].ProgramData.R[0]"
	for _, cpu := range []string{"cj1m", "cj1h"} {
		t.Run(cpu, func(t *testing.T) {
			source := "../../../omron/cxp/testdata/" + cpu + ".cxp"
			before, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			base, err := cxp.Read(before)
			if err != nil {
				t.Fatal(err)
			}
			dest := filepath.Join(t.TempDir(), "cases")
			if err := prepare(source, dest, rung); err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(filepath.Join(dest, "manifest.json"))
			if err != nil {
				t.Fatal(err)
			}
			var m manifest
			if err := json.Unmarshal(b, &m); err != nil {
				t.Fatal(err)
			}
			if m.OriginalSHA256 != digest(before) || len(m.Cases) != len(seeds()) || len(m.CPUs) != 1 {
				t.Fatal("incomplete manifest")
			}
			original, _ := os.ReadFile(filepath.Join(dest, "original", "original.cxp"))
			if !reflect.DeepEqual(original, before) {
				t.Fatal("original changed")
			}
			for _, c := range m.Cases {
				packed, err := os.ReadFile(filepath.Join(dest, c.Name, "project.cxp"))
				if err != nil {
					t.Fatal(err)
				}
				text, err := os.ReadFile(filepath.Join(dest, c.Name, "project.cxt"))
				if err != nil {
					t.Fatal(err)
				}
				if digest(packed) != c.SHA256 || digest(text) != c.CXTSHA256 {
					t.Fatal("hash mismatch")
				}
				got, err := cxp.Read(packed)
				if err != nil {
					t.Fatal(err)
				}
				// Reversing the one edit must recover the complete original assignment
				// tree, including all graphical FB cells, settings and task metadata.
				var originalSource string
				for _, r := range base.Summary().Rungs {
					if r.Path == rung {
						for i, line := range r.Instructions {
							if i > 0 {
								originalSource += "\n"
							}
							originalSource += line.Text
						}
					}
				}
				if err := got.SetRung(rung, originalSource); err != nil {
					t.Fatal(err)
				}
				expected := canonicalTree(t, base)
				actual := canonicalTree(t, got)
				if !reflect.DeepEqual(expected, actual) {
					t.Fatalf("%s changed fields beyond the selected rung", c.Name)
				}
			}
			if err := prepare(source, dest, rung); err == nil {
				t.Fatal("overwrote corpus")
			}
			after, _ := os.ReadFile(source)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("source modified")
			}
		})
	}
}
func canonicalTree(t *testing.T, d *cxp.Document) []cxp.Field {
	t.Helper()
	b, err := d.MarshalCXT()
	if err != nil {
		t.Fatal(err)
	}
	got, err := cxp.ReadEncoding(b, d.Encoding)
	if err != nil {
		t.Fatal(err)
	}
	return got.Fields
}
func TestPrepareRejectsGraphicalRung(t *testing.T) {
	data, err := os.ReadFile("../../../omron/cxp/testdata/cj1m.cxp")
	if err != nil {
		t.Fatal(err)
	}
	d, err := cxp.Read(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range d.Summary().Rungs {
		if len(r.Graphic) > 0 {
			dest := filepath.Join(t.TempDir(), "cases")
			if err := prepare("../../../omron/cxp/testdata/cj1m.cxp", dest, r.Path); err == nil {
				t.Fatal("replaced graphic rung")
			}
			if _, err := os.Stat(dest); !os.IsNotExist(err) {
				t.Fatal("published partial corpus")
			}
			return
		}
	}
	t.Fatal("missing graphic fixture")
}
