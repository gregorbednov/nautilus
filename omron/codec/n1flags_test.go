package codec_test

import (
	"errors"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
)

func TestN1TaskFlagsMatchNativeOptionMatrix(t *testing.T) {
	for _, mode := range []codec.TimerCounterMode{codec.TimerModeBCD, codec.TimerModeBinary} {
		rows := loadLibraryProbeResults(t, "header-options-"+string(mode)+"-manifest.json", 32, "program-object-v1")
		seen := map[string]map[codec.N1TaskOptions]bool{}
		for _, row := range rows {
			t.Run(string(mode)+"/"+row.Device+"/"+row.ID, func(t *testing.T) {
				if row.ProgramObject.HeaderOptions == nil {
					t.Fatal("missing option readbacks")
				}
				options := *row.ProgramObject.HeaderOptions
				if seen[row.Device] == nil {
					seen[row.Device] = map[codec.N1TaskOptions]bool{}
				}
				if seen[row.Device][options] {
					t.Fatal("duplicate option combination")
				}
				seen[row.Device][options] = true
				model := codec.CJ1MCPU21
				if row.Device == "CJ1H-H" {
					model = codec.CJ1HCPU65
				}
				flags, err := codec.EncodeN1TaskFlags(codec.Profile{Model: model, TimerCounterMode: mode}, options)
				if err != nil {
					t.Fatal(err)
				}
				// The complete native file is checked: changing these options
				// must leave code, records, configuration and all other words intact.
				checkOwnedProgramImage(t, row, mode, flags)
			})
		}
		if len(seen) != 2 || len(seen["CJ1M"]) != 16 || len(seen["CJ1H-H"]) != 16 {
			t.Fatal("incomplete option matrix")
		}
	}
}

func TestN1TaskFlagsRequireExplicitQualifiedProfile(t *testing.T) {
	for _, profile := range []codec.Profile{{}, {Model: codec.CJ1MCPU21}, {Model: "CJ1M CPU22", TimerCounterMode: codec.TimerModeBCD}, {Model: codec.CJ1HCPU65, UnitVersion: "4.0", TimerCounterMode: codec.TimerModeBCD}, {Model: codec.CJ1MCPU21, TimerCounterMode: "guess"}} {
		if flags, err := codec.EncodeN1TaskFlags(profile, codec.N1TaskOptions{}); flags != 0 || !errors.Is(err, codec.ErrUnqualifiedProfile) {
			t.Fatal("unqualified flags", flags, err)
		}
	}
}
