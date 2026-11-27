// Package nbpanel handles the compiled project formats used by NB10W-TW01B.
package nbpanel

import (
	"encoding/binary"
	"fmt"
	"math"
)

const (
	hmiMagic            = "eViewHMI"
	hmiHeaderSize       = 48
	screenHeaderSize    = 36
	componentSlotSize   = 8
	screenTableField    = 32
	screenEndField      = 36
	screenMetadataField = 40
)

var little = binary.LittleEndian

type Screen struct {
	ID         uint16      `json:"id"`
	Width      int         `json:"width"`
	Height     int         `json:"height"`
	Components []Component `json:"components"`
}

type Component struct {
	Type   uint32 `json:"type"`
	Offset int    `json:"offset"`
	Size   int    `json:"size"`
}

func screenRange(image []byte, count, id int) (int, int) {
	table := u32(image, screenTableField)
	from := u32(image, table+id*4)
	to := u32(image, screenEndField)
	if id+1 < count {
		to = u32(image, table+(id+1)*4)
	}
	return from, to
}

// ParseHMI checks the file checksum and all screen and component ranges.
// The returned offsets refer to the supplied image, not its package.
func ParseHMI(image []byte) ([]Screen, error) {
	if len(image) < hmiHeaderSize || string(image[:8]) != hmiMagic ||
		u32(image, 8) != 28 || u32(image, 20) != 24 {
		return nil, fmt.Errorf("unsupported eView HMI header")
	}
	if u32(image, 12) != len(image) || little.Uint32(image[16:]) != hmiChecksum(image) {
		return nil, fmt.Errorf("invalid eView HMI length or checksum")
	}
	table, end := u32(image, screenTableField), u32(image, screenEndField)
	if table < hmiHeaderSize || end > len(image) || end < 4 || table > end-4 {
		return nil, fmt.Errorf("invalid HMI screen table range")
	}
	start := u32(image, table)
	if start < table+4 || start > end-screenHeaderSize || (start-table)%4 != 0 {
		return nil, fmt.Errorf("invalid first HMI screen offset")
	}
	count := (start - table) / 4
	if count > 1<<16 {
		return nil, fmt.Errorf("too many HMI screen slots")
	}
	screens := make([]Screen, count)
	for index := range screens {
		slot := table + index*4
		from := u32(image, slot)
		to := end
		if slot+4 < start {
			to = u32(image, slot+4)
		}
		if from < start || to > end || from > to-screenHeaderSize-4 {
			return nil, fmt.Errorf("screen %d: invalid record range", index)
		}
		screen, err := parseScreen(image, from, to)
		if err != nil {
			return nil, fmt.Errorf("screen %d: %w", index, err)
		}
		if int(screen.ID) != index {
			return nil, fmt.Errorf("screen slot %d: record has ID %d", index, screen.ID)
		}
		screens[index] = screen
	}
	return screens, nil
}

func parseScreen(image []byte, from, to int) (Screen, error) {
	record := image[from:to]
	word := func(at int) int { return int(int16(little.Uint16(record[at:]))) }
	screen := Screen{
		ID:     little.Uint16(record[26:]),
		Width:  word(4) - word(0) + 1,
		Height: word(6) - word(2) + 1,
	}
	if screen.Width <= 0 || screen.Height <= 0 {
		return screen, fmt.Errorf("invalid geometry")
	}
	count := int(little.Uint16(record[34:]))
	data := from + screenHeaderSize + count*componentSlotSize + 4
	if data > to {
		return screen, fmt.Errorf("component table extends past screen")
	}
	screen.Components = make([]Component, count)
	for i := range screen.Components {
		at := from + screenHeaderSize + i*componentSlotSize
		offset := u32(image, at+4)
		next := to
		if i+1 < count {
			next = u32(image, at+componentSlotSize+4)
		}
		if offset < data || next <= offset || next > to {
			return screen, fmt.Errorf("component %d: invalid record range", i)
		}
		screen.Components[i] = Component{
			Type:   little.Uint32(image[at:]),
			Offset: offset,
			Size:   next - offset,
		}
	}
	return screen, nil
}

func hmiChecksum(image []byte) uint32 {
	var sum uint32
	for _, b := range image[hmiHeaderSize:] {
		sum += uint32(b)
	}
	return sum
}

func u32(data []byte, at int) int {
	return int(min(uint64(little.Uint32(data[at:])), uint64(math.MaxInt)))
}
