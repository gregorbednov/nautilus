package fbcode_test

import (
	"encoding/binary"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/internal/fbcode"
)

func TestDefinitionRecordBoundsAndIsolation(t *testing.T) {
	profile := codec.Profile{Model: codec.CJ1MCPU21, TimerCounterMode: codec.TimerModeBCD}
	entry, exit, err := fbcode.DefinitionEnvelope(8, 1024)
	if err != nil {
		t.Fatal(err)
	}
	code := append(entry, 0xe21f3000)
	code = append(code, exit...)
	before := append([]uint32(nil), code...)
	record, err := fbcode.AssembleDefinitionRecord(profile, 1023, strings.Repeat("N", 64), 65, 65, code)
	if err != nil || len(record) != 28 || record[0] != 1023 || record[2] != 28 || record[22] != 0x4141 || !reflect.DeepEqual(record[23:], code) {
		t.Fatal(record, err)
	}
	record[23] = 0
	if !reflect.DeepEqual(code, before) {
		t.Fatal("record aliases code")
	}
	for _, test := range []struct {
		number          int
		name            string
		inputs, outputs int
	}{{-1, "X", 1, 1}, {1024, "X", 1, 1}, {0, strings.Repeat("X", 65), 1, 1}, {0, "", 1, 1}, {0, "Имя", 1, 1}, {0, "X", 0, 1}, {0, "X", 1, 0}, {0, "X", 66, 1}, {0, "X", 1, 66}, {0, "X", -1, 1}} {
		if record, err := fbcode.AssembleDefinitionRecord(profile, test.number, test.name, test.inputs, test.outputs, code); record != nil || !errors.Is(err, fbcode.ErrDefinition) {
			t.Fatal("unqualified metadata admitted", record, err)
		}
	}
	for _, bad := range [][]uint32{nil, {0xb0400809, 0x65440800, 0xe21f3000, 0xbb800805, 0x80000000}, {0xb0400809, 0x65440400, 0, 0xbb800805, 0x80000000}, {0xb0400809, 0x65440400, 0xe21f3000, 0xbb800805, 0}} {
		if record, err := fbcode.AssembleDefinitionRecord(profile, 0, "X", 1, 1, bad); record != nil || !errors.Is(err, fbcode.ErrDefinition) {
			t.Fatal("malformed record code admitted", record, err)
		}
	}
	for _, id := range []int{-1, 2048, 65535} {
		if entry, exit, err := fbcode.DefinitionEnvelope(8, id); entry != nil || exit != nil || !errors.Is(err, fbcode.ErrDefinition) {
			t.Fatal(entry, exit, err)
		}
	}
}

func FuzzDefinitionRecordAssembly(f *testing.F) {
	f.Add([]byte{9, 8, 0x40, 0xb0, 0, 4, 0x44, 0x65, 0, 0x30, 0x1f, 0xe2, 5, 8, 0x80, 0xbb, 0, 0, 0, 0x80}, 0, "BLINK", 4, 2)
	f.Fuzz(func(t *testing.T, data []byte, number int, name string, inputs, outputs int) {
		if len(data) > 8192 || len(name) > 256 || len(data)%4 != 0 {
			t.Skip()
		}
		code := make([]uint32, len(data)/4)
		for i := range code {
			code[i] = binary.LittleEndian.Uint32(data[i*4:])
		}
		record, err := fbcode.AssembleDefinitionRecord(codec.Profile{Model: codec.CJ1MCPU21, TimerCounterMode: codec.TimerModeBCD}, number, name, inputs, outputs, code)
		if err != nil {
			if record != nil {
				t.Fatal("partial record")
			}
			return
		}
		if len(record) != 23+len(code) || record[2] != uint32(len(record)) || record[0] != uint32(number) || record[22] != uint32(inputs<<8|outputs) || !reflect.DeepEqual(record[23:], code) {
			t.Fatal("invalid serialized record")
		}
	})
}
