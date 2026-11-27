package nbpanel

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

const (
	textComponent       = 0x109
	navigationComponent = 0x305
	changeFrame         = 0x0201
	maxUserScreenID     = 32767
	panelWidth          = 800
	panelHeight         = 480
)

var errScreenCapacity = errors.New("exceeds the reference screen capacity")

type Text struct {
	X, Y, Width, Height int
	Value               string
	Size                int
	RGB                 uint32
}

type Navigation struct {
	X, Y, Width, Height int
	Target              int
	Label               Text
}

type Object interface{ offlineObject() }

func (Text) offlineObject()        {}
func (Navigation) offlineObject()  {}
func (NumberInput) offlineObject() {}
func (BitControl) offlineObject()  {}
func (Primitive) offlineObject()   {}
func (Bitmap) offlineObject()      {}

type OfflineScreen struct {
	ID      int
	Objects []Object
}

// BuildOffline compiles supplied user screens and retains the reference's
// other screens and resources.
func (pkg *Package) BuildOffline(screens []OfflineScreen) ([]byte, error) {
	declared := make(map[int]OfflineScreen, len(screens))
	screenCount := len(pkg.Screens)
	for _, screen := range screens {
		if screen.ID < 0 || (screen.ID > 0 && screen.ID < 10) || screen.ID > maxUserScreenID {
			return nil, fmt.Errorf("screen %d: expected a user screen ID (0 or 10 through %d)", screen.ID, maxUserScreenID)
		}
		if _, exists := declared[screen.ID]; exists {
			return nil, fmt.Errorf("screen %d: duplicate screen ID", screen.ID)
		}
		declared[screen.ID] = screen
		screenCount = max(screenCount, screen.ID+1)
	}
	if _, exists := declared[0]; !exists {
		return nil, fmt.Errorf("screen 0: missing start screen")
	}
	reference := pkg.data[pkg.hmi:pkg.hmiEnd]
	if screenCount > len(pkg.Screens) {
		return pkg.buildExpandedScreens(reference, declared, screenCount)
	}
	data := bytes.Clone(pkg.data)
	image := data[pkg.hmi:pkg.hmiEnd]
	pool := artworkPool{base: len(image), limit: maxPackageSize - len(data)}
	for _, screen := range screens {
		from, to := screenRange(reference, len(pkg.Screens), screen.ID)
		record, err := pkg.encodeScreen(reference, screen, declared, from, to-from, nil)
		if errors.Is(err, errScreenCapacity) {
			record, err = pkg.encodeScreen(reference, screen, declared, from, to-from, &pool)
		}
		if errors.Is(err, errScreenCapacity) {
			return pkg.buildExpandedScreens(reference, declared, screenCount)
		}
		if err != nil {
			return nil, fmt.Errorf("screen %d: %w", screen.ID, err)
		}
		clear(image[from:to])
		copy(image[from:to], record)
	}
	return pkg.finishBuild(data, pool.data, nil)
}

func (pkg *Package) finishBuild(data, extra []byte, directory *screenDirectory) ([]byte, error) {
	if len(extra) != 0 {
		var err error
		data, err = pkg.appendHMIData(data, extra)
		if err != nil {
			return nil, err
		}
	}
	image := data[pkg.hmi : pkg.hmiEnd+len(extra)]
	if directory != nil {
		if directory.count != len(pkg.Screens) {
			if err := updateSystemScreenCount(image, len(pkg.Screens), directory.count); err != nil {
				return nil, err
			}
		}
		little.PutUint32(image[screenTableField:], uint32(directory.table))
		little.PutUint32(image[screenEndField:], uint32(directory.end))
		little.PutUint32(image[screenMetadataField:], uint32(directory.metadata))
	}
	little.PutUint32(image[12:], uint32(len(image)))
	little.PutUint32(image[16:], hmiChecksum(image))
	writeHMIIntegrity(data, image, pkg.project)
	return data, nil
}

