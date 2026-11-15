package fbcode_test

import (
	"os"
	"reflect"
	"testing"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/il"
	"github.com/joyautomation/nautilus/omron/internal/fbcode"
	"github.com/joyautomation/nautilus/omron/um"
)

func TestObservedSourcePinOrderRebuildsCapturedCall(t *testing.T) {
	for _, fixture := range []struct{ source, image, model string }{
		{"cj1m.cxp", "cj1m-input-bit.n1", codec.CJ1MCPU21},
		{"cj1h.cxp", "cj1h-baseline.n1", codec.CJ1HCPU65},
	} {
		data, err := os.ReadFile("../../cxp/testdata/" + fixture.source)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := cxp.Read(data)
		if err != nil {
			t.Fatal(err)
		}
		const program = "Resource[0].Programs.Program[0]"
		plan, err := doc.PlanFBCall(program, program+".Sections.Sec[0].ProgramData.R[4]")
		if err != nil {
			t.Fatal(err)
		}
		var body [][]uint32
		appendPin := func(pin cxp.FBCallPin, usage fbcode.ParameterUsage) {
			// Fixture-specific transport controls, NOT a production mapping of
			// numeric CXP enums. Code1's signedness/type identity remains unknown;
			// all measured16-bit scalar copies use the same native MOV recipe.
			var kind fbcode.ScalarType
			switch pin.Declaration.TypeCode {
			case 0:
				kind = fbcode.Bool
			case 1:
				kind = fbcode.Word
			default:
				t.Fatal("unqualified fixture type")
			}
			words, e := fbcode.ScalarTransfer(8, kind, usage, pin.Value.Operand)
			if e != nil {
				t.Fatal(e)
			}
			body = append(body, words)
		}
		for _, pin := range plan.Inputs {
			appendPin(pin, fbcode.Input)
		}
		// The measured FBICALL literal recipe is still an explicit test control;
		// identifiers now come from the selected source instance/definition.
		body = append(body, []uint32{0xb2410806, 0x61440000 | uint32(plan.InstanceNumber), 0x65440000 | uint32(plan.TaskID), 0x80000000})
		for _, pin := range plan.Outputs {
			appendPin(pin, fbcode.Output)
		}
		frame, err := fbcode.AssembleInvocation(8, body, false)
		if err != nil {
			t.Fatal(err)
		}
		enable, err := il.Parse("LD " + plan.Enable.Operand)
		if err != nil {
			t.Fatal(err)
		}
		words, err := codec.EncodeInstruction(codec.Profile{Model: fixture.model}, enable)
		if err != nil {
			t.Fatal(err)
		}
		words = append(words, frame...)
		data, err = os.ReadFile("../../um/testdata/" + fixture.image)
		if err != nil {
			t.Fatal(err)
		}
		image, err := um.ParseN1(data)
		if err != nil || len(words) != 27 || !reflect.DeepEqual(words, image.Tasks[0].Code[25:52]) {
			t.Fatal("source pin ordering/bindings differ from full captured call", fixture, err)
		}
	}
}
