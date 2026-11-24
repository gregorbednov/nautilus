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
		appendPin := func(pin cxp.FBCallPin) {
			words, e := fbcode.ScalarTransferForPin(8, pin)
			if e != nil {
				t.Fatal(e)
			}
			body = append(body, words)
		}
		for _, pin := range plan.Inputs {
			appendPin(pin)
		}
		call, err := fbcode.InstanceCall(8, plan.InstanceNumber, plan.TaskID)
		if err != nil {
			t.Fatal(err)
		}
		body = append(body, call)
		for _, pin := range plan.Outputs {
			appendPin(pin)
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
