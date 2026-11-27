package nbpanel

import (
	"bytes"
	"testing"
)

func TestBuildOfflineMovesLargeArtworkOutsideScreen(t *testing.T) {
	reference, err := ParsePackage(bytes.NewReader(panelArtworkFixture()))
	if err != nil {
		t.Fatal(err)
	}
	pixels := bytes.Repeat([]byte{0xe0, 0x07}, 32*32)
	alpha := bytes.Repeat([]byte{125}, 32*32)
	resource, err := compressBitmap(32, 32, pixels, alpha)
	if err != nil {
		t.Fatal(err)
	}
	bitmap := Bitmap{X: 10, Y: 20, Width: 60, Height: 60, resource: resource}
	lamp := BitControl{X: 80, Y: 20, Width: 60, Height: 60, Index: 57,
		Artwork: &BitArtwork{states: [2][]byte{resource, resource}}}
	out, err := reference.BuildOffline([]OfflineScreen{{ID: 0, Objects: []Object{bitmap, lamp}}})
	if err != nil {
		t.Fatal(err)
	}
	built, err := ParsePackage(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	old := reference.data[reference.hmi:reference.hmiEnd]
	hmi := out[built.hmi:built.hmiEnd]
	if u32(hmi, screenTableField) != u32(old, screenTableField) ||
		u32(hmi, screenEndField) != u32(old, screenEndField) ||
		!bytes.Equal(hmi[u32(old, screenEndField):len(old)], old[u32(old, screenEndField):]) {
		t.Fatal("moving artwork changed the screen directory or metadata")
	}
	first, second := built.Screens[0].Components[0], built.Screens[0].Components[1]
	bitmapAt, group := u32(hmi, first.Offset+8), u32(hmi, second.Offset+32)
	if first.Size != bitmapRecordSize || bitmapAt != len(old) || group != bitmapAt+len(resource) ||
		u32(hmi, group) != 2 || u32(hmi, group+4) != group+8 ||
		u32(hmi, group+8) != group+16 || u32(hmi, group+12) != group+16+len(resource) {
		t.Fatal("wrong pointers to expanded artwork")
	}
	if !bytes.Equal(hmi[bitmapAt:group], resource) ||
		!bytes.Equal(hmi[group+16:group+16+len(resource)], resource) ||
		!bytes.Equal(hmi[group+16+len(resource):], resource) {
		t.Fatal("expanded bitmap or artwork states were modified")
	}
	if _, err := built.suffixOffsets(out); err != nil {
		t.Fatalf("relocated package directory: %v", err)
	}
	beforeResources := u32(reference.data, reference.project+112)
	afterResources := u32(out, built.project+112)
	for _, field := range []int{8, 16} {
		size := u32(reference.data, beforeResources+field)
		before := u32(reference.data, beforeResources+field+4)
		after := u32(out, afterResources+field+4)
		if u32(out, afterResources+field) != size || after-before != built.hmiEnd-reference.hmiEnd ||
			!bytes.Equal(out[after:after+size], reference.data[before:before+size]) {
			t.Fatalf("relocating the suffix changed driver at resource field %d", field)
		}
	}
	if !bytes.Equal(out[u32(out, built.project+4):], reference.data[u32(reference.data, reference.project+4):]) {
		t.Fatal("relocating the suffix changed the package footer")
	}
}

// panelArtworkFixture adds the resource directories used by growth builds.
func panelArtworkFixture() []byte {
	data := panelFixture()
	project := u32(data, 8)
	logo := u32(data, project+104)
	resources := logo + 4
	first := resources + 96
	second := first + 4
	drivers := second + 4
	driver := drivers + 12
	module := driver + 4
	helper := module + 4
	archive := helper + 4
	tail := archive + 8
	data = append(data[:logo], make([]byte, tail+28-logo)...)
	for field, value := range map[int]int{
		4: tail, 8: tail + 16, 100: 4, 108: 1, 112: resources,
		120: drivers, 128: 4, 132: helper, 136: 8, 140: archive,
		144: 1, 148: drivers, 152: 4, 156: module,
	} {
		little.PutUint32(data[project+field:], uint32(value))
	}
	little.PutUint32(data[resources:], 3)
	little.PutUint32(data[resources+8:], 4)
	little.PutUint32(data[resources+12:], uint32(first))
	little.PutUint32(data[resources+16:], 4)
	little.PutUint32(data[resources+20:], uint32(second))
	copy(data[first:], "\x7fELF")
	copy(data[second:], "MZ\x01\x02")
	little.PutUint32(data[drivers:], 1)
	little.PutUint32(data[drivers+4:], 4)
	little.PutUint32(data[drivers+8:], uint32(driver))
	for _, at := range []int{driver, module, helper} {
		copy(data[at:], "\x7fELF")
	}
	copy(data[archive:], "EVUNCOM")
	copy(data[tail:], "footer data")
	return data
}
