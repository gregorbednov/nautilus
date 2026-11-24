package um_test

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/um"
)

func taskFixture(t *testing.T, filename string) ([]uint32, []string, []uint32) {
	t.Helper()
	b, err := os.ReadFile("testdata/" + filename)
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		WordsHex, Instructions []string
		Offsets                []uint32
	}
	if err = json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	var words []uint32
	for _, s := range f.WordsHex {
		n, err := strconv.ParseUint(s, 16, 32)
		if err != nil {
			t.Fatal(err)
		}
		words = append(words, uint32(n))
	}
	return words, f.Instructions, f.Offsets
}

// A synthetic envelope derived from the archived minimal native Probe task;
// only code, total length and task length change. It is not a live UM copy.
func taskImage(t *testing.T, code []uint32) *um.N1Image {
	t.Helper()
	b, err := os.ReadFile("../compiler/testdata/n1-no-fb-cj1h-h.n1")
	if err != nil {
		t.Fatal(err)
	}
	const start = 654
	out := slices.Clone(b[:start*4])
	for _, word := range code {
		out = binary.BigEndian.AppendUint32(out, word)
	}
	out = append(out, b[655*4:]...)
	binary.BigEndian.PutUint32(out[32*4:], uint32(len(out)/4))
	binary.BigEndian.PutUint32(out[(618+26)*4:], uint32(37+len(code)))
	im, err := um.ParseN1(out)
	if err != nil {
		t.Fatal(err)
	}
	return im
}

func TestCPU21WordCalls(t *testing.T) {
	words, want, offsets := taskFixture(t, "cj1m-task256-code.json")
	profile := codec.Profile{Model: codec.CJ1MCPU21, TimerCounterMode: codec.TimerModeBCD}
	report, err := taskImage(t, words).DecodeTasks(profile)
	if err != nil {
		t.Fatal(err)
	}
	task := report.Tasks[0]
	if !task.Complete || !task.DecodedPrefixRoundTripVerified || task.DecodedWords != 53 || len(task.Instructions) != 25 {
		t.Fatal(task)
	}
	for i, ins := range task.Instructions {
		end := uint32(len(words))
		if i+1 < len(offsets) {
			end = offsets[i+1]
		}
		if ins.CodeOffsetWords != offsets[i] || ins.ImageOffsetWords != task.CodeOffsetWords+offsets[i] ||
			ins.Instruction.Source != want[i] || !slices.Equal(ins.Words, words[offsets[i]:end]) {
			t.Fatalf("instruction %d: %+v", i, ins)
		}
	}
	corrupt := slices.Clone(words)
	corrupt[16] ^= 1 << 24 // change the address class of the first result
	for _, code := range [][]uint32{corrupt, words[:20]} {
		r, err := taskImage(t, code).DecodeTasks(profile)
		if err != nil {
			t.Fatal(err)
		}
		if r.AllTaskCodeDecoded || r.Tasks[0].DecodedWords != 1 || !slices.Equal(r.Tasks[0].OpaqueTail, code[1:]) {
			t.Fatal("invalid or partial invocation accepted", r)
		}
	}
}

