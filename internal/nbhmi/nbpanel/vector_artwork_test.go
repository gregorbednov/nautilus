package nbpanel

import "testing"

func TestVectorArtworkPaintOrderAndStateOffsets(t *testing.T) {
	red, blue := uint32(0xff0000), uint32(0x0000ff)
	state := VectorArtwork{Width: 100, Height: 50, Shapes: []Primitive{
		{Kind: "rect", Width: 100, Height: 50, Fill: &red},
		{Kind: "rect", X: 8, Y: 8, Width: 20, Height: 20, Fill: &blue},
	}}
	artwork, err := NewVectorBitArtwork(state, VectorArtwork{Width: 100, Height: 50})
	if err != nil {
		t.Fatal(err)
	}
	const offset = 0x40100
	bit := BitControl{Width: 100, Height: 50, Index: 57, Artwork: artwork}
	_, record, err := encodeBitControl(&Package{}, nil, Screen{Width: 800, Height: 480}, offset, bit, nil)
	if err != nil {
		t.Fatal(err)
	}
	group := u32(record, 28) - offset
	if group != lampRecordSize || u32(record, 32) != 0 || u32(record, group) != 2 ||
		u32(record, group+4) != offset+group+8 {
		t.Fatalf("incorrect vector artwork group: %x", record)
	}
	off, on := u32(record, group+8)-offset, u32(record, group+12)-offset
	if off != group+16 || on != off+80 || u32(record, on+4) != 0 {
		t.Fatalf("incorrect vector state offsets or empty state: %x", record[group:])
	}
	first := int(little.Uint16(record[off+10:]))
	second := int(little.Uint16(record[off+14:]))
	if little.Uint16(record[off:]) != 100 || little.Uint16(record[off+2:]) != 50 ||
		u32(record, off+4) != 2 || first != 16 || second != 48 ||
		little.Uint16(record[off+first+20:]) != 0xf800 || little.Uint16(record[off+second+20:]) != 0x001f {
		t.Fatalf("incorrect vector geometry or paint order: %x", record[off:on])
	}
}
