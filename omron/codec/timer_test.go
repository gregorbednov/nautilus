package codec_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/il"
	"github.com/joyautomation/nautilus/omron/um"
)

func TestTimersAgainstLibraryOracle(t *testing.T) {
	encoded, rejected, outside := 0, 0, 0
	for _, mode := range []codec.TimerCounterMode{codec.TimerModeBCD, codec.TimerModeBinary} {
		for _, row := range loadLibraryResults(t, "timer-matrix-"+string(mode)+"-manifest.json", 420) {
			instruction, err := il.Parse(row.Mnemonic + " " + strings.Join(row.Operands, " "))
			if err != nil {
				t.Fatal(err)
			}
			profile := codec.Profile{Model: row.Device + " " + row.CPU, TimerCounterMode: mode}
			words, err := codec.EncodeInstruction(profile, instruction)
			indirect := strings.HasPrefix(row.Operands[1], "*") || strings.HasPrefix(row.Operands[1], "@")
			if !row.Accepted || indirect {
				if err == nil || len(words) != 0 {
					t.Fatalf("emitted invalid/unqualified %s %s", mode, instruction.Source)
				}
				if indirect {
					outside++
				} else {
					rejected++
				}
				continue
			}
			if err != nil || !reflect.DeepEqual(words, row.Words) {
				t.Fatalf("%s %s: got %x/%v, oracle %x", mode, instruction.Source, words, err, row.Words)
			}
			decoded, count, err := codec.DecodeInstruction(profile, append(append([]uint32{}, row.Words...), 0xbb800001))
			if err != nil || count != 3 {
				t.Fatalf("decode %s: %d/%v", instruction.Source, count, err)
			}
			again, err := codec.EncodeInstruction(profile, decoded)
			if err != nil || !reflect.DeepEqual(again, row.Words) {
				t.Fatal("timer round trip changed oracle words")
			}
			encoded++
		}
	}
	if encoded < 200 || rejected < 400 || outside != 48 {
		t.Fatalf("unexpected timer coverage: %d/%d/%d", encoded, rejected, outside)
	}
	t.Logf("timer oracle: encoded=%d rejected=%d outside direct forms=%d", encoded, rejected, outside)
}

func TestTimerProjectContainerAnchors(t *testing.T) {
	// These full-project captures corroborate numeric containers only: the same
	// containers can have binary-mode meanings. FB-relative operands stay opaque.
	for _, name := range []string{"cj1h-baseline.n1", "cj1m-input-bit.n1"} {
		data, err := os.ReadFile(filepath.Join("..", "um", "testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		image, err := um.ParseN1(data)
		if err != nil {
			t.Fatal(err)
		}
		for _, source := range []string{"TIM 0 #0100", "TIMH(015) 6 #0050", "TIMH(015) 3 #0016", "TIMH(015) 1 #0014", "TIMH(015) 5 #0050", "TIMH(015) 4 #0008", "TIMH(015) 2 #0020"} {
			instruction, err := il.Parse(source)
			if err != nil {
				t.Fatal(err)
			}
			words, err := codec.EncodeInstruction(codec.Profile{Model: codec.CJ1MCPU21, TimerCounterMode: codec.TimerModeBCD}, instruction)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, task := range image.Tasks {
				for i := 0; i+len(words) <= len(task.Code); i++ {
					if reflect.DeepEqual(task.Code[i:i+len(words)], words) {
						found = true
					}
				}
			}
			if !found {
				t.Fatalf("missing numeric timer anchor in %s: %s", name, source)
			}
		}
	}
}

func TestTimerModeIsExplicit(t *testing.T) {
	words := []uint32{0xbb4000c0, 0x5000e000, 0x65400010}
	profile := codec.Profile{Model: codec.CJ1MCPU21}
	if _, count, err := codec.DecodeInstruction(profile, words); !errors.Is(err, codec.ErrUnqualifiedProfile) || count != 0 {
		t.Fatal("guessed unknown timer mode")
	}
	for _, name := range []string{"TIM", "TIMX"} {
		instruction, _ := il.Parse(name + " 0 #0010")
		if code, err := codec.EncodeInstruction(profile, instruction); !errors.Is(err, codec.ErrUnqualifiedProfile) || len(code) != 0 {
			t.Fatal("encoded timer without mode")
		}
	}
	for _, mode := range []codec.TimerCounterMode{codec.TimerModeBCD, codec.TimerModeBinary} {
		profile.TimerCounterMode = mode
		instruction, count, err := codec.DecodeInstruction(profile, words)
		want := "TIM"
		if mode == codec.TimerModeBinary {
			want = "TIMX"
		}
		if err != nil || count != 3 || instruction.Mnemonic != want {
			t.Fatal("mode did not select timer interpretation")
		}
	}
	profile.TimerCounterMode = "automatic"
	if _, count, err := codec.DecodeInstruction(profile, words); !errors.Is(err, codec.ErrUnqualifiedProfile) || count != 0 {
		t.Fatal("accepted unknown mode")
	}
}

func TestTimerRejectsMalformedContainers(t *testing.T) {
	for _, mode := range []codec.TimerCounterMode{codec.TimerModeBCD, codec.TimerModeBinary} {
		profile := codec.Profile{Model: codec.CJ1MCPU21, TimerCounterMode: mode}
		for _, words := range [][]uint32{
			{0xbb4000c0}, {0xbb4000c0, 0x5000e000},
			{0xbb4000c0, 0x5000f000, 0x65400010}, // Counter slot for timer.
			{0xbb4000d0, 0x5000e000, 0x65400010}, // Timer slot for counter.
			{0xbb4000c0, 0x5000e000, 0x65410010}, // Reserved preset tag.
			{0xbb4000c0, 0x5000e000, 0x64810000}, // Unqualified indirect preset.
			{0xbb4000c0, 0x5000e000, 0x6400da00}, // Address gap.
		} {
			if _, count, err := codec.DecodeInstruction(profile, words); err == nil || count != 0 {
				t.Fatalf("consumed malformed timer %x", words)
			}
		}
	}
	profile := codec.Profile{Model: codec.CJ1MCPU21, TimerCounterMode: codec.TimerModeBCD}
	for _, preset := range []uint32{0x6540abcd, 0x65440010} {
		if _, count, err := codec.DecodeInstruction(profile, []uint32{0xbb4000c0, 0x5000e000, preset}); err == nil || count != 0 {
			t.Fatal("decoded binary literal in BCD")
		}
	}
}
