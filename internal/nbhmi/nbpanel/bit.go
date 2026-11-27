package nbpanel

import "fmt"

const (
	bitLampComponent = 0x201
	switchComponent  = 0x303
	localBitLimit    = 9000
	switchCaptionAt  = 172
	lampRecordSize   = 108
	vectorGraphicAt  = 28
	rasterGraphicAt  = 32
)

// BitControl is a lamp when Caption is nil and a toggle switch otherwise.
type BitControl struct {
	X, Y, Width, Height int
	Index               int
	CIO                 bool
	Caption             *Text
	ConfirmWaitSeconds  int
	Artwork             *BitArtwork
}

// BitArtwork holds the resources for states 0 and 1 of a bit.
type BitArtwork struct {
	states [2][]byte
	vector bool
}

func NewBitArtwork(off, on []byte) (*BitArtwork, error) {
	artwork := &BitArtwork{}
	for state, source := range [2][]byte{off, on} {
		resource, err := bitmapResource(source)
		if err != nil {
			return nil, fmt.Errorf("state %d: %w", state, err)
		}
		artwork.states[state] = resource
	}
	return artwork, nil
}

func encodeBitControl(pkg *Package, reference []byte, screen Screen, offset int, bit BitControl, pool *artworkPool) (uint32, []byte, error) {
	if !fits(screen.Width, screen.Height, bit.X, bit.Y, bit.Width, bit.Height) {
		return 0, nil, fmt.Errorf("bit control bounds are outside the screen")
	}
	address, err := encodeBitAddress(reference, bit)
	if err != nil {
		return 0, nil, err
	}
	if bit.Caption == nil {
		if bit.ConfirmWaitSeconds != 0 {
			return 0, nil, fmt.Errorf("a bit lamp cannot ask for confirmation")
		}
		record, err := encodeBitLamp(pkg, reference, offset, bit, address, pool)
		return bitLampComponent, record, err
	}
	record, err := encodeBitSwitch(pkg, reference, offset, bit, address, pool)
	return switchComponent, record, err
}

func encodeBitLamp(pkg *Package, reference []byte, offset int, bit BitControl, address [16]byte, pool *artworkPool) ([]byte, error) {
	const (
		label1 = 68
		label2 = 88
	)
	record := make([]byte, lampRecordSize)
	putBounds(record, bit.X, bit.Y, bit.Width, bit.Height)
	copy(record[8:], address[:])
	little.PutUint32(record[24:], 1)
	little.PutUint32(record[36:], uint32(offset+label1))
	little.PutUint32(record[40:], uint32(offset+label2))
	little.PutUint32(record[56:], uint32(offset+60))
	// Each caption is empty.
	for _, at := range []int{label1, label2} {
		putBounds(record[at:], bit.Width/2, bit.Height/2, 1, 1)
		little.PutUint16(record[at+12:], 16)
	}
	return attachBitGraphic(pkg, reference, record, offset, bitLampComponent, bit.Artwork, pool)
}

func encodeBitSwitch(pkg *Package, reference []byte, offset int, bit BitControl, address [16]byte, pool *artworkPool) ([]byte, error) {
	row, err := buttonCaption(bit.Width, bit.Height, *bit.Caption)
	if err != nil {
		return nil, fmt.Errorf("switch caption: %w", err)
	}
	control, timeout, err := pkg.confirmation(reference, bit.ConfirmWaitSeconds)
	if err != nil {
		return nil, err
	}
	record := make([]byte, switchCaptionAt+2*len(row))
	putBounds(record, bit.X, bit.Y, bit.Width, bit.Height)
	copy(record[8:], address[:])
	copy(record[44:], address[:])
	little.PutUint32(record[24:], 1)
	little.PutUint32(record[36:], uint32(offset+switchCaptionAt))
	little.PutUint32(record[40:], uint32(offset+switchCaptionAt+len(row)))
	little.PutUint16(record[60:], 2) // Toggle.
	little.PutUint32(record[62:], 0xffffffff)
	little.PutUint32(record[86:], uint32(offset+94))
	little.PutUint32(record[90:], uint32(offset+164))
	little.PutUint16(record[94:], control)
	little.PutUint16(record[102:], timeout)
	little.PutUint16(record[110:], 0xfe00)
	little.PutUint16(record[118:], 1)
	little.PutUint16(record[120:], 0x100)
	little.PutUint32(record[122:], 0xffffffff)
	little.PutUint32(record[130:], uint32(offset+138))
	little.PutUint32(record[150:], 1)
	little.PutUint32(record[154:], 3)
	putGlyphs(record[158:], []uint16{'L', 'B', 'T'})
	copy(record[switchCaptionAt:], row)
	copy(record[switchCaptionAt+len(row):], row)
	return attachBitGraphic(pkg, reference, record, offset, switchComponent, bit.Artwork, pool)
}

func attachBitGraphic(pkg *Package, reference, record []byte, offset int, kind uint32, artwork *BitArtwork, pool *artworkPool) ([]byte, error) {
	at := rasterGraphicAt
	if artwork == nil {
		graphic, err := bitGraphic(pkg, reference, kind)
		if err != nil {
			return nil, err
		}
		if kind == switchComponent {
			at = vectorGraphicAt
		}
		little.PutUint32(record[at:], graphic)
		return record, nil
	}

	const groupHeaderSize = 8
	const stateTableSize = 8
	if len(artwork.states[0]) == 0 || len(artwork.states[1]) == 0 {
		return nil, fmt.Errorf("missing bit state artwork")
	}
	if artwork.vector {
		at = vectorGraphicAt
	}
	// The group and its state table use offsets from the start of the HMI image.
	groupAt := pool.offset(offset + len(record))
	tableAt := groupAt + groupHeaderSize
	firstAt := tableAt + stateTableSize
	little.PutUint32(record[at:], uint32(groupAt))
	group := make([]byte, groupHeaderSize+stateTableSize)
	little.PutUint32(group, 2)
	little.PutUint32(group[4:], uint32(tableAt))
	little.PutUint32(group[8:], uint32(firstAt))
	little.PutUint32(group[12:], uint32(firstAt+len(artwork.states[0])))
	return pool.store(record, group, artwork.states[0], artwork.states[1])
}

func bitGraphic(pkg *Package, image []byte, kind uint32) (uint32, error) {
	table := u32(image, screenTableField)
	for _, screen := range pkg.Screens {
		for _, component := range screen.Components {
			if component.Type != kind {
				continue
			}
			at := component.Offset
			record := image[at : at+component.Size]
			var graphic int
			switch kind {
			case bitLampComponent:
				if len(record) < lampRecordSize || u32(record, 28) != 0 ||
					u32(record, 56) != at+60 || u32(record, 36) != at+68 {
					continue
				}
				graphic = u32(record, 32)
			case switchComponent:
				if len(record) < switchCaptionAt || little.Uint16(record[60:]) != 2 ||
					u32(record, 86) != at+94 || u32(record, 130) != at+138 {
					continue
				}
				graphic = u32(record, 28)
			}
			if graphic >= hmiHeaderSize && graphic < table-8 && u32(image, graphic) == 2 {
				return uint32(graphic), nil
			}
		}
	}
	return 0, fmt.Errorf("reference package needs a compatible two-state graphic for component %#x", kind)
}
