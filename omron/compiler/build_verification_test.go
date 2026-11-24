package compiler_test

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
)

func TestImageBuildVerificationRequiresExactReproduction(t *testing.T) {
	source, err := os.ReadFile("testdata/n1-no-fb.cxt")
	if err != nil {
		t.Fatal(err)
	}
	image, err := os.ReadFile("testdata/n1-no-fb-cj1h-h.n1")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("testdata/n1-finalization-probe-settings.json")
	if err != nil {
		t.Fatal(err)
	}
	var settings compiler.N1ImageSettings
	if err = json.Unmarshal(raw, &settings); err != nil {
		t.Fatal(err)
	}
	profile := codec.Profile{Model: codec.CJ1HCPU65, TimerCounterMode: codec.TimerModeBCD}
	proof, err := compiler.VerifyCXPImageBuild(profile, source, "auto", "Resource[0]", settings, image)
	if err != nil {
		t.Fatal(err)
	}
	if !proof.Matches(profile.Model, image) || proof.Matches(codec.CJ1MCPU21, image) {
		t.Fatal("proof lost target binding")
	}
	report := proof.Report()
	if report.Method != "local-recompile-exact-bytes-v1" || report.ImageBytes != 2624 || report.ImageSHA256 != "598e01ac119b890c94f5495e267269fa9546aff32e130f481d68e1ed35601b84" || report.UnitVersion != "" || report.SourceSHA256 == "" || report.SettingsSHA256 == "" {
		t.Fatal("incomplete evidence", report)
	}
	// Caller edits and exported report mutation cannot rewrite the proof.
	saved := bytes.Clone(image)
	image[0] ^= 1
	source[0] ^= 1
	settings.DefinitionNumbers["bogus"] = 9
	report.Model = codec.CJ1MCPU21
	if proof.Matches(profile.Model, image) || !proof.Matches(profile.Model, saved) || proof.Report().Model != profile.Model {
		t.Fatal("proof aliases caller")
	}
	if _, err = compiler.VerifyCXPImageBuild(profile, source, "auto", "Resource[0]", settings, saved); err == nil {
		t.Fatal("changed source/settings accepted")
	}
	source[0] ^= 1
	delete(settings.DefinitionNumbers, "bogus")
	if _, err = compiler.VerifyCXPImageBuild(profile, source, "auto", "Resource[0]", settings, image); err == nil {
		t.Fatal("changed N1 accepted")
	}
	if _, err = compiler.VerifyCXPImageBuild(profile, source, "auto", "Resource[9]", settings, saved); err == nil {
		t.Fatal("wrong resource accepted")
	}
	profile.UnitVersion = "3.0"
	if _, err = compiler.VerifyCXPImageBuild(profile, source, "auto", "Resource[0]", settings, saved); err == nil {
		t.Fatal("unqualified unit version accepted")
	}
	var zero compiler.VerifiedImageBuild
	var nilProof *compiler.VerifiedImageBuild
	if zero.Matches(codec.CJ1HCPU65, saved) || nilProof.Matches(codec.CJ1HCPU65, saved) || nilProof.Report().Method != "" {
		t.Fatal("uninitialized proof accepted")
	}
}
