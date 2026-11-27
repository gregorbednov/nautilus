package nbpanel

import "fmt"

const (
	numberInputComponent = 0x306
	numberRecordSize     = 252
	numberShapeAt        = 180
	numberShapeEnd       = 244
	localWordLimit       = 9000 // The system registers start here.
	numberFormat         = 1
	numberDigits         = 4
)

type NumberInput struct {
	X, Y, Width, Height int
	Word                int
	Min, Max, Decimals  int
	ConfirmWaitSeconds  int
}

func encodeNumberInput(pkg *Package, reference []byte, screen Screen, offset int, input NumberInput) ([]byte, error) {
	if !fits(screen.Width, screen.Height, input.X, input.Y, input.Width, input.Height) ||
		input.Width < 120 || input.Height < 50 {
		return nil, fmt.Errorf("number input requires at least 120×50 pixels inside the screen")
	}
	if input.Word < 0 || input.Word >= localWordLimit-1 {
		return nil, fmt.Errorf("number input needs two consecutive user LW registers below LW%d", localWordLimit)
	}
	if input.Min < 0 || input.Min > input.Max || input.Max > 9999 || input.Decimals < 0 || input.Decimals >= numberDigits {
		return nil, fmt.Errorf("expected literal limits from 0 to 9999 and zero to three decimal places")
	}
	if len(pkg.Screens) <= 3 || pkg.Screens[3].Width != 235 || pkg.Screens[3].Height != 295 ||
		len(pkg.Screens[3].Components) == 0 {
		return nil, fmt.Errorf("reference package needs the stock numeric keyboard in screen 3")
	}
	shape, err := numberInputShape(pkg, reference)
	if err != nil {
		return nil, err
	}
	control, timeout, err := pkg.confirmation(reference, input.ConfirmWaitSeconds)
	if err != nil {
		return nil, err
	}

	record := make([]byte, numberRecordSize)
	putBounds(record, input.X, input.Y, input.Width, input.Height)
	// Address descriptors at 8 and 160 select the same local two-word value.
	putLocalWord(record[8:], input.Word)
	putLocalWord(record[160:], input.Word)
	copy(record[28:32], shape[28:32])
	// Stock font, keyboard placement, limit colors and disabled indirect limit.
	for _, field := range []struct {
		at    int
		value uint32
	}{
		{48, 0xfe00}, {52, 9000}, {56, 0x01000001},
		{60, 0x00040014},
		{88, 0x11}, {92, 0x01000020}, {96, 0x100},
		{116, 0x1f}, {120, 0x1f}, {124, 0xffffffff},
		{176, 1},
	} {
		little.PutUint32(record[field.at:], field.value)
	}
	little.PutUint16(record[64:], uint16(input.Width-11))
	little.PutUint16(record[66:], uint16(input.Height-16))
	record[68], record[70], record[71] = numberFormat, numberDigits, byte(input.Decimals)
	little.PutUint32(record[72:], uint32(input.Max))
	little.PutUint32(record[76:], uint32(input.Min))
	little.PutUint32(record[80:], 9999)
	putLocalWord(record[100:], 0) // Placeholder for the disabled indirect limit.
	little.PutUint32(record[148:], uint32(offset+numberShapeAt))
	little.PutUint32(record[152:], uint32(offset+numberShapeEnd))
	copy(record[numberShapeAt:numberShapeEnd], shape[numberShapeAt:numberShapeEnd])
	little.PutUint16(record[numberShapeAt:], control)
	little.PutUint16(record[numberShapeAt+8:], timeout)
	little.PutUint32(record[216:], uint32(offset+224))
	return record, nil
}

func putLocalWord(record []byte, word int) {
	little.PutUint32(record, 0x02000000)
	little.PutUint32(record[4:], 0xff02)
	little.PutUint32(record[8:], uint32(word))
	little.PutUint32(record[12:], 0x01000002)
}

func numberInputShape(pkg *Package, image []byte) ([]byte, error) {
	screenTable := u32(image, screenTableField)
	for _, screen := range pkg.Screens {
		for _, component := range screen.Components {
			if component.Type != numberInputComponent || component.Size < numberRecordSize {
				continue
			}
			at := component.Offset
			record := image[at : at+component.Size]
			graphic := u32(record, 28)
			if u32(record, 144) == 0 && u32(record, 148) == at+numberShapeAt &&
				u32(record, 152) == at+numberShapeEnd && u32(record, 216) == at+224 &&
				u32(record, 180)&^confirmControlFlag == 2 && u32(record, 196) == 0xfe00 &&
				graphic >= hmiHeaderSize && graphic < screenTable-8 && u32(image, graphic) == 1 {
				return record, nil
			}
		}
	}
	return nil, fmt.Errorf("reference package needs a supported stock Number Input component")
}
