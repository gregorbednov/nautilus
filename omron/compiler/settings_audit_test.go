package compiler_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/cxp"
)

func TestSavedImageSettingsExposeOverridesAndUnimplementedStorage(t *testing.T) {
	for _, name := range []string{"cj1m.cxp", "cj1h.cxp"} {
		source, err := os.ReadFile("../cxp/testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		saved := bytes.Clone(source)
		report, err := compiler.AuditCXPImageSettings(profile(), source, "auto", "Resource[0]", imageSettings(t))
		if err != nil {
			t.Fatal(err)
		}
		states := map[string]string{}
		for _, c := range report.Checks {
			states[c.ID] = c.State
			if c.ID == "stack-address" && (c.Source != "D0" || c.Recipe != "AC0") {
				t.Fatal("lost explicit stack override", c)
			}
		}
		if states["bank-0"] != "passed" || states["bank-1"] != "passed" || states["bank-3"] != "passed" || states["bank-4"] != "passed" || states["bank-2"] != "unverified" || states["stack-address"] != "unverified" || states["stack-size-units"] != "unverified" {
			t.Fatal("invented defaults or mapping", states)
		}
		if report.StackSelection != "use-default" || states["stack-default-table"] != "passed" || report.SourceSHA256 != fmt.Sprintf("%x", sha256.Sum256(source)) || report.DeclarationRows != 7 || len(report.DeclarationValues) != 5 || len(report.SavedMemory) != 9 || report.SourceSettingsVerified || report.StorageInitialized || report.GlobalStorageAudited || report.HardwareQualified || report.WriteQualified || report.NetworkAccess || !bytes.Equal(source, saved) {
			t.Fatal("incorrect scope/qualification", report)
		}
		for _, v := range report.DeclarationValues {
			if v.Column5 != "FALSE" && v.Column5 != "0" {
				t.Fatal("changed declaration spelling", v)
			}
		}
	}
}
func TestSettingsAuditAgreementDoesNotQualifyRuntimeMapping(t *testing.T) {
	source, err := os.ReadFile("../cxp/testdata/cj1m.cxp")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := cxp.Read(source)
	if err != nil {
		t.Fatal(err)
	}
	source = bytes.Replace(doc.Source, []byte(`StartAddress:="D0"`), []byte(`StartAddress:="AC0"`), 1)
	source = bytes.Replace(source, []byte(`StartAddress:="AC0";`), []byte("UseDefault:=false;\nStartAddress:=\"AC0\";"), 1)
	report, err := compiler.AuditCXPImageSettings(profile(), source, "auto", "Resource[0]", imageSettings(t))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range report.Checks {
		if c.ID == "stack-address" {
			found = true
			if c.State != "passed" {
				t.Fatal("exact text not recognized")
			}
		}
	}
	if !found || report.SourceSettingsVerified || report.StorageInitialized {
		t.Fatal("text equality promoted into runtime proof")
	}
	quoted := bytes.Replace(source, []byte("0,TIMER_1,13,"), []byte("0,\"TIMER_1\",13,"), 1)
	if _, err = compiler.AuditCXPImageSettings(profile(), quoted, "auto", "Resource[0]", imageSettings(t)); err == nil {
		t.Fatal("unsupported quoted declaration metadata accepted")
	}
	duplicate := bytes.Replace(source, []byte(`NumRegisters:=5;`), []byte("NumRegisters:=5;\nNumRegisters:=5;"), 1)
	if _, err = compiler.AuditCXPImageSettings(profile(), duplicate, "auto", "Resource[0]", imageSettings(t)); err == nil {
		t.Fatal("duplicate settings accepted")
	}
	if _, err = compiler.AuditCXPImageSettings(profile(), source, "auto", "Resource[9]", imageSettings(t)); err == nil {
		t.Fatal("missing resource accepted")
	}
	if _, err = compiler.AuditCXPImageSettings(codec.Profile{Model: codec.CJ1MCPU21, UnitVersion: "3.0"}, source, "auto", "Resource[0]", imageSettings(t)); err == nil {
		t.Fatal("unqualified CPU accepted")
	}
}

func TestSettingsAuditExplicitAndDefaultStackSelection(t *testing.T) {
	data, err := os.ReadFile("../cxp/testdata/cj1m.cxp")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := cxp.Read(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"true", "1", "false", "0", "TRUE", "FALSE"} {
		source := bytes.Replace(doc.Source, []byte(`StartAddress:="D0";`), []byte("UseDefault:="+flag+";\nStartAddress:=\"D0\";"), 1)
		r, err := compiler.AuditCXPImageSettings(profile(), source, "auto", "Resource[0]", imageSettings(t))
		if err != nil {
			t.Fatal(err)
		}
		explicit := flag == "false" || flag == "0" || flag == "FALSE"
		if r.StackSelectionOrigin != "explicit-source-field" || r.StackCallCandidate == nil || r.StackCallCandidate.FirstSize != 64 || r.StackCallCandidate.TotalSize != 1536 || r.SourceSettingsVerified {
			t.Fatal("incorrect static branch", r)
		}
		if explicit && (r.StackSelection != "explicit" || r.StackCallCandidate.Address != "D0") || !explicit && (r.StackSelection != "use-default" || r.StackCallCandidate.Address != "AC0") {
			t.Fatal("UseDefault ignored", r)
		}
	}
	bad := bytes.Replace(doc.Source, []byte(`StartAddress:="D0";`), []byte("UseDefault:=maybe;\nStartAddress:=\"D0\";"), 1)
	if _, err := compiler.AuditCXPImageSettings(profile(), bad, "auto", "Resource[0]", imageSettings(t)); err == nil {
		t.Fatal("ambiguous UseDefault accepted")
	}
}

func TestSettingsAuditBR2UsesOnlyConditionalDefaultCandidate(t *testing.T) {
	data, err := os.ReadFile("../cxp/testdata/cj1m.cxp")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := cxp.Read(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, insert := range []string{"", `BR[2]:="DIFF8191,0";`, `BR[2]:="DIFF123,0";`} {
		source := bytes.Replace(doc.Source, []byte(`BR[1]:="C3072,1024";`), []byte("BR[1]:=\"C3072,1024\";\n"+insert), 1)
		r, err := compiler.AuditCXPImageSettings(profile(), source, "auto", "Resource[0]", imageSettings(t))
		if err != nil {
			t.Fatal(err)
		}
		states := map[string]string{}
		for _, c := range r.Checks {
			states[c.ID] = c.State
		}
		if r.BR2Selection != "initialized-default-candidate" || r.BR2DefaultCandidate == nil || *r.BR2DefaultCandidate != (codec.N1MemoryBase{Address: "DIFF8191", Size: 0}) || states["bank-2"] != "unverified" || states["bank-2-default-table"] != "passed" || states["fb-support-profile"] != "unverified" || r.FBSupportProfileVerified || r.SourceUseFunctionBlockFlag != "1" || r.SourceSettingsVerified {
			t.Fatal("saved/missing bank promoted into profile proof", r)
		}
	}
	settings := imageSettings(t)
	settings.Memory.Bases[2].Address = "DIFF8190"
	r, err := compiler.AuditCXPImageSettings(profile(), data, "auto", "Resource[0]", settings)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range r.Checks {
		if c.ID == "bank-2-default-table" && c.State != "blocked" {
			t.Fatal("different recipe BR2 accepted")
		}
	}
	source := bytes.Replace(doc.Source, []byte("UseFunctionBlockFlag:=1;"), []byte("UseFunctionBlockFlag:=0;"), 1)
	r, err = compiler.AuditCXPImageSettings(profile(), source, "auto", "Resource[0]", imageSettings(t))
	if err != nil || r.SourceUseFunctionBlockFlag != "0" || r.FBSupportProfileVerified || r.SourceSettingsVerified {
		t.Fatal("source flag inferred active register branch")
	}
}

func TestSettingsAuditNativeMasksMatchRetainedWorkerObservations(t *testing.T) {
	raw, err := os.ReadFile("testdata/native-profile-observations.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus struct {
		Origin, SourceEventLogSHA256        string
		HardwareQualified, UnitVersionKnown bool
		Events                              []struct {
			Kind, Device, CPU                       string
			Selector, FamilyMaskLow, FamilyMaskHigh uint32
		}
	}
	if err = json.Unmarshal(raw, &corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.HardwareQualified || corpus.UnitVersionKnown || len(corpus.Events) != 3 {
		t.Fatal("retained fixture scope changed")
	}
	for _, event := range corpus.Events {
		if event.Kind != "profile" {
			continue
		}
		model := codec.CJ1MCPU21
		if event.Device == "CJ1H-H" {
			model = codec.CJ1HCPU65
		}
		source, err := os.ReadFile("../cxp/testdata/cj1m.cxp")
		if err != nil {
			t.Fatal(err)
		}
		report, err := compiler.AuditCXPImageSettings(codec.Profile{Model: model, TimerCounterMode: codec.TimerModeBCD}, source, "auto", "Resource[0]", imageSettings(t))
		if err != nil {
			t.Fatal(err)
		}
		got := report.NativeProfileCandidate
		if got == nil || got.Selector != event.Selector || got.FamilyMaskLow != event.FamilyMaskLow || got.FamilyMaskHigh != event.FamilyMaskHigh || got.SourceEventLogSHA256 != corpus.SourceEventLogSHA256 || got.Origin != corpus.Origin || got.UnitVersionKnown || report.FBSupportProfileVerified || report.SavedCS2SupportVersion != "768" {
			t.Fatal("native observation promoted or transcribed incorrectly", got)
		}
		doc, err := cxp.Read(source)
		if err != nil {
			t.Fatal(err)
		}
		changed := bytes.Replace(doc.Source, []byte("CS2SupportVersion:=768;"), []byte("CS2SupportVersion:=65535;"), 1)
		report, err = compiler.AuditCXPImageSettings(codec.Profile{Model: model, TimerCounterMode: codec.TimerModeBCD}, changed, "auto", "Resource[0]", imageSettings(t))
		if err != nil {
			t.Fatal(err)
		}
		if report.SavedCS2SupportVersion != "65535" || report.NativeProfileCandidate.UnitVersionKnown || report.FBSupportProfileVerified || report.SourceSettingsVerified {
			t.Fatal("saved stamp became unit version")
		}
	}
}

func TestSettingsAuditLocalVersionStateIsRetainedNotHardware(t *testing.T) {
	raw, err := os.ReadFile("testdata/native-local-profile-state.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Origin, SourceEventLogSHA256                        string
		HardwareQualified, UnitVersionKnown, WriteQualified bool
		Events                                              []struct {
			Kind, Device, CPU                                      string
			LibraryDefaultVersionBytes, LibraryCurrentVersionBytes [3]uint8
			LibraryExtendedProtect                                 uint32
			HardwareUnitVersionKnown                               bool
		}
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Events) != 6 || fixture.HardwareQualified || fixture.UnitVersionKnown || fixture.WriteQualified {
		t.Fatal("bad fixture scope")
	}
	source, err := os.ReadFile("../cxp/testdata/cj1m.cxp")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := cxp.Read(source)
	if err != nil {
		t.Fatal(err)
	}
	profiles := 0
	for _, event := range fixture.Events {
		if event.Kind != "profile" {
			continue
		}
		profiles++
		model := codec.CJ1MCPU21
		if event.Device == "CJ1H-H" {
			model = codec.CJ1HCPU65
		}
		for _, stamp := range []string{"0", "768", "65535"} {
			input := bytes.Replace(doc.Source, []byte("CS2SupportVersion:=768;"), []byte("CS2SupportVersion:="+stamp+";"), 1)
			report, err := compiler.AuditCXPImageSettings(codec.Profile{Model: model, TimerCounterMode: codec.TimerModeBCD}, input, "auto", "Resource[0]", imageSettings(t))
			if err != nil {
				t.Fatal(err)
			}
			if report.SavedCS2SupportVersion != stamp {
				t.Fatal("source stamp control not applied")
			}
			state := report.NativeProfileCandidate.LibraryState
			if state == nil || state.DefaultVersionBytes != event.LibraryDefaultVersionBytes || state.CurrentVersionBytes != event.LibraryCurrentVersionBytes || state.ExtendedProtect != event.LibraryExtendedProtect || state.Origin != fixture.Origin || state.SourceEventLogSHA256 != fixture.SourceEventLogSHA256 {
				t.Fatal("retained state differs", state)
			}
			if state.HardwareUnitVersionKnown || event.HardwareUnitVersionKnown || report.NativeProfileCandidate.UnitVersionKnown || report.HardwareQualified || report.WriteQualified || report.FBSupportProfileVerified || report.SourceSettingsVerified || report.NetworkAccess {
				t.Fatal("local defaults promoted into qualification")
			}
			found := false
			for _, check := range report.Checks {
				if check.ID == "source-version-provenance" {
					found = true
					if check.State != "unverified" {
						t.Fatal("source stamp promoted")
					}
				}
			}
			if !found {
				t.Fatal("version provenance missing")
			}
			state.DefaultVersionBytes[0] = 99
			state.CurrentVersionBytes[0] = 99
			state.ExtendedProtect = 99
		}
	}
	if profiles != 2 {
		t.Fatal("missing model")
	}
}
