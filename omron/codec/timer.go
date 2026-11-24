package codec

import (
	"fmt"
	"strings"
)

type timerInstruction struct {
	bcd, binary string
	opcode      uint32
	area        Area
}

var timerInstructions = []timerInstruction{
	{"TIM", "TIMX", 0xbb4000c0, Timer},
	{"TIMH", "TIMHX", 0xbb4000c1, Timer},
	{"CNT", "CNTX", 0xbb4000d0, Counter},
}

func timerName(profile Profile, definition timerInstruction) (string, error) {
	switch profile.TimerCounterMode {
	case TimerModeBCD:
		return definition.bcd, nil
	case TimerModeBinary:
		return definition.binary, nil
	default:
		return "", fmt.Errorf("%w: timer/counter mode required", ErrUnqualifiedProfile)
	}
}

func isBCD(value uint16) bool {
	for shift := uint(0); shift < 16; shift += 4 {
		if (value>>shift)&15 > 9 {
			return false
		}
	}
	return true
}

func encodeTimer(profile Profile, definition timerInstruction, mnemonic string, operands []string) ([]uint32, error) {
	name, err := timerName(profile, definition)
	if err != nil {
		return nil, err
	}
	if name != mnemonic || len(operands) != 2 {
		return nil, ErrUnqualifiedInstruction
	}
	// Numeric indices and an explicit matching T/C prefix designate the same slot.
	indexText := operands[0]
	prefix := "T"
	if definition.area == Counter {
		prefix = "C"
	}
	if strings.HasPrefix(strings.ToUpper(indexText), prefix) {
		indexText = indexText[1:]
	}
	index, err := decimal(indexText, 12)
	if err != nil {
		return nil, ErrWordOperand
	}
	number := uint32(0x5000e000) + uint32(index)
	if definition.area == Counter {
		number += 0x1000
	}
	var preset uint32
	if strings.HasPrefix(operands[1], "#") || strings.HasPrefix(operands[1], "&") {
		literal, err := ParseLiteral16(operands[1])
		if err != nil {
			return nil, err
		}
		if profile.TimerCounterMode == TimerModeBCD && (literal.Decimal || !isBCD(literal.Value)) {
			return nil, ErrWordOperand
		}
		preset = 0x65400000 | uint32(literal.Value)
		if literal.Decimal {
			preset |= 0x40000
		}
	} else {
		address, err := ParseWordAddress(operands[1])
		if err != nil {
			return nil, err
		}
		field, err := address.field(false)
		if err != nil {
			return nil, err
		}
		preset = 0x64000000 | field
	}
	return []uint32{definition.opcode, number, preset}, nil
}

func decodeTimer(profile Profile, definition timerInstruction, words []uint32) (string, error) {
	name, err := timerName(profile, definition)
	if err != nil {
		return "", err
	}
	if len(words) < 3 {
		return "", ErrUnqualifiedInstruction
	}
	numberBase := uint32(0x5000e000)
	if definition.area == Counter {
		numberBase += 0x1000
	}
	if words[1]&0xfffff000 != numberBase {
		return "", ErrWordOperand
	}
	var preset string
	switch words[2] & 0xffff0000 {
	case 0x65400000, 0x65440000:
		literal := Literal16{uint16(words[2]), words[2]&0x40000 != 0}
		if profile.TimerCounterMode == TimerModeBCD && (literal.Decimal || !isBCD(literal.Value)) {
			return "", ErrWordOperand
		}
		preset = literal.String()
	default:
		if words[2]&0xfffe0000 != 0x64000000 {
			return "", ErrWordOperand
		}
		address, err := wordAddressFromField(words[2]&0x1ffff, false)
		if err != nil {
			return "", err
		}
		preset = address.String()
	}
	return fmt.Sprintf("%s %d %s", name, words[1]&0xfff, preset), nil
}
