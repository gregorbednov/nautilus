package compiler_test

import (
	"encoding/binary"
	"os"
	"reflect"
	"testing"

	"github.com/joyautomation/nautilus/omron/compiler"
)

func TestTaskConfigurationMatchesEntireCapturedHeaderTable(t *testing.T) {
	for _, spec := range []struct{ source, image, model string }{{"cj1m.cxp", "cj1m-input-bit.n1", "CJ1M CPU21"}, {"cj1h.cxp", "cj1h-baseline.n1", "CJ1H-H CPU65"}} {
		doc := callDocument(t, spec.source)
		p := profile()
		p.Model = spec.model
		cfg, err := compiler.CompileCXPTaskConfiguration(p, doc, "Resource[0]")
		if err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile("../um/testdata/" + spec.image)
		if err != nil {
			t.Fatal(err)
		}
		want := make([]uint32, 288)
		for i := range want {
			want[i] = binary.BigEndian.Uint32(raw[(42+i)*4:])
		}
		if cfg.HeaderOffsetWords != 42 || !reflect.DeepEqual(cfg.Words, want) || cfg.Words[256] != 0xd100 || cfg.Words[257] != 0xd101 {
			t.Fatal("whole configuration table differs")
		}
		resource, err := compiler.CompileCXPResourceBody(p, doc, "Resource[0]", map[string]int{"BLINK": 0}, [7]uint32{})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(resource.TaskConfiguration.Words, want) {
			t.Fatal("resource configuration differs")
		}
		callField(t, doc, symbolProgram+".OpStart").Value = "0"
		disabled, err := compiler.CompileCXPTaskConfiguration(p, doc, "Resource[0]")
		if err != nil {
			t.Fatal(err)
		}
		want[256] = 0x9100
		if !reflect.DeepEqual(disabled.Words, want) {
			t.Fatal("startup clear changed unrelated configuration")
		}
	}
}

func TestTaskStartupMetadataDoesNotChangeCodeOrDirectory(t *testing.T) {
	doc := callDocument(t, "cj1m.cxp")
	before, err := compiler.CompileCXPResourceBody(profile(), doc, "Resource[0]", map[string]int{"BLINK": 0}, [7]uint32{})
	if err != nil {
		t.Fatal(err)
	}
	callField(t, doc, symbolProgram+".OpStart").Value = "0"
	after, err := compiler.CompileCXPResourceBody(profile(), doc, "Resource[0]", map[string]int{"BLINK": 0}, [7]uint32{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before.Words, after.Words) || after.TaskConfiguration.Words[256] != 0x9100 || after.TaskBlock.Tasks[0].Source.OperationStart != 0 {
		t.Fatal("startup setting changed task encoding or was ignored")
	}
	callField(t, doc, symbolProgram+".TaskID").Value = "287"
	sparse, err := compiler.CompileCXPTaskConfiguration(profile(), doc, "Resource[0]")
	if err != nil {
		t.Fatal(err)
	}
	if sparse.Words[256] != 0x1100 || sparse.Words[287] != 0x911f || sparse.Words[257] != 0xd101 || sparse.Words[0] != 0x2000 || sparse.Words[255] != 0x20ff {
		t.Fatal("absent/present task configuration defaults wrong")
	}
	for _, value := range []string{"-1", "2", "01"} {
		callField(t, doc, symbolProgram+".OpStart").Value = value
		if cfg, err := compiler.CompileCXPTaskConfiguration(profile(), doc, "Resource[0]"); cfg != nil || err == nil {
			t.Fatal("native bit masking leaked into source validation", value)
		}
	}
}
