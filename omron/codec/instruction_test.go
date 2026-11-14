package codec_test

import (
	"encoding/binary"
	"errors"
	"reflect"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/il"
)

func TestCodecRejectsUnqualifiedProfilesAndForms(t *testing.T) {
	valid, err := il.Parse("LD W0.00")
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range []codec.Profile{{}, {Model: "CJ1M CPU22"}, {Model: codec.CJ1MCPU21, UnitVersion: "4.0"}} {
		if words, err := codec.EncodeInstruction(profile, valid); !errors.Is(err, codec.ErrUnqualifiedProfile) || len(words) != 0 {
			t.Fatal("qualified unknown profile")
		}
		if _, count, err := codec.DecodeInstruction(profile, []uint32{0xe205e000}); !errors.Is(err, codec.ErrUnqualifiedProfile) || count != 0 {
			t.Fatal("decoded unknown profile")
		}
	}
	profile := codec.Profile{Model: codec.CJ1MCPU21}
	for _, source := range []string{"@LD TR0", "%OUT W0.00", "@MOV(021) #0001 D0", "LD(999) W0.00", "END(999)", "END W0.00", "LD", "LD W0.00 W0.01", "LD Symbol", "LD CIO0.00", "LD H512.00", "OUT T0", "SET C0", "OUT A447.15"} {
		instruction, err := il.Parse(source)
		if err != nil {
			t.Fatal(err)
		}
		if words, err := codec.EncodeInstruction(profile, instruction); err == nil || len(words) != 0 {
			t.Fatalf("emitted unqualified code for %s", source)
		}
	}
	for _, words := range [][]uint32{nil, {0}, {0xb2400140}, {0xe2c5e000}, {0xe205a000}, {0xfa038400}, {0xfa03e000}} {
		if _, count, err := codec.DecodeInstruction(profile, words); err == nil || count != 0 {
			t.Fatalf("consumed unknown/invalid code %x", words)
		}
	}
}

func TestNativeAddressSemantics(t *testing.T) {
	for _, test := range []struct {
		source, canonical string
		area              codec.Area
		word              uint16
		bit               uint8
	}{
		{"w1.15", "W1.15", codec.Work, 1, 15}, {"W0", "W0.00", codec.Work, 0, 0},
		{"6143.15", "6143.15", codec.CIO, 6143, 15}, {"H511.15", "H511.15", codec.Holding, 511, 15},
		{"A959.15", "A959.15", codec.Auxiliary, 959, 15}, {"D32767.15", "D32767.15", codec.Data, 32767, 15},
		{"T4095", "T4095", codec.Timer, 4095, 0}, {"C4095", "C4095", codec.Counter, 4095, 0},
	} {
		address, err := codec.ParseBitAddress(test.source)
		if err != nil || address.Area != test.area || address.Word != test.word || address.Bit != test.bit || address.String() != test.canonical {
			t.Fatalf("%s: %+v/%v", test.source, address, err)
		}
	}
	for _, source := range []string{"", "-1.00", "W-1.00", "W512.00", "H512.00", "A960.00", "D32768.00", "T4096", "C4096", "W0.16", "W0.015", "W0.", "W0.-1", "T0.00", "C0.Q", "CIO0.00", "E0_1.00", "@D0"} {
		if _, err := codec.ParseBitAddress(source); !errors.Is(err, codec.ErrBitAddress) {
			t.Fatalf("accepted %q", source)
		}
	}
	invalid := codec.BitAddress{}
	if _, err := codec.ParseBitAddress(invalid.String()); err == nil {
		t.Fatal("invalid typed address formatted as valid native operand")
	}
}

func TestCJ1RejectsDirectDMBitInstructions(t *testing.T) {
	for _, model := range []string{codec.CJ1MCPU21, codec.CJ1HCPU65} {
		profile := codec.Profile{Model: model}
		for _, name := range []string{"LD", "LDNOT", "AND", "ANDNOT", "OR", "ORNOT", "OUT", "OUTNOT", "SET", "RSET"} {
			instruction, err := il.Parse(name + " D0.00")
			if err != nil {
				t.Fatal(err)
			}
			if words, err := codec.EncodeInstruction(profile, instruction); !errors.Is(err, codec.ErrUnqualifiedInstruction) || len(words) != 0 {
				t.Fatalf("emitted CJ2-only DM bit form: %s", name)
			}
		}
		for _, word := range []uint32{0xe2080000, 0xe2280000, 0xe2480000, 0xe2680000, 0xe2880000, 0xe2a80000, 0xfa080000, 0xfa280000, 0xf0480000, 0xf0680000} {
			if _, count, err := codec.DecodeInstruction(profile, []uint32{word}); !errors.Is(err, codec.ErrUnqualifiedInstruction) || count != 0 {
				t.Fatalf("decoded CJ2-only DM bit form %x", word)
			}
		}
		instruction, _ := il.Parse("MOV D0 D1")
		if _, err := codec.EncodeInstruction(profile, instruction); err != nil {
			t.Fatal("blocked valid DM word transfer", err)
		}
	}
}

func FuzzDecodeInstruction(f *testing.F) {
	for _, words := range [][]uint32{{0xe205e000}, {0xfa05e001}, {0xbb800001}, {0xb2400140}, {0xe205a000}, {0}, {0xb2400140, 0x61400001, 0x54010000}, {0xb2400140, 0x6000e000, 0x5400f000, 0xbb800001}, {0xbb4000c0, 0x5000e000, 0x65400010}, {0xbb4000c1, 0x5000efff, 0x6544ffff}, {0xbb4000d0, 0x5000f000, 0x6400b840}, {0xbb4000c0, 0x5000e000, 0x6540abcd}, {0xe3200000}, {0xe3400000}, {0xe3000000}, {0xb1000002}, {0xbb000003}, {0xe212200f}, {0xfa12200f}, {0xe2122010}, {0xe2522000}, {0xe2c5e000, 0xcb800000, 0xc2125002}, {0xe2c5e000, 0xcba00000, 0xc2725002}, {0xfac00000, 0xd445e000}, {0xfae00000, 0xd465e000}, {0xe2d20000, 0xcb800000, 0xc2525002}, {0xfac00000}, {0xe2c5e000, 0xcb800001, 0xc2125002}} {
		var data []byte
		for _, word := range words {
			data = binary.BigEndian.AppendUint32(data, word)
		}
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 64 {
			return
		}
		var words []uint32
		for len(data) >= 4 {
			words = append(words, binary.BigEndian.Uint32(data))
			data = data[4:]
		}
		for _, mode := range []codec.TimerCounterMode{"", codec.TimerModeBCD, codec.TimerModeBinary} {
			profile := codec.Profile{Model: codec.CJ1MCPU21, TimerCounterMode: mode}
			instruction, count, err := codec.DecodeInstruction(profile, words)
			if err != nil {
				if count != 0 {
					t.Fatal("failed decode consumed input")
				}
				continue
			}
			encoded, err := codec.EncodeInstruction(profile, instruction)
			if err != nil || count <= 0 || count > len(words) || !reflect.DeepEqual(encoded, words[:count]) {
				t.Fatal("decoder changed native container")
			}
		}
	})
}
