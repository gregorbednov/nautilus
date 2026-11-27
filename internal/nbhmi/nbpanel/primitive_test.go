package nbpanel

import "testing"

func TestPrimitivePathsUseFrameRelativeCoordinates(t *testing.T) {
	screen := Screen{Width: 800, Height: 480}
	line := Primitive{Kind: "line", StrokeWidth: 1, Stroke: 0xff0000,
		Points: []Point{{522, 158}, {509, 158}}}
	kind, record, err := encodePrimitive(screen, line)
	if err != nil || kind != lineComponent || len(record) != 28 {
		t.Fatalf("invalid line type or length, kind %#x, bytes %d, err %v", kind, len(record), err)
	}
	expected := []uint16{508, 157, 522, 158, 1, 1, 0xf800, 0, 0, 2, 14, 1, 1, 1}
	for i, word := range expected {
		if got := little.Uint16(record[i*2:]); got != word {
			t.Fatalf("line word %d = %d, want %d", i, got, word)
		}
	}
	polygon := line
	polygon.Kind = "polygon"
	polygon.Points = []Point{{509, 153}, {502, 158}, {509, 162}}
	kind, record, err = encodePrimitive(screen, polygon)
	if err != nil || kind != polygonComponent || len(record) != 44 || u32(record, 28) != 3 {
		t.Fatalf("invalid polygon type or point count, kind %#x, bytes %d, err %v", kind, len(record), err)
	}
	polygon.Points[1].X = 800
	if _, _, err := encodePrimitive(screen, polygon); err == nil {
		t.Fatal("polygon point outside the screen was accepted")
	}
}

func TestPrimitiveFillAndRoundedRectangle(t *testing.T) {
	screen := Screen{Width: 800, Height: 480}
	black := uint32(0)
	rect := Primitive{Kind: "rect", X: 13, Y: 25, Width: 50, Height: 40, Fill: &black}
	kind, record, err := encodePrimitive(screen, rect)
	if err != nil || kind != rectangleComponent || len(record) != 32 {
		t.Fatalf("invalid rectangle type or length, kind %#x, bytes %d, err %v", kind, len(record), err)
	}
	if little.Uint16(record[8:]) != 0 || little.Uint16(record[16:]) != 1 || little.Uint16(record[20:]) != 0x0821 {
		t.Fatal("opaque black fill or disabled stroke was lost")
	}
	rect.Kind, rect.Radius, rect.Fill = "roundrect", 25, nil
	rect.Stroke, rect.StrokeWidth = 0x00ff00, 5
	_, record, err = encodePrimitive(screen, rect)
	if err != nil || little.Uint16(record[8:]) != 1 || little.Uint16(record[10:]) != 5 ||
		little.Uint16(record[12:]) != 0x07e0 || little.Uint16(record[16:]) != 0 ||
		little.Uint16(record[28:]) != 20 {
		t.Fatalf("invalid rounded outline: %x, err %v", record, err)
	}
}

func TestPrimitiveVerticalGradient(t *testing.T) {
	screen := Screen{Width: 800, Height: 480}
	start, end := uint32(0xff0000), uint32(0)
	rect := Primitive{Kind: "roundrect", X: 13, Y: 25, Width: 50, Height: 40, Radius: 8,
		Fill: &start, EndFill: &end, Stroke: 0x00ff00, StrokeWidth: 3}
	kind, record, err := encodePrimitive(screen, rect)
	if err != nil || kind != rectangleComponent || len(record) != 32 {
		t.Fatalf("invalid rectangle type or length, kind %#x, bytes %d, err %v", kind, len(record), err)
	}
	if little.Uint16(record[12:]) != 0x07e0 || little.Uint16(record[16:]) != 0x100 ||
		little.Uint16(record[20:]) != 0xf800 || little.Uint16(record[24:]) != 0x0821 ||
		little.Uint16(record[28:]) != 8 {
		t.Fatalf("wrong gradient fill, stroke or radius: %x", record)
	}
	rect.Kind, rect.Height = "rect", 1
	_, record, err = encodePrimitive(screen, rect)
	if err != nil || little.Uint16(record[16:]) != 1 || little.Uint16(record[20:]) != 0xf800 {
		t.Fatalf("wrong fill for a one-pixel-high gradient: %x, err %v", record, err)
	}
	rect.Fill = nil
	if _, _, err := encodePrimitive(screen, rect); err == nil {
		t.Fatal("gradient without a starting color was accepted")
	}
}
