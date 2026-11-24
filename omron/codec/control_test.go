package codec_test

import (
	"reflect"
	"testing"

	"github.com/joyautomation/nautilus/omron/internal/fbcode"
)

func TestInvocationControlBitsAgainstNativeMethods(t *testing.T) {
	accepted, rejected := 0, 0
	for _, row := range loadLibraryProbeResults(t, "control-manifest.json", 54, "control-bits-v1") {
		if !row.Accepted {
			if len(row.ControlStages) != 0 {
				t.Fatal("control methods ran on failed instruction")
			}
			rejected++
			continue
		}
		if len(row.ControlStages) != 2 || row.ControlStages[0].Operation != "ResetCIIflag" || row.ControlStages[1].Operation != "SetCCFlags" {
			t.Fatal("incomplete control-method sequence")
		}
		for index, stage := range row.ControlStages {
			if len(stage.Words) != len(row.Words) || !reflect.DeepEqual(stage.Words[1:], row.Words[1:]) || stage.Flags != row.FlagsBeforeControl {
				t.Fatal("control operation changed operand words/object flags")
			}
			want := row.Words[0] & 0xdfffffff
			if index == 1 {
				want |= 0x023e0000
			}
			if stage.Words[0] != want {
				t.Fatal("native control operation differs", row.ID, index)
			}
		}
		end := row.Mnemonic == "FBEND"
		words, err := fbcode.InvocationPostamble(8, row.Words, end)
		if err != nil {
			t.Fatal(err)
		}
		expected := row.ControlStages[0].Words
		if !end && expected[0]&0xc0000000 == 0x80000000 {
			expected = row.ControlStages[1].Words
		}
		if !reflect.DeepEqual(words, expected) {
			t.Fatal("Go invocation postamble differs from measured primitives", row.ID)
		}
		accepted++
	}
	if accepted != 50 || rejected != 4 {
		t.Fatal("control probe coverage changed", accepted, rejected)
	}
}

func TestInvocationSkeletonAgainstNativeCompilation(t *testing.T) {
	for _, row := range loadLibraryProbeResults(t, "invocation-manifest.json", 4, "invocation-skeleton-v1") {
		probe := row.Invocation
		if !row.Accepted || probe == nil || probe.GenerateResult != 1 || probe.FinishResult != 1 || len(probe.Before.Words) != 0 || probe.Before.Flags != 0x40 || probe.After.Flags != 0xc0 {
			t.Fatal("incomplete native invocation lifecycle")
		}
		pre, post, err := fbcode.InvocationSkeleton(8, probe.Option)
		if err != nil {
			t.Fatal(err)
		}
		for _, pair := range []struct {
			want [][]uint32
			got  []invocationInstruction
		}{{pre, probe.Preamble}, {post, probe.Epilogue}} {
			if len(pair.want) != len(pair.got) {
				t.Fatal("generated invocation list length changed")
			}
			for i := range pair.want {
				if !reflect.DeepEqual(pair.want[i], pair.got[i].Words) || pair.got[i].Flags&0x80 == 0 {
					t.Fatal("Go invocation recipe differs from native compiled list")
				}
			}
		}
		header, err := fbcode.InvocationHeader(8, len(pre))
		if err != nil || !reflect.DeepEqual(header, probe.After.Words) {
			t.Fatal("native deferred header differs", header, probe.After.Words, err)
		}
	}
}
