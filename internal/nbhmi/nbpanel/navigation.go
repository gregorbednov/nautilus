package nbpanel

import (
	"fmt"
	"strings"
)

const (
	keyShapeAt   = 84
	keyShapeEnd  = 144
	keyDetailAt  = 154
	keyCaptionAt = 162
)

func encodeNavigation(pkg *Package, reference []byte, screen Screen, offset int, key Navigation) ([]byte, error) {
	if !fits(screen.Width, screen.Height, key.X, key.Y, key.Width, key.Height) {
		return nil, fmt.Errorf("navigation bounds are outside the screen")
	}
	row, err := buttonCaption(key.Width, key.Height, key.Label)
	if err != nil {
		return nil, fmt.Errorf("navigation caption: %w", err)
	}
	shape, err := navigationShape(pkg, reference)
	if err != nil {
		return nil, err
	}
	record := make([]byte, keyCaptionAt+2*len(row))
	putBounds(record, key.X, key.Y, key.Width, key.Height)
	little.PutUint16(record[24:], 1)
	little.PutUint16(record[26:], 0xffff)
	copy(record[28:32], shape[28:32]) // Reference to the stock button graphic.
	little.PutUint32(record[36:], uint32(offset+keyCaptionAt))
	little.PutUint32(record[40:], uint32(offset+keyCaptionAt+len(row)))
	little.PutUint32(record[44:], changeFrame)
	little.PutUint32(record[48:], uint32(key.Target))
	little.PutUint32(record[52:], 0xffffffff)
	little.PutUint32(record[76:], uint32(offset+keyShapeAt))
	little.PutUint32(record[80:], uint32(offset+keyDetailAt))
	copy(record[keyShapeAt:keyShapeEnd], shape[keyShapeAt:keyShapeEnd])
	little.PutUint32(record[120:], uint32(offset+128))
	little.PutUint32(record[144:], 3)
	putGlyphs(record[148:], []uint16{'N', 'A', 'V'})
	copy(record[keyCaptionAt:], row)
	copy(record[keyCaptionAt+len(row):], row)
	return record, nil
}

func buttonCaption(width, height int, caption Text) ([]byte, error) {
	if strings.ContainsRune(caption.Value, '\n') {
		return nil, fmt.Errorf("caption must be a single line")
	}
	glyphs, err := textGlyphs(width, height, caption)
	if err != nil {
		return nil, err
	}
	row := make([]byte, 20+len(glyphs)*2)
	putBounds(row, caption.X, caption.Y, caption.Width, caption.Height)
	little.PutUint16(row[8:], rgb565(caption.RGB))
	row[11] = 1 // Left-aligned plain vector font.
	little.PutUint16(row[12:], uint16(caption.Size))
	little.PutUint32(row[16:], uint32(len(glyphs)))
	putGlyphs(row[20:], glyphs)
	return row, nil
}

func navigationShape(pkg *Package, image []byte) ([]byte, error) {
	screenTable := u32(image, screenTableField)
	for _, screen := range pkg.Screens {
		for _, component := range screen.Components {
			if component.Type != navigationComponent || component.Size < keyCaptionAt {
				continue
			}
			at := component.Offset
			key := image[at : at+component.Size]
			graphic := u32(key, 28)
			if u32(key, 44) == changeFrame && u32(key, 76) == at+keyShapeAt &&
				u32(key, 80) == at+keyDetailAt && u32(key, 120) == at+128 &&
				u32(key, 84) == 2 && u32(key, 92) == 30 && u32(key, 100) == 0xfe00 &&
				graphic >= hmiHeaderSize && graphic < screenTable-8 && u32(image, graphic) == 2 {
				return key, nil
			}
		}
	}
	return nil, fmt.Errorf("reference package needs a supported stock Change Frame button")
}
