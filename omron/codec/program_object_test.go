package codec_test

import (
	"encoding/binary"
	"encoding/hex"
	"reflect"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/il"
	"github.com/joyautomation/nautilus/omron/internal/fbcode"
	"github.com/joyautomation/nautilus/omron/um"
)

type programInstruction struct {
	Mnemonic string   `json:"mnemonic"`
	Operands []string `json:"operands"`
}

type programObjectResult struct {
	Definition    *programDefinitionResult `json:"definition"`
	Memory        *memoryReadback          `json:"memory"`
	HeaderOptions *codec.N1TaskOptions     `json:"headerOptions"`
	OwnerBound    bool                     `json:"ownerBound"`
	CompileResult int                      `json:"compileResult"`
	CIIEnabled    bool                     `json:"ciiEnabled"`
	Words         []uint32                 `json:"words"`
	File          *programFileResult       `json:"file"`
}

type programDefinitionResult struct {
	CompileResults []int    `json:"compileResults"`
	Messages       []string `json:"messages"`
	ErrorCount     int      `json:"errorCount"`
	WarningCount   int      `json:"warningCount"`
	Words          []uint32 `json:"words"`
}

type programFileResult struct {
	Returned   int    `json:"returned"`
	ErrorCount int    `json:"errorCount"`
	Path       string `json:"path"`
	ByteLength int    `json:"byteLength"`
	Hex        string `json:"fileHex"`
}

// This qualifies owned object construction and native writer geometry, not a
// CXP frontend or production image builder. The default KBase has no FB memory
// setup and different flags from the saved project captures.
func TestOwnedProgramNativeCompileAndFile(t *testing.T) {
	for _, row := range loadLibraryProbeResults(t, "program-object-manifest.json", 8, "program-object-v1") {
		t.Run(row.Device+"/"+row.ID, func(t *testing.T) {
			checkOwnedProgramImage(t, row, codec.TimerModeBCD, 0x08000000)
		})
	}
}

func checkOwnedProgramImage(t *testing.T, row nativeResult, mode codec.TimerCounterMode, flags uint32) {
	t.Helper()
	result := row.ProgramObject
	model := codec.CJ1MCPU21
	if row.Device == "CJ1H-H" {
		model = codec.CJ1HCPU65
	}
	profile := codec.Profile{Model: model, TimerCounterMode: mode}
	var prefix []uint32
	if result.Definition != nil {
		body, err := codec.EncodeFBRelativeBody(profile, []codec.FBRelativeInstruction{{Mnemonic: "LD", Operands: []codec.FBRelativeOperand{{BaseRegister: 3, Offset: 0, Unit: "bit"}}}, {Mnemonic: "OUT", Operands: []codec.FBRelativeOperand{{BaseRegister: 3, Offset: 1, Unit: "bit"}}}})
		if err != nil {
			t.Fatal(err)
		}
		entry, exit, err := fbcode.DefinitionEnvelope(8, 0)
		if err != nil {
			t.Fatal(err)
		}
		code := append(append(entry, body...), exit...)
		if !reflect.DeepEqual(code, result.Definition.Words) {
			t.Fatal("definition compiled code differs", result.Definition.Words)
		}
		record, err := fbcode.AssembleDefinitionRecord(profile, 0, "ProbeFB", 1, 1, code)
		if err != nil {
			t.Fatal(err)
		}
		prefix = append([]uint32{1, 0, 0, 0}, record...)
	}
	directory, taskStart := 330+len(prefix), 618+len(prefix)
	source := "END"
	if strings.HasPrefix(row.ID, "direct-") {
		source = "LD W0.00\nOUT W0.01\nEND"
	}
	var expected []uint32
	for _, line := range strings.Split(source, "\n") {
		instruction, err := il.Parse(line)
		if err != nil {
			t.Fatal(err)
		}
		words, err := codec.EncodeInstruction(codec.Profile{Model: model, TimerCounterMode: mode}, instruction)
		if err != nil {
			t.Fatal(err)
		}
		// GenerateInst clears CII; only a normal SetCIIFlag call changes
		// this state in the native experiment. Never patch object memory.
		if !result.CIIEnabled {
			words[0] &^= 0x20000000
		}
		expected = append(expected, words...)
	}
	if !reflect.DeepEqual(result.Words, expected) {
		t.Fatalf("program code %x != codec %x", result.Words, expected)
	}
	file := result.File
	if file.Returned != 1 || file.ErrorCount != 0 || file.ByteLength > 65536 {
		t.Fatal("native file failed")
	}
	raw, err := hex.DecodeString(file.Hex)
	if err != nil || len(raw) != file.ByteLength {
		t.Fatal("native file hex/length", err)
	}
	image, err := um.ParseN1(raw)
	if err != nil {
		t.Fatal(err)
	}
	if image.Flags != flags || image.DirectoryOffsetWords != uint32(directory) || len(image.Tasks) != 1 {
		t.Fatal("default native layout differs", image.Summary())
	}
	task := image.Tasks[0]
	if task.Number != 256 || task.NameASCII != "Probe" || task.Flags != 0 || task.IndependentIRDR || task.OffsetWords != uint32(taskStart) || task.LengthWords != uint32(len(expected)+37) || !reflect.DeepEqual(task.Code, expected) {
		t.Fatal("task wrapper/code differs", image.Summary())
	}
	// Account for every word, including unused configuration and
	// directory slots, rather than comparing only the decoded body.
	want := make([]uint32, 655+len(expected)+len(prefix))
	options := codec.N1TaskOptions{ExtendedTIMHold: true}
	if result.HeaderOptions != nil {
		options = *result.HeaderOptions
	}
	var memory *codec.N1FBMemorySettings
	if result.Memory != nil {
		value := settingsFromReadback(result.Memory)
		memory = &value
	}
	header, err := codec.EncodeN1Header(profile, options, codec.N1HeaderLayout{DefinitionWords: len(prefix), TaskBlockWords: 288 + 37 + len(expected)}, memory)
	if err != nil {
		t.Fatal(err)
	}
	copy(want, header)
	copy(want[330:], prefix)
	for i := 0; i < 288; i++ {
		flags := uint32(0x2000)
		if i >= 256 {
			flags = 0x1000
		}
		want[42+i] = uint32(i) | flags
	}
	want[42+256] = 0xd100
	want[directory+256] = 288
	want[taskStart] = 256
	want[taskStart+1] = 0x50726f
	want[taskStart+2] = 0x626500
	want[taskStart+26] = uint32(len(expected) + 37)
	want[taskStart+27] = 32
	copy(want[taskStart+32:], []uint32{0, 0xbbc00f43, 0x61440100, 0x65440000})
	copy(want[taskStart+36:], expected)
	want[len(want)-1] = 0xbb800f48
	if len(raw) != 4*len(want) {
		t.Fatal("unexpected file extent")
	}
	for i, w := range want {
		if got := binary.BigEndian.Uint32(raw[i*4:]); got != w {
			t.Fatalf("word %d: %08x != %08x", i, got, w)
		}
	}
}
