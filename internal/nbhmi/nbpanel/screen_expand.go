package nbpanel

import (
	"bytes"
	"errors"
	"fmt"
)

type screenDirectory struct {
	table, end, metadata, count int
}

const emptyScreenSize = screenHeaderSize + 4

func (pkg *Package) buildExpandedScreens(reference []byte, declared map[int]OfflineScreen, count int) ([]byte, error) {
	metadata, err := referenceScreenMetadata(reference)
	if err != nil {
		return nil, err
	}
	base := len(reference)
	tableSize := 4 * count
	limit := maxPackageSize - len(pkg.data) - len(metadata)
	if tableSize > limit {
		return nil, fmt.Errorf("expanded HMI exceeds the panel package size limit")
	}
	area := make([]byte, tableSize)
	for id := range count {
		at := base + len(area)
		little.PutUint32(area[id*4:], uint32(at))
		capacity := limit - len(area)
		var record []byte
		if screen, exists := declared[id]; exists {
			record, err = pkg.encodeScreen(reference, screen, declared, at, capacity, nil)
			if errors.Is(err, errScreenCapacity) {
				return nil, fmt.Errorf("expanded screen %d exceeds the panel package size limit", id)
			}
			if err != nil {
				return nil, fmt.Errorf("expanded screen %d: %w", id, err)
			}
		} else if id < len(pkg.Screens) {
			from, to := screenRange(reference, len(pkg.Screens), id)
			if to-from > capacity {
				return nil, fmt.Errorf("expanded HMI exceeds the panel package size limit")
			}
			record = bytes.Clone(reference[from:to])
			// Rebase the component directory. Opaque records retain valid links
			// into the original HMI data, which the package still contains.
			for i, component := range pkg.Screens[id].Components {
				slot := screenHeaderSize + i*componentSlotSize
				little.PutUint32(record[slot+4:], uint32(at+component.Offset-from))
			}
		} else {
			if capacity < emptyScreenSize {
				return nil, fmt.Errorf("expanded HMI exceeds the panel package size limit")
			}
			// The native screen directory is indexed by ID, including gaps.
			record = make([]byte, emptyScreenSize)
			putUserScreenHeader(record, id)
		}
		area = append(area, record...)
	}
	end := base + len(area)
	directory := screenDirectory{table: base, end: end, metadata: end + 8, count: count}
	metadataAt := len(area)
	area = append(area, metadata...)
	little.PutUint32(area[metadataAt:], uint32(end+4))
	return pkg.finishBuild(pkg.data, area, &directory)
}

func putUserScreenHeader(record []byte, id int) {
	putBounds(record, 0, 0, panelWidth, panelHeight)
	// The base window uses the same black fill and frame defaults as Frame0.
	little.PutUint16(record[8:], 0xffff)
	little.PutUint16(record[16:], 0xffff)
	little.PutUint16(record[20:], opaqueBlack565)
	little.PutUint16(record[24:], 0x0500)
	little.PutUint16(record[26:], uint16(id))
	// No attached sheet windows.
	for at := 28; at < 34; at += 2 {
		little.PutUint16(record[at:], 0xffff)
	}
}

// The terminal settings store the highest screen ID and the number of
// screens separately from the screen directory.
func updateSystemScreenCount(image []byte, previousCount, count int) error {
	const (
		systemTableField    = 24
		screenSettingsEntry = 16
		screenSettingsSize  = 16
	)
	invalid := fmt.Errorf("reference package has unsupported terminal screen settings")
	table := u32(image, screenTableField)
	system := u32(image, systemTableField)
	if system < hmiHeaderSize || system > table-screenSettingsEntry-4 {
		return invalid
	}
	settings := u32(image, system+screenSettingsEntry)
	if settings < system+screenSettingsEntry+4 || settings > table-screenSettingsSize ||
		u32(image, settings) != screenSettingsSize || u32(image, settings+4) != previousCount-1 ||
		u32(image, settings+8) != previousCount {
		return invalid
	}
	little.PutUint32(image[settings+4:], uint32(count-1))
	little.PutUint32(image[settings+8:], uint32(count))
	return nil
}

// The eight bytes after the screens precede six links to other HMI sections.
// Copy the directory and retain the locations of the referenced sections.
func referenceScreenMetadata(image []byte) ([]byte, error) {
	const size = 8 + 6*4
	end := u32(image, screenEndField)
	metadata := u32(image, screenMetadataField)
	invalid := fmt.Errorf("reference package has an unsupported screen metadata directory")
	if end > len(image)-size || metadata != end+8 ||
		u32(image, end) != end+4 {
		return nil, invalid
	}
	for at := metadata; at < end+size; at += 4 {
		target := u32(image, at)
		if target < end+size || target >= len(image) {
			// A rebuilt directory may still point to the original sections.
			if target < hmiHeaderSize || target >= u32(image, screenTableField) {
				return nil, invalid
			}
		}
	}
	return image[end : end+size], nil
}
