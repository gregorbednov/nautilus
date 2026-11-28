package nbpanel

import "fmt"

// CIOBitLimit is the number of bit addresses in the supported CIO range.
const CIOBitLimit = 6144 * 16

// LocalBitLimit is the first local bit reserved for system use.
const LocalBitLimit = 9000

func encodeBitAddress(reference []byte, bit BitControl) ([16]byte, error) {
	var address [16]byte
	area := uint32(0xfe00) // Local LB.
	if bit.CIO {
		if bit.Index < 0 || bit.Index >= CIOBitLimit {
			return address, fmt.Errorf("expected a CIO bit from 0.00 through 6143.15")
		}
		device, err := finsBitDevice(reference)
		if err != nil {
			return address, err
		}
		little.PutUint32(address[:], device)
		area = 0x3200 // The Omron HMI's CIO bit selector.
	} else if bit.Index < 0 || bit.Index >= LocalBitLimit {
		return address, fmt.Errorf("expected a user LB register below LB%d", LocalBitLimit)
	}
	little.PutUint32(address[4:], area)
	little.PutUint32(address[8:], uint32(bit.Index))
	little.PutUint32(address[12:], 0x01000001)
	return address, nil
}

func finsBitDevice(image []byte) (uint32, error) {
	const (
		systemDirectoryField = 24
		deviceCount          = 4
		deviceRecordSize     = 32
		externalSlot         = 3
		finsUDPDriver        = 0x55
	)
	invalid := fmt.Errorf("CIO requires a reference package with one supported Omron FINS/UDP PLC")
	if len(image) < hmiHeaderSize {
		return 0, invalid
	}
	system := u32(image, systemDirectoryField)
	screenTable := u32(image, screenTableField)
	if screenTable > len(image) || system < hmiHeaderSize || system > screenTable-12 {
		return 0, invalid
	}
	from, to := u32(image, system+4), u32(image, system+8)
	const directorySize = 4 + 4*deviceCount
	if from < system+12 || to > screenTable || from > to-directorySize-deviceRecordSize*deviceCount ||
		u32(image, from) != deviceCount {
		return 0, invalid
	}
	first := from + directorySize
	for slot := range deviceCount {
		offset := first + deviceRecordSize*slot
		if u32(image, from+4+4*slot) != offset || image[offset] != byte(slot) {
			return 0, invalid
		}
	}
	// The first three slots are the NB10W's built-in channels.
	for slot, kind := range [externalSlot]uint16{0x0207, 0x0207, 0x0008} {
		offset := first + deviceRecordSize*slot
		if image[offset+1] != 0 || little.Uint16(image[offset+2:]) != kind {
			return 0, invalid
		}
	}
	at := first + externalSlot*deviceRecordSize
	node := little.Uint16(image[at+24:])
	if image[at+1] != finsUDPDriver || little.Uint16(image[at+2:]) == 0 || node == 0 || node > 254 {
		return 0, invalid
	}
	return uint32(node)<<16 | externalSlot<<8, nil
}