func (pkg *Package) encodeScreen(reference []byte, screen OfflineScreen, declared map[int]OfflineScreen, offset, capacity int, pool *artworkPool) ([]byte, error) {
	count := len(screen.Objects)
	if count == 0 || count > 65535 {
		return nil, fmt.Errorf("invalid component count")
	}
	headerSize := emptyScreenSize + count*componentSlotSize
	if headerSize > capacity {
		return nil, errScreenCapacity
	}
	screenRecord := make([]byte, headerSize)
	geometry := Screen{Width: panelWidth, Height: panelHeight}
	if screen.ID < len(pkg.Screens) {
		geometry = pkg.Screens[screen.ID]
		if geometry.Width != panelWidth || geometry.Height != panelHeight {
			return nil, fmt.Errorf("expected an 800×480 screen slot in the reference")
		}
		from, _ := screenRange(reference, len(pkg.Screens), screen.ID)
		copy(screenRecord[:screenHeaderSize], reference[from:from+screenHeaderSize])
	} else {
		putUserScreenHeader(screenRecord, screen.ID)
	}
	little.PutUint16(screenRecord[34:], uint16(count))
	for i, component := range screen.Objects {
		at := offset + len(screenRecord)
		kind, record, err := pkg.encodeObject(geometry, at, component, declared, reference, pool)
		if err != nil {
			return nil, fmt.Errorf("component %d: %w", i, err)
		}
		if len(record) > capacity-len(screenRecord) {
			return nil, fmt.Errorf("component %d: %w", i, errScreenCapacity)
		}
		slot := screenHeaderSize + i*componentSlotSize
		little.PutUint32(screenRecord[slot:], kind)
		little.PutUint32(screenRecord[slot+4:], uint32(at))
		screenRecord = append(screenRecord, record...)
	}
	return screenRecord, nil
}

func (pkg *Package) encodeObject(screen Screen, offset int, object Object, declared map[int]OfflineScreen, reference []byte, pool *artworkPool) (uint32, []byte, error) {
	switch obj := object.(type) {
	case Text:
		record, err := encodeText(screen, obj)
		return textComponent, record, err
	case Navigation:
		if _, exists := declared[obj.Target]; !exists {
			return 0, nil, fmt.Errorf("destination screen %d must be in the project", obj.Target)
		}
		record, err := encodeNavigation(pkg, reference, screen, offset, obj)
		return navigationComponent, record, err
	case NumberInput:
		record, err := encodeNumberInput(pkg, reference, screen, offset, obj)
		return numberInputComponent, record, err
	case BitControl:
		return encodeBitControl(pkg, reference, screen, offset, obj, pool)
	case Primitive:
		return encodePrimitive(screen, obj)
	case Bitmap:
		record, err := encodeBitmap(screen, offset, obj, pool)
		return bitmapComponent, record, err
	default:
		return 0, nil, fmt.Errorf("unsupported panel object")
	}
}

func encodeText(screen Screen, text Text) ([]byte, error) {
	glyphs, err := textGlyphs(screen.Width, screen.Height, text)
	if err != nil {
		return nil, err
	}
	record := make([]byte, (32+2*len(glyphs)+3)&^3)
	putBounds(record, text.X, text.Y, text.Width, text.Height)
	little.PutUint32(record[8:], 1)
	little.PutUint16(record[16:], uint16(text.Width-1))
	little.PutUint16(record[18:], uint16(text.Height-1))
	little.PutUint16(record[20:], rgb565(text.RGB))
	record[23] = 1 // Left-aligned plain vector font.
	little.PutUint16(record[24:], uint16(text.Size))
	little.PutUint32(record[28:], uint32(len(glyphs)))
	putGlyphs(record[32:], glyphs)
	return record, nil
}

func textGlyphs(width, height int, text Text) ([]uint16, error) {
	if !fits(width, height, text.X, text.Y, text.Width, text.Height) {
		return nil, fmt.Errorf("text bounds are outside their container")
	}
	if text.Size < 8 || text.Size > 64 || text.RGB > 0xffffff {
		return nil, fmt.Errorf("invalid text font size or color")
	}
	if text.Value == "" || !utf8.ValidString(text.Value) || len(text.Value) > 4096 ||
		strings.ContainsAny(text.Value, "\x00\r") {
		return nil, fmt.Errorf("invalid text content")
	}
	return utf16.Encode([]rune(strings.ReplaceAll(text.Value, "\n", "\r"))), nil
}

func fits(width, height, x, y, w, h int) bool {
	return x >= 0 && y >= 0 && w > 0 && h > 0 && x < width && y < height &&
		w <= width-x && h <= height-y
}

func putBounds(record []byte, x, y, width, height int) {
	little.PutUint16(record[0:], uint16(x))
	little.PutUint16(record[2:], uint16(y))
	little.PutUint16(record[4:], uint16(x+width-1))
	little.PutUint16(record[6:], uint16(y+height-1))
}

func putGlyphs(data []byte, glyphs []uint16) {
	for i, glyph := range glyphs {
		little.PutUint16(data[2*i:], glyph)
	}
}

func rgb565(rgb uint32) uint16 {
	return uint16((rgb>>8)&0xf800 | (rgb>>5)&0x07e0 | (rgb>>3)&0x001f)
}