func TestN1TaskDecodingCounterReadback(t *testing.T) {
	words, want, _ := taskFixture(t, "cj1h-counter-code.json")
	im := taskImage(t, words)
	before := im.Original()
	// Edits to exported diagnostic fields must not change source of truth.
	im.Tasks[0].Code[0] = 0xffffffff
	im.Tasks[0].OffsetWords = 0
	r, err := im.DecodeTasks(codec.Profile{Model: codec.CJ1HCPU65, TimerCounterMode: codec.TimerModeBCD})
	if err != nil {
		t.Fatal(err)
	}
	if !r.AllTaskCodeDecoded || r.SourceRestored || r.SymbolNamesRestored || r.HardwareQualified || len(r.Tasks) != 1 {
		t.Fatal("incorrect qualification", r)
	}
	task := r.Tasks[0]
	if !task.Complete || !task.DecodedPrefixRoundTripVerified || task.DecodedWords != 24 || task.CodeOffsetWords != 654 || len(task.Instructions) != 18 || len(task.OpaqueTail) != 0 {
		t.Fatal("incorrect coverage", task)
	}
	var got []string
	var roundTrip []uint32
	for _, ins := range task.Instructions {
		if ins.ImageOffsetWords != 654+ins.CodeOffsetWords || !slices.Equal(ins.Words, words[ins.CodeOffsetWords:int(ins.CodeOffsetWords)+len(ins.Words)]) {
			t.Fatal("incorrect spans")
		}
		got = append(got, ins.Instruction.Source)
		encoded, err := codec.EncodeInstruction(codec.Profile{Model: codec.CJ1HCPU65, TimerCounterMode: codec.TimerModeBCD}, ins.Instruction)
		if err != nil {
			t.Fatal(err)
		}
		roundTrip = append(roundTrip, encoded...)
	}
	if !reflect.DeepEqual(got, want) || !slices.Equal(words, roundTrip) || !slices.Equal(im.Original(), before) {
		t.Fatal("source/code changed", got)
	}
}

func TestN1TaskDecodingStopsAtFirstUnknownOrTruncatedInstruction(t *testing.T) {
	for _, words := range [][]uint32{{0xe2040005, 0xdeadbeef, 0xe2040004, 0xbb800001}, {0xe2040005, 0xbb4000d0}} {
		r, err := taskImage(t, words).DecodeTasks(codec.Profile{Model: codec.CJ1HCPU65, TimerCounterMode: codec.TimerModeBCD})
		if err != nil {
			t.Fatal(err)
		}
		task := r.Tasks[0]
		if r.AllTaskCodeDecoded || task.Complete || task.DecodedWords != 1 || len(task.Instructions) != 1 || task.StopReason == "" || !slices.Equal(task.OpaqueTail, words[1:]) {
			t.Fatal("skipped unknown words or consumed operands", task)
		}
	}
}

func TestN1TaskDecodingReportLimitAndInvalidProfile(t *testing.T) {
	code := make([]uint32, 65538)
	for i := range code {
		code[i] = 0xa0000000
	}
	code[len(code)-1] = 0xbb800001
	r, err := taskImage(t, code).DecodeTasks(codec.Profile{Model: codec.CJ1HCPU65, TimerCounterMode: codec.TimerModeBCD})
	if err != nil || r.AllTaskCodeDecoded || r.Tasks[0].DecodedWords != 65536 || len(r.Tasks[0].OpaqueTail) != 2 || !strings.Contains(r.Tasks[0].StopReason, "limit") {
		t.Fatal("unbounded report", err)
	}
	for _, p := range []codec.Profile{{}, {Model: codec.CJ1HCPU65}, {Model: codec.CJ1HCPU65, TimerCounterMode: codec.TimerModeBinary}} {
		if out, err := taskImage(t, []uint32{0xbb800001}).DecodeTasks(p); out != nil || err == nil {
			t.Fatal("invalid profile accepted")
		}
	}
	var nilImage *um.N1Image
	if out, err := nilImage.DecodeTasks(codec.Profile{Model: codec.CJ1HCPU65, TimerCounterMode: codec.TimerModeBCD}); out != nil || err == nil {
		t.Fatal("nil image accepted")
	}
	im := taskImage(t, []uint32{0xbb800001})
	bytes := im.Original()
	binary.BigEndian.PutUint32(bytes[6*4:], 0x40000000)
	binaryImage, err := um.ParseN1(bytes)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := binaryImage.DecodeTasks(codec.Profile{Model: codec.CJ1HCPU65, TimerCounterMode: codec.TimerModeBCD}); out != nil || err == nil {
		t.Fatal("binary header accepted as BCD")
	}
}
