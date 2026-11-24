package compiler_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"slices"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/internal/fbcode"
)

// Rebuild the composite writer reference. The scalar CXT supplies the caller,
// pin layout and placement. Only the measured ST body and private size differ.
// This example does not exercise a CXP ST parser or the project version check.
func TestSTStringWriterReference(t *testing.T) {
	want, err := os.ReadFile("testdata/native-st-string-reference.n1")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(want)) != "95abccf8c1a2d4002e5240173ff641d91efadc3380875f3e3f7736d458bf1f55" {
		t.Fatal("native ST writer reference changed")
	}
	source, err := os.ReadFile("testdata/n1-source-invocation.cxt")
	if err != nil {
		t.Fatal(err)
	}
	for _, model := range []string{codec.CJ1MCPU21, codec.CJ1HCPU65} {
		t.Run(model, func(t *testing.T) {
			profile := codec.Profile{Model: model, TimerCounterMode: codec.TimerModeBCD}
			doc, err := cxp.Read(source)
			if err != nil {
				t.Fatal(err)
			}
			settings := imageSettings(t)
			body, err := compiler.CompileCXPResourceBody(profile, doc, "Resource[0]", settings.DefinitionNumbers, settings.NextEnable)
			if err != nil {
				t.Fatal(err)
			}
			defs, instances := body.FBPrefix.Definitions, body.FBPrefix.Instances
			if len(defs.Records) != 1 || len(instances.Records) != 1 || len(body.Calls) != 1 {
				t.Fatal("expected one linked instance")
			}
			// STCompiler emitted #MOVL #43004142 3,BR3 for Msg := 'ABC'.
			// The literal is stored as two successive big-endian character pairs.
			literal := []byte{'A', 'B', 'C', 0}
			move := []uint32{0xb3400141, 0x69400000 | uint32(binary.BigEndian.Uint16(literal[:2])),
				0x61400000 | uint32(binary.BigEndian.Uint16(literal[2:])), 0x5d3c3003}
			logic, err := codec.EncodeFBRelativeBody(profile, []codec.FBRelativeInstruction{
				{Mnemonic: "LD", Operands: []codec.FBRelativeOperand{{BaseRegister: 3, Offset: 2, Unit: "bit"}}},
				{Mnemonic: "OUT", Operands: []codec.FBRelativeOperand{{BaseRegister: 3, Offset: 3, Unit: "bit"}}},
			})
			if err != nil {
				t.Fatal(err)
			}
			def := defs.Definitions[0]
			entry, exit, err := fbcode.DefinitionEnvelope(8, def.BodyNumber)
			if err != nil {
				t.Fatal(err)
			}
			code := slices.Concat(entry, move, logic, exit)
			// Reuse the scalar definition metadata up to its body. The record
			// length and instruction stream are the only changed fields.
			record := slices.Clone(def.RecordWords[:len(def.RecordWords)-len(def.Body.Words)])
			record[2] = uint32(len(record) + len(code))
			defSection := slices.Concat(defs.Words[:defs.Records[0].OffsetWords], record, code)
			instanceSection := slices.Clone(instances.Words)
			const br3 = 3
			instanceSection[instances.Records[0].OffsetWords+3+br3] = (181 + 15) / 16
			layout := codec.N1HeaderLayout{DefinitionWords: len(defSection), InstanceWords: len(instanceSection), TaskBlockWords: len(body.TaskBlock.Words)}
			header, err := codec.EncodeN1Header(profile, settings.Options, layout, &settings.Memory)
			if err != nil {
				t.Fatal(err)
			}
			words := slices.Concat(header, body.TaskConfiguration.Words, defSection, instanceSection, body.TaskBlock.Words)
			var got bytes.Buffer
			if err := binary.Write(&got, binary.BigEndian, words); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got.Bytes(), want) {
				t.Fatal("reconstructed definition, instance and task differ from the native N1")
			}
		})
	}
}
