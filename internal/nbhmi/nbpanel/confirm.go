package nbpanel

import "fmt"

const (
	DefaultConfirmWaitSeconds = 30
	MaxConfirmWaitSeconds     = 0xffff
	confirmControlFlag        = 0x1000
	confirmScreenID           = 7
	confirmResponseWord       = 9370
)

// confirmation returns the control flags and timeout for a stock dialog.
func (pkg *Package) confirmation(image []byte, seconds int) (uint16, uint16, error) {
	const defaultFlags = 2
	if seconds == 0 {
		return defaultFlags, DefaultConfirmWaitSeconds, nil
	}
	if seconds < 1 || seconds > MaxConfirmWaitSeconds {
		return 0, 0, fmt.Errorf("confirmation timeout is outside the supported range")
	}
	if len(pkg.Screens) <= confirmScreenID {
		return 0, 0, fmt.Errorf("reference package is missing confirmation screen %d", confirmScreenID)
	}
	var yes, cancel bool
	for _, component := range pkg.Screens[confirmScreenID].Components {
		if component.Type != 0x302 || component.Size < 64 { // Word Set.
			continue
		}
		row := image[component.Offset : component.Offset+component.Size]
		if u32(row, 8) != 0x02000000 || u32(row, 12) != 0xff02 ||
			u32(row, 16) != confirmResponseWord || u32(row, 44) != 1 {
			continue
		}
		switch u32(row, 48) {
		case 1:
			yes = true
		case 2:
			cancel = true
		}
	}
	if !yes || !cancel {
		return 0, 0, fmt.Errorf("reference confirmation screen needs Yes and Cancel actions on LW%d", confirmResponseWord)
	}
	return defaultFlags | confirmControlFlag, uint16(seconds), nil
}
