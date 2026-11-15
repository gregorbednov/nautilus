package fbcode

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/joyautomation/nautilus/omron/cxp"
)

var ErrInstance = errors.New("unqualified scalar FB instance frame")

// InstanceBaseMove emits the measured CII-enabled generated base setup. Private
// H addresses are restricted here; ordinary public memory bounds are unchanged.
func InstanceBaseMove(bank int, address string) ([]uint32, error) {
	var value uint32
	switch bank {
	case 0, 1, 3, 4:
		prefix, min, max, origin := "T", 3072, 4095, 0xe000
		if bank == 1 {
			prefix, min, max, origin = "C", 3072, 4095, 0xf000
		}
		if bank == 3 {
			prefix, min, max, origin = "H", 512, 1407, 0xd800
		}
		if bank == 4 {
			prefix, min, max, origin = "H", 1408, 1535, 0xd800
		}
		if !strings.HasPrefix(address, prefix) {
			return nil, ErrInstance
		}
		n, err := strconv.Atoi(address[1:])
		if err != nil || n < min || n > max || strconv.Itoa(n) != address[1:] {
			return nil, ErrInstance
		}
		value = uint32(origin + n)
		return []uint32{0xbb7e0144, 0x50000000 | value, 0x5d0c0000 | uint32(bank)<<12}, nil
	case 2:
		if address != "#00009FFF" && address != "#9FFF" {
			return nil, ErrInstance
		}
		return []uint32{0xbb7e0141, 0x69409fff, 0x61400000, 0x5d0c2000}, nil
	default:
		return nil, ErrInstance
	}
}

func InstanceEnvelope(number int) (entry, exit []uint32, err error) {
	if number < 0 || number > 2047 {
		return nil, nil, ErrInstance
	}
	return []uint32{0xb040080a, 0x65440000 | uint32(number)}, []uint32{0xbb800803, 0x80000000}, nil
}

// InstanceControl is qualified only for the generated LD FI / RSET FI / #SET
// ENO sequence. Its final control bits must not be applied to general logic.
func InstanceControl(number int) ([][]uint32, error) {
	if number < 0 || number > 2047 {
		return nil, ErrInstance
	}
	return [][]uint32{{0xe203c000 | uint32(number)}, {0xf863c000 | uint32(number)}, {0xfa5f3001}}, nil
}

func InstanceTransfer(bit, push bool, offset int) ([]uint32, error) {
	if offset < 0 || offset > 2047 {
		return nil, ErrInstance
	}
	n := uint32(offset)
	if bit {
		if push {
			return []uint32{0xbb7f0147, 0x633c3000 | n, 0x5717e009}, nil
		}
		return []uint32{0xbb7f0147, 0x6317e001, 0x573c3000 | n}, nil
	}
	if push {
		return nil, ErrInstance
	} // Word-output frames have not been qualified.
	return []uint32{0xbb7f0140, 0x6117f001, 0x553c3000 | n}, nil
}

func InstanceBodyCall(number int) ([]uint32, error) {
	if number < 0 || number > 2047 {
		return nil, ErrInstance
	}
	return []uint32{0xbb7e0804, 0x65440000 | uint32(number), 0x80000000}, nil
}

// ValidateInstanceBases checks one saved placement against its logical frame;
// it neither performs global placement nor audits other instances for overlap.
func ValidateInstanceBases(bases [5]cxp.FBInstanceBase, words [5]int) error {
	for bank, base := range bases {
		if base.Size != words[bank] {
			return fmt.Errorf("%w: BR%d size mismatch", ErrInstance, bank)
		}
		if _, err := InstanceBaseMove(bank, base.Address); err != nil {
			return err
		}
	}
	if words[1] != 0 || words[2] != 0 || words[4] != 0 || bases[1].Address != "C3072" || bases[4].Address != "H1408" {
		return ErrInstance
	}
	t, _ := strconv.Atoi(bases[0].Address[1:])
	h, _ := strconv.Atoi(bases[3].Address[1:])
	if t+words[0] > 4096 || h+words[3] > 1408 {
		return fmt.Errorf("%w: saved placement exceeds private region", ErrInstance)
	}
	return nil
}
