package nbpanel

import "testing"

func TestLocalBitLampAndSwitchShareBinding(t *testing.T) {
	const (
		lamp   = 512
		button = 640
		shape  = 64
		out    = 320
	)
	image := make([]byte, button+switchCaptionAt)
	little.PutUint32(image[screenTableField:], 128)
	little.PutUint32(image[shape:], 2)
	little.PutUint32(image[lamp+32:], shape)
	little.PutUint32(image[lamp+36:], lamp+68)
	little.PutUint32(image[lamp+56:], lamp+60)
	little.PutUint32(image[button+28:], shape)
	little.PutUint16(image[button+60:], 2)
	little.PutUint32(image[button+86:], button+94)
	little.PutUint32(image[button+130:], button+138)
	screen := Screen{Width: 800, Height: 480, Components: []Component{
		{Type: bitLampComponent, Offset: lamp, Size: lampRecordSize},
		{Type: switchComponent, Offset: button, Size: switchCaptionAt},
	}}
	pkg := &Package{Screens: []Screen{screen}}
	bit := BitControl{X: 20, Y: 80, Width: 80, Height: 54, Index: 57}
	kind, record, err := encodeBitControl(pkg, image, screen, out, bit, nil)
	if err != nil {
		t.Fatal(err)
	}
	if kind != bitLampComponent || u32(record, 12) != 0xfe00 ||
		u32(record, 16) != bit.Index || u32(record, 32) != shape || u32(record, 40) != out+88 {
		t.Fatalf("invalid local lamp binding or graphic: %x", record)
	}
	bit.Caption = &Text{X: 8, Y: 10, Width: 60, Height: 32, Value: "Toggle", Size: 15}
	kind, record, err = encodeBitControl(pkg, image, screen, out, bit, nil)
	if err != nil {
		t.Fatal(err)
	}
	if kind != switchComponent || u32(record, 12) != 0xfe00 || u32(record, 48) != 0xfe00 ||
		u32(record, 16) != bit.Index || u32(record, 52) != bit.Index ||
		u32(record, 28) != shape || little.Uint16(record[94:]) != 2 ||
		u32(record, 36) != out+switchCaptionAt || u32(record, 130) != out+138 {
		t.Fatalf("invalid local switch binding or graphic: %x", record)
	}
}

func TestLocalBitArtworkUsesAbsoluteOffsetsInStateOrder(t *testing.T) {
	off := bmpFixture(24, 1, []byte{0, 0, 255, 0, 0, 255, 0, 0})
	on := bmpFixture(24, 1, []byte{0, 255, 0, 0, 255, 0, 0, 0})
	artwork, err := NewBitArtwork(off, on)
	if err != nil {
		t.Fatal(err)
	}
	const offset = 0x40180
	screen := Screen{Width: 800, Height: 480}
	bit := BitControl{X: 20, Y: 80, Width: 80, Height: 54, Index: 57, Artwork: artwork}
	buttonCaption := Text{X: 8, Y: 10, Width: 60, Height: 32, Value: "Toggle", Size: 15}
	for _, tc := range []struct {
		name    string
		kind    uint32
		caption *Text
		groupAt int
	}{
		{"lamp", bitLampComponent, nil, lampRecordSize},
		{"switch", switchComponent, &buttonCaption, switchCaptionAt + 2*(20+2*len("Toggle"))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bit.Caption = tc.caption
			kind, record, err := encodeBitControl(&Package{}, nil, screen, offset, bit, nil)
			if err != nil {
				t.Fatal(err)
			}
			if kind != tc.kind || u32(record, 28) != 0 || u32(record, 32) != offset+tc.groupAt {
				t.Fatalf("wrong raster graphic pointer in component %#x", kind)
			}
			group := record[tc.groupAt:]
			firstAt, secondAt := u32(group, 8)-offset, u32(group, 12)-offset
			if u32(group, 0) != 2 || u32(group, 4) != offset+tc.groupAt+8 ||
				firstAt != tc.groupAt+16 || secondAt != firstAt+len(artwork.states[0]) {
				t.Fatalf("wrong graphic state table: %x", group[:16])
			}
			checkBitmapPixels(t, record[firstAt:secondAt], []byte{0, 0xf8, 0, 0xf8}, nil)
			checkBitmapPixels(t, record[secondAt:], []byte{0xe0, 0x07, 0xe0, 0x07}, nil)
		})
	}
}
