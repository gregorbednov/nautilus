package nbpanel

import (
	"bytes"
	"crypto/md5"
	"strings"
	"testing"
)

func TestBuildOfflineText(t *testing.T) {
	data := panelFixture()
	reference, err := ParsePackage(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	before := bytes.Clone(reference.data)
	result, err := reference.BuildOffline([]OfflineScreen{{ID: 0, Objects: []Object{
		Text{X: 8, Y: 4, Width: 120, Height: 42, Size: 18, RGB: 0xff8000, Value: "A\nБ"},
		Text{X: 12, Y: 62, Width: 220, Height: 32, Size: 14, RGB: 0xffffff, Value: "TEST"},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, reference.data) || len(result) != len(data) {
		t.Fatal("build modified the reference or changed the package size")
	}
	built, err := ParsePackage(bytes.NewReader(result))
	if err != nil {
		t.Fatal(err)
	}
	first := built.Screens[0].Components
	if len(first) != 2 || first[0].Type != textComponent || first[1].Type != textComponent {
		t.Fatalf("screen 0 components = %+v", first)
	}
	record := result[built.hmi+first[0].Offset:]
	if u32(record, 28) != 3 || little.Uint16(record[20:]) != 0xfc00 ||
		!bytes.Equal(record[32:38], []byte{0x41, 0, 0x0d, 0, 0x11, 0x04}) {
		t.Fatalf("wrong text record: %x", record[:40])
	}
	outside := Text{X: 799, Width: 2, Height: 1, Value: "A", Size: 12}
	if _, err := reference.BuildOffline([]OfflineScreen{{ID: 0, Objects: []Object{outside}}}); err == nil {
		t.Fatal("text outside the screen was accepted")
	}
}

func TestParsePackageRejectsDamagedData(t *testing.T) {
	const project = pkgHeaderSize + 12
	const image = project + projectHeadLen + len("NB.dat")
	tests := []struct {
		name string
		at   int
		want string
	}{
		{"runtime", pkgHeaderSize, "runtime entry"},
		{"model", project + 12, "unsupported panel model"},
		{"digest", project + integrityField + 2, "invalid compiled HMI digest"},
		{"checksum", image + integrityBytes, "invalid eView HMI length or checksum"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := panelFixture()
			data[tt.at] ^= 0xff
			_, err := ParsePackage(bytes.NewReader(data))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %q", err, tt.want)
			}
		})
	}
}

func TestOfflineNavigationRewritesReferences(t *testing.T) {
	data := panelFixture()
	project := pkgHeaderSize + 12
	hmi := project + projectHeadLen + len("NB.dat")
	image := data[hmi : hmi+integrityBytes+256]
	screen := u32(image, u32(image, screenTableField))
	at := screen + screenHeaderSize + componentSlotSize + 4
	const graphic = 64
	put := func(pos, value int) { little.PutUint32(image[pos:], uint32(value)) }
	little.PutUint16(image[screen+34:], 1)
	put(screen+36, navigationComponent)
	put(screen+40, at)
	put(graphic, 2)
	put(at+28, graphic)
	put(at+44, changeFrame)
	put(at+76, at+keyShapeAt)
	put(at+80, at+keyDetailAt)
	put(at+84, 2)
	put(at+92, 30)
	put(at+100, 0xfe00)
	put(at+120, at+128)
	put(16, int(hmiChecksum(image)))
	writeHMIIntegrity(data, image, project)
	pkg, err := ParsePackage(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	label := Text{X: 10, Y: 6, Width: 70, Height: 35, Value: "Go", Size: 18}
	key := Navigation{X: 40, Y: 60, Width: 90, Height: 50, Target: 0, Label: label}
	objects := []Object{label, key}
	result, err := pkg.BuildOffline([]OfflineScreen{{ID: 0, Objects: objects}})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParsePackage(bytes.NewReader(result))
	if err != nil {
		t.Fatal(err)
	}
	component := parsed.Screens[0].Components[1]
	next := component.Offset
	record := result[parsed.hmi+next:]
	if next == at || component.Type != navigationComponent || u32(record, 28) != graphic ||
		u32(record, 36) != next+keyCaptionAt || u32(record, 76) != next+keyShapeAt ||
		u32(record, 120) != next+128 || !bytes.Equal(record[keyCaptionAt+20:keyCaptionAt+24], []byte{'G', 0, 'o', 0}) {
		t.Fatalf("invalid navigation record at %d", next)
	}
	key.Target = 10
	objects[1] = key
	_, err = pkg.BuildOffline([]OfflineScreen{{ID: 0, Objects: objects}})
	if err == nil || !strings.Contains(err.Error(), "destination screen 10") {
		t.Fatalf("got %v, want missing destination", err)
	}
}

// panelFixture assembles a minimal structural fixture without Omron's runtime.
func panelFixture() []byte {
	const project = pkgHeaderSize + 12
	const name = project + projectHeadLen
	const hmi = name + len("NB.dat")
	const hmiLen = integrityBytes + 256
	const tail = hmi + hmiLen
	const screenTable = 128
	data := make([]byte, tail+28)
	put := func(at, value int) { little.PutUint32(data[at:], uint32(value)) }
	put(0, 1)
	put(8, project)
	put(12, project)
	copy(data[16:], "PKGEXT")
	put(22, 3)
	put(26, 3)
	for i := range 3 {
		entry, runtime := 30+i*16, pkgHeaderSize+i*4
		put(entry, i+1)
		put(entry+8, 4)
		put(entry+12, runtime)
		copy(data[runtime:], "\x7fELF")
	}
	put(project, project+12)
	put(project+4, tail)
	put(project+8, tail+16)
	put(project+12, modelIDNB10W)
	put(project+16, name)
	put(project+20, hmiLen)
	put(project+24, hmi)
	put(project+104, tail)
	copy(data[name:], "NB.dat")
	copy(data[hmi:], hmiMagic)
	put(hmi+8, 28)
	put(hmi+12, hmiLen)
	put(hmi+20, 24)
	put(hmi+32, screenTable)
	put(hmi+36, 512)
	put(hmi+screenTable, screenTable+4)
	screen := data[hmi+screenTable+4:]
	little.PutUint16(screen[4:], 799)
	little.PutUint16(screen[6:], 479)
	put(hmi+16, int(hmiChecksum(data[hmi:tail])))
	// All padding entries are zero. The digest is stored directly at the
	// destination indices in this fixture.
	for i, octet := range md5.Sum(data[hmi : hmi+integrityBytes]) {
		data[project+integrityField+digestDestination[i]] = octet
	}
	return data
}
