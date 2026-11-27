package nbpanel

import (
	"bytes"
	"strings"
	"testing"
)

func TestBuildOfflineExpandsComponentDirectory(t *testing.T) {
	data := panelArtworkFixture()
	project := u32(data, 8)
	hmiAt := u32(data, project+24)
	image := data[hmiAt : hmiAt+u32(data, project+20)]
	table := u32(image, screenTableField)
	first, second := table+8, table+128
	put := func(at, value int) { little.PutUint32(image[at:], uint32(value)) }
	copy(image[first:first+screenHeaderSize], image[table+4:table+4+screenHeaderSize])
	put(table, first)
	put(table+4, second)
	copy(image[second:second+screenHeaderSize], image[first:first+screenHeaderSize])
	little.PutUint16(image[second+26:], 1)
	little.PutUint16(image[second+34:], 1)
	put(second+36, navigationComponent)
	oldComponent := second + screenHeaderSize + componentSlotSize + 4
	put(second+40, oldComponent)
	// A retained component may link to data in its original record.
	put(oldComponent+36, oldComponent+88)
	put(oldComponent+88, 0xabcd)
	end := writeScreenFixtureMetadata(data, image)

	reference, err := ParsePackage(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	before := bytes.Clone(data)
	objects := make([]Object, 24)
	for i := range objects {
		objects[i] = Text{X: i * 4, Y: 20, Width: 60, Height: 30, Value: "A", Size: 16}
	}
	out, err := reference.BuildOffline([]OfflineScreen{{ID: 0, Objects: objects}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(reference.data, before) {
		t.Fatal("expanded build modified its reference")
	}
	expanded, err := ParsePackage(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	newImage := out[expanded.hmi:expanded.hmiEnd]
	newEnd := u32(newImage, screenEndField)
	if len(expanded.Screens[0].Components) != len(objects) ||
		u32(newImage, screenTableField) != len(image) ||
		u32(newImage, screenMetadataField) != newEnd+8 ||
		u32(newImage, newEnd) != newEnd+4 ||
		!bytes.Equal(newImage[newEnd+8:newEnd+32], image[end+8:end+32]) {
		t.Fatal("expanded screen or metadata directory was not rebuilt correctly")
	}
	retained := expanded.Screens[1].Components[0]
	pointer := u32(newImage, retained.Offset+36)
	if retained.Offset < len(image) || retained.Type != navigationComponent ||
		pointer != oldComponent+88 || u32(newImage, pointer) != 0xabcd {
		t.Fatal("expanded directory did not preserve the retained component's pointer")
	}
}

func TestBuildOfflineAddsSparseScreenIDs(t *testing.T) {
	data := panelArtworkFixture()
	project := u32(data, 8)
	start := u32(data, project+24)
	image := data[start : start+u32(data, project+20)]
	writeScreenFixtureMetadata(data, image)
	reference, err := ParsePackage(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	objects := []Object{Text{Width: 100, Height: 30, Value: "New screen", Size: 18}}
	out, err := reference.BuildOffline([]OfflineScreen{
		{ID: maxUserScreenID, Objects: objects}, {ID: 0, Objects: objects}, {ID: 11, Objects: objects},
	})
	if err != nil {
		t.Fatal(err)
	}
	built, err := ParsePackage(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if len(built.Screens) != maxUserScreenID+1 {
		t.Fatalf("got %d screen slots", len(built.Screens))
	}
	builtImage := out[built.hmi:built.hmiEnd]
	system := u32(builtImage, 24)
	settings := u32(builtImage, system+16)
	if u32(builtImage, settings+4) != maxUserScreenID || u32(builtImage, settings+8) != len(built.Screens) {
		t.Fatal("terminal screen settings did not grow with the directory")
	}
	if u32(builtImage, 28) != u32(image, 28) {
		t.Fatal("expanding the screen directory modified the adjacent HMI header field")
	}
	for _, id := range []int{11, 12, maxUserScreenID - 1, maxUserScreenID} {
		screen := built.Screens[id]
		wantObjects := 0
		if id == 11 || id == maxUserScreenID {
			wantObjects = 1
		}
		if int(screen.ID) != id || screen.Width != panelWidth || screen.Height != panelHeight ||
			len(screen.Components) != wantObjects {
			t.Fatalf("unexpected screen %d: %+v", id, screen)
		}
	}
	for _, id := range []int{-1, 7, maxUserScreenID + 1} {
		_, err := reference.BuildOffline([]OfflineScreen{{ID: 0, Objects: objects}, {ID: id, Objects: objects}})
		if err == nil || !strings.Contains(err.Error(), "expected a user screen ID") {
			t.Fatalf("screen %d: expected invalid ID, got %v", id, err)
		}
	}
}

func writeScreenFixtureMetadata(data, image []byte) int {
	const system, settings = 64, 84
	table := u32(image, screenTableField)
	count := (u32(image, table) - table) / 4
	little.PutUint32(image[24:], system)
	little.PutUint32(image[28:], 44)
	little.PutUint32(image[system+16:], settings)
	little.PutUint32(image[settings:], 16)
	little.PutUint32(image[settings+4:], uint32(count-1))
	little.PutUint32(image[settings+8:], uint32(count))
	end := u32(image, screenEndField)
	little.PutUint32(image[end:], uint32(end+4))
	little.PutUint32(image[screenMetadataField:], uint32(end+8))
	for i := range 6 {
		little.PutUint32(image[end+8+i*4:], uint32(end+32+i*4))
	}
	little.PutUint32(image[16:], hmiChecksum(image))
	writeHMIIntegrity(data, image, u32(data, 8))
	return end
}
