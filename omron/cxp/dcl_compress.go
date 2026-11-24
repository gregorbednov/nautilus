package cxp

import (
	"bytes"
	"fmt"
	"slices"
)

// The captured binary-mode DCL writer uses a 4 KiB dictionary and prefers the
// closest equal match. Short matches can yield to a better match one byte later.
// The public literal-only CXP Pack behavior remains independent of this writer.
func packFBSource(data []byte) ([]byte, error) {
	const dictionary = 4096
	const maxMatch = 516
	const maxPacked = 0x7fff
	positions := make(map[uint16][]int)
	pair := func(i int) uint16 { return uint16(data[i])<<8 | uint16(data[i+1]) }
	for offset := 0; offset+1 < len(data); offset++ {
		key := pair(offset)
		positions[key] = append(positions[key], offset)
	}
	match := func(offset int) (length, distance int) {
		available := min(maxMatch, len(data)-offset)
		if available < 2 {
			return 0, 0
		}
		candidates := positions[pair(offset)]
		start, _ := slices.BinarySearch(candidates, offset-dictionary+1)
		for _, previous := range candidates[start:] {
			if previous >= offset-1 {
				break
			}
			if length >= 2 && data[previous+length-1] != data[offset+length-1] {
				continue
			}
			size := 2
			for size < available && data[previous+size] == data[offset+size] {
				size++
			}
			if size >= length {
				length, distance = size, offset-previous
				if length == maxMatch {
					break
				}
			}
		}
		return length, distance
	}
	w := dclWriter{buffer: []byte{0, 6}, used: 16}
	lengthCodes := binaryDCLCodes(lengthTable)
	distanceCodes := binaryDCLCodes(distanceTable)
	for offset := 0; offset < len(data); {
		length, distance := match(offset)
		useMatch := length > 2 || length == 2 && distance <= 256
		if useMatch && length < 8 && offset+length < len(data) {
			next, _ := match(offset + 1)
			useMatch = next <= length || next == length+1 && distance <= 129
		}
		if !useMatch {
			w.put(uint32(data[offset])<<1, 9)
			offset++
		} else {
			w.length(length, lengthCodes)
			lowBits := 6
			if length == 2 {
				lowBits = 2
			}
			value := uint32(distance - 1)
			code := distanceCodes[value>>lowBits]
			w.put(code.value, code.bits)
			w.put(value, lowBits)
			offset += length
		}
		if len(w.buffer) > maxPacked {
			return nil, fmt.Errorf("FBL exceeds the captured file-length field")
		}
	}
	w.length(519, lengthCodes)
	if len(w.buffer) > maxPacked {
		return nil, fmt.Errorf("FBL exceeds the captured file-length field")
	}
	decoded, err := Explode(w.buffer, MaxSize)
	if err != nil || !bytes.Equal(data, decoded) {
		return nil, fmt.Errorf("FBL failed lossless compression verification")
	}
	return w.buffer, nil
}

type dclCode struct {
	value uint32
	bits  int
}

func binaryDCLCodes(table huffman) []dclCode {
	codes := make([]dclCode, len(table.symbols))
	first, offset := 0, 0
	for bits := 1; bits < len(table.count); bits++ {
		for i := 0; i < table.count[bits]; i++ {
			value := uint32(first + i)
			var reversed uint32
			for b := 0; b < bits; b++ {
				reversed = reversed<<1 | ((value>>b)&1 ^ 1)
			}
			codes[table.symbols[offset+i]] = dclCode{reversed, bits}
		}
		offset += table.count[bits]
		first = (first + table.count[bits]) << 1
	}
	return codes
}

type dclWriter struct {
	buffer []byte
	used   int
}

func (w *dclWriter) put(value uint32, bits int) {
	for bit := 0; bit < bits; bit++ {
		if w.used/8 == len(w.buffer) {
			w.buffer = append(w.buffer, 0)
		}
		w.buffer[w.used/8] |= byte((value>>bit)&1) << (w.used % 8)
		w.used++
	}
}

func (w *dclWriter) length(length int, codes []dclCode) {
	for symbol, base := range lengthBase {
		extra := lengthExtra[symbol]
		if length >= base && length < base+(1<<extra) {
			code := codes[symbol]
			w.put(1, 1)
			w.put(code.value, code.bits)
			w.put(uint32(length-base), extra)
			return
		}
	}
}
