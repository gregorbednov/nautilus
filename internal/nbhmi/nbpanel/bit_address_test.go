package nbpanel

import (
	"bytes"
	"testing"
)

func TestCIOBitControlUsesTheReferenceDevice(t *testing.T) {
	image := make([]byte, 512)
	const system, table = 64, 96
	put := func(offset, value int) { little.PutUint32(image[offset:], uint32(value)) }
	put(24, system)
	put(screenTableField, len(image))
	put(system+4, table)
	put(system+8, table+20+4*32)
	put(table, 4)
	for i := range 4 {
		at := table + 20 + i*32
		put(table+4+i*4, at)
		image[at] = byte(i)
	}
	for i, kind := range []uint16{0x0207, 0x0207, 0x0008} {
		little.PutUint16(image[table+20+i*32+2:], kind)
	}
	const device = table + 20 + 3*32
	image[device+1] = 0x55
	little.PutUint16(image[device+2:], 9600)
	little.PutUint16(image[device+24:], 7)

	bmp := bmpFixture(24, 1, []byte{0, 0, 255, 0, 0, 255, 0, 0})
	artwork, err := NewBitArtwork(bmp, bmp)
	if err != nil {
		t.Fatal(err)
	}
	control := BitControl{Width: 80, Height: 50, Index: CIOBitLimit - 1, CIO: true, Artwork: artwork}
	screen := Screen{Width: 800, Height: 480}
	pkg := &Package{}
	const offset = 0x40180
	kind, record, err := encodeBitControl(pkg, image, screen, offset, control, nil)
	if err != nil {
		t.Fatal(err)
	}
	if kind != bitLampComponent || u32(record, 8) != 0x070300 ||
		u32(record, 12) != 0x3200 || u32(record, 16) != CIOBitLimit-1 ||
		u32(record, 20) != 0x01000001 {
		t.Fatalf("wrong CIO lamp address: %x", record[8:24])
	}
	address := record[8:24]
	control.Caption = &Text{X: 6, Y: 8, Width: 60, Height: 30, Value: "CIO", Size: 18}
	kind, record, err = encodeBitControl(pkg, image, screen, offset, control, nil)
	if err != nil {
		t.Fatal(err)
	}
	if kind != switchComponent || !bytes.Equal(record[8:24], address) || !bytes.Equal(record[44:60], address) {
		t.Fatal("CIO switch read and write addresses differ from the lamp")
	}
	control.Index = CIOBitLimit
	if _, _, err := encodeBitControl(pkg, image, screen, offset, control, nil); err == nil {
		t.Fatal("accepted an out-of-range CIO address")
	}
	control.Index = 0
	image[device+1] = 0x08
	if _, _, err := encodeBitControl(pkg, image, screen, offset, control, nil); err == nil {
		t.Fatal("accepted an unsupported PLC driver")
	}
}
