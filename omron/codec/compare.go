package codec

import "strings"

// CMP reads both operands. In particular its second operand is not a MOV-style
// writable destination: constants and read-only Auxiliary words are allowed.
func encodeCMP(operands []string) ([]uint32, error) {
	if len(operands) != 2 {
		return nil, ErrUnqualifiedInstruction
	}
	words := []uint32{0xb2400100, 0, 0}
	for i, operand := range operands {
		position := uint32(i) << 26
		if strings.HasPrefix(operand, "#") || strings.HasPrefix(operand, "&") {
			literal, err := ParseLiteral16(operand)
			if err != nil {
				return nil, err
			}
			words[i+1] = 0x61400000 | position | uint32(literal.Value)
			if literal.Decimal {
				words[i+1] |= 0x40000
			}
		} else {
			address, err := ParseWordAddress(operand)
			if err != nil {
				return nil, err
			}
			field, err := address.field(false)
			if err != nil {
				return nil, err
			}
			words[i+1] = 0x60000000 | position | field
		}
	}
	return words, nil
}

func decodeCMP(words []uint32) (string, error) {
	if len(words) < 3 {
		return "", ErrUnqualifiedInstruction
	}
	operands := make([]string, 2)
	for i := range operands {
		position := uint32(i) << 26
		word := words[i+1]
		switch word & 0xffff0000 {
		case 0x61400000 | position, 0x61440000 | position:
			operands[i] = (Literal16{Value: uint16(word), Decimal: word&0x40000 != 0}).String()
		default:
			if word&0xfffe0000 != 0x60000000|position {
				return "", ErrWordOperand
			}
			address, err := wordAddressFromField(word&0x1ffff, false)
			if err != nil {
				return "", err
			}
			operands[i] = address.String()
		}
	}
	return "CMP(020) " + strings.Join(operands, " "), nil
}
