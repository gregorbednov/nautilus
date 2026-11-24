package cxp

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestExternalFBVariableIsNotAnInOutPort(t *testing.T) {
	doc := originalFBSource(t, "cj1h.cxp")
	list := fbTestField(t, doc, fbDefinition+".POUHeader.PouVarList")
	list.Raw += "\n3,SharedBit,0,,0,,0,,0,,0;"
	fbTestField(t, doc, fbDefinition+".POUHeader.PouVarCount").Value = "8"
	if got := doc.Summary().FBDeclarationFindings; len(got) != 0 {
		t.Fatalf("unexpected version finding for external variable: %+v", got)
	}
	if _, err := doc.SourceForFBDefinition(fbDefinition); err == nil || !strings.Contains(err.Error(), "SharedBit: external FB variable linking") {
		t.Fatalf("expected unsupported external link: %v", err)
	}
}

func TestFBDeclarationNativeConstraints(t *testing.T) {
	raw, err := os.ReadFile("testdata/native-fb-header-versions.json")
	if err != nil {
		t.Fatal(err)
	}
	var capture struct {
		SchemaVersion            int
		PHeadSHA256, KBaseSHA256 string
		Cases                    []struct {
			Model                                                             string
			Usage, TypeCode, StringSize, ReadResult, Empty, Declared, Cleared int
			Validation                                                        []struct {
				ST, Result int
				Message    string
			}
		}
	}
	if err := json.Unmarshal(raw, &capture); err != nil {
		t.Fatal(err)
	}
	if capture.SchemaVersion != 1 || capture.PHeadSHA256 != "463eb4402bb6fa00453a5d7a745d77ced6be454d77aa2a683eb67de7e38ba71c" ||
		capture.KBaseSHA256 != "8ab6abdd01a4928a3ec0a5f7d235e1b2a658a47a17e8407fe53ba538554b261f" {
		t.Fatal("unexpected COM capture provenance")
	}
	seen := map[string]bool{}
	for _, c := range capture.Cases {
		var fixture string
		switch c.Model {
		case "CJ1M CPU21":
			fixture = "cj1m.cxp"
		case "CJ1H-H CPU65":
			fixture = "cj1h.cxp"
		default:
			t.Fatal("unknown CPU", c.Model)
		}
		if c.Usage < 0 || c.Usage > 3 || (c.TypeCode != 0 && c.TypeCode != 2 && c.TypeCode != 15) ||
			c.ReadResult != 1 || c.Empty != 0 || c.Cleared != 0 || len(c.Validation) != 2 ||
			(c.TypeCode == 15 && c.StringSize != 16) || (c.TypeCode != 15 && c.StringSize != 0) {
			t.Fatalf("invalid declaration readback: %+v", c)
		}
		for _, check := range c.Validation {
			if check.ST != 0 && check.ST != 1 {
				t.Fatal("unknown language", check.ST)
			}
			key := fmt.Sprintf("%s/%d/%d/%d", c.Model, c.Usage, c.TypeCode, check.ST)
			if seen[key] {
				t.Fatal("duplicate observation", key)
			}
			seen[key] = true
			doc := originalFBSource(t, fixture)
			list := fbTestField(t, doc, fbDefinition+".POUHeader.PouVarList")
			fileUsage := c.Usage
			if c.Usage == 3 { // POUHeadDoc usage for In Out
				fileUsage = 4
			}
			row := fmt.Sprintf("%d,ProbeVar,%d,,0,,0,;", fileUsage, c.TypeCode)
			list.Raw += "\n" + row
			count := fbTestField(t, doc, fbDefinition+".POUHeader.PouVarCount")
			count.Value = "8"
			if check.ST == 1 {
				fbTestField(t, doc, strings.TrimSuffix(fbDefinition, ".FunctionBlock")+".FBType").Value = `"FBST"`
			}
			findings := doc.Summary().FBDeclarationFindings
			invalid := check.Result == 0
			needVersion := c.Declared == 1 && !invalid
			if check.Result != 0 && check.Result != 1 || invalid != (check.Message != "") || c.Declared != 0 && c.Declared != 1 {
				t.Fatal(key, "unexpected vendor result")
			}
			if !invalid && !needVersion {
				if len(findings) != 0 {
					t.Fatal(key, findings)
				}
				continue
			}
			if len(findings) != 1 {
				t.Fatal(key, findings)
			}
			f := findings[0]
			if f.Variable != "ProbeVar" || f.Path != fbDefinition+".POUHeader.PouVarList" ||
				f.Line != list.Line+2+strings.Count(list.Raw, "\n") || f.Usage != fileUsage || f.TypeCode != c.TypeCode ||
				(f.Issue != "") != invalid || (f.MinimumCJ1UnitVersion == "4.0") != needVersion {
				t.Fatal(key, f)
			}
			if check.ST == 0 {
				_, err := doc.SourceForFBDefinition(fbDefinition)
				if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("line %d: ProbeVar:", f.Line)) {
					t.Fatal(key, err)
				}
			}
		}
	}
	if len(seen) != 48 {
		t.Fatal("incomplete native declaration coverage", len(seen))
	}
}
