package nbpanel

import (
	"strings"
	"testing"
)

func TestNumberInputWritesLocalBindingAndLiteralLimits(t *testing.T) {
	const sample = 512
	const graphic = 64
	image := make([]byte, sample+numberRecordSize)
	little.PutUint32(image[screenTableField:], 128)
	little.PutUint32(image[graphic:], 1)
	ref := image[sample:]
	little.PutUint32(ref[28:], graphic)
	little.PutUint32(ref[148:], sample+numberShapeAt)
	little.PutUint32(ref[152:], sample+numberShapeEnd)
	little.PutUint32(ref[180:], 2)
	little.PutUint32(ref[188:], 30)
	little.PutUint32(ref[196:], 0xfe00)
	little.PutUint32(ref[216:], sample+224)
	pkg := &Package{Screens: []Screen{
		{Width: 800, Height: 480, Components: []Component{{Type: numberInputComponent, Offset: sample, Size: numberRecordSize}}},
		{},
		{},
		{Width: 235, Height: 295, Components: []Component{{Type: 0x305}}},
	}}
	input := NumberInput{X: 55, Y: 210, Width: 180, Height: 56, Word: 37, Min: 125, Max: 750, Decimals: 1}
	const offset = 300
	record, err := encodeNumberInput(pkg, image, pkg.Screens[0], offset, input)
	if err != nil {
		t.Fatal(err)
	}
	if len(record) != numberRecordSize || u32(record, 16) != input.Word || u32(record, 168) != input.Word ||
		u32(record, 12) != 0xff02 || u32(record, 164) != 0xff02 ||
		u32(record, 72) != input.Max || u32(record, 76) != input.Min || int(record[71]) != input.Decimals {
		t.Fatal("number input uses incorrect local addresses or limits")
	}
	if u32(record, 28) != graphic || u32(record, 148) != offset+numberShapeAt ||
		u32(record, 152) != offset+numberShapeEnd || u32(record, 216) != offset+224 {
		t.Fatal("number input uses incorrect graphic references")
	}
	input.Word = 8999
	if _, err := encodeNumberInput(pkg, image, pkg.Screens[0], offset, input); err == nil ||
		!strings.Contains(err.Error(), "two consecutive user LW registers") {
		t.Fatalf("expected system register rejection, got %v", err)
	}
}
