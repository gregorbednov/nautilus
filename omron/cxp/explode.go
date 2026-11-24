// Package cxp reads CX-Programmer's compressed CXP and uncompressed CXT projects.
// It does not invoke CX-One or decode a CPU executable image.
package cxp

import (
	"errors"
	"fmt"
	"io"
)

// MaxSize bounds both the input and expanded project before parsing.
const MaxSize = 32 << 20

type bitReader struct {
	data []byte
	pos  int
}

func (b *bitReader) bits(n int) (int, error) {
	if b.pos+n > len(b.data)*8 {
		return 0, io.ErrUnexpectedEOF
	}
	v := 0
	for i := 0; i < n; i++ {
		v |= int(b.data[b.pos/8]>>(b.pos%8)&1) << i
		b.pos++
	}
	return v, nil
}

type huffman struct {
	count   [14]int
	symbols []int
}

func table(rep []byte) huffman {
	var lengths []int
	for _, r := range rep {
		for i := 0; i < int(r>>4)+1; i++ {
			lengths = append(lengths, int(r&15))
		}
	}
	var h huffman
	for _, n := range lengths {
		h.count[n]++
	}
	for n := 1; n < 14; n++ {
		for s, l := range lengths {
			if l == n {
				h.symbols = append(h.symbols, s)
			}
		}
	}
	return h
}
func (b *bitReader) symbol(h huffman) (int, error) {
	code, first, index := 0, 0, 0
	for n := 1; n < 14; n++ {
		bit, err := b.bits(1)
		if err != nil {
			return 0, err
		}
		code |= bit ^ 1
		count := h.count[n]
		if code >= first && code < first+count {
			return h.symbols[index+code-first], nil
		}
		index += count
		first = (first + count) << 1
		code <<= 1
	}
	return 0, errors.New("invalid PKWARE Huffman code")
}

// Static code lengths are properties of PKWARE DCL, as documented by
// Mark Adler's zlib/contrib/blast (https://github.com/madler/zlib).
var literalTable = table([]byte{11, 124, 8, 7, 28, 7, 188, 13, 76, 4, 10, 8, 12, 10, 12, 10, 8, 23, 8, 9, 7, 6, 7, 8, 7, 6, 55, 8, 23, 24, 12, 11, 7, 9, 11, 12, 6, 7, 22, 5, 7, 24, 6, 11, 9, 6, 7, 22, 7, 11, 38, 7, 9, 8, 25, 11, 8, 11, 9, 12, 8, 12, 5, 38, 5, 38, 5, 11, 7, 5, 6, 21, 6, 10, 53, 8, 7, 24, 10, 27, 44, 253, 253, 253, 252, 252, 252, 13, 12, 45, 12, 45, 12, 61, 12, 45, 44, 173})
var lengthTable = table([]byte{2, 35, 36, 53, 38, 23})
var distanceTable = table([]byte{2, 20, 53, 230, 247, 151, 248})
var lengthBase = [16]int{3, 2, 4, 5, 6, 7, 8, 9, 10, 12, 16, 24, 40, 72, 136, 264}
var lengthExtra = [16]int{0, 0, 0, 0, 0, 0, 0, 0, 1, 2, 3, 4, 5, 6, 7, 8}

// Explode decodes one complete PKWARE DCL stream, including its end marker.
// Truncation, trailing bytes, invalid back references and expansion beyond
// limit are errors. No partial project is returned after an error.
func Explode(data []byte, limit int) ([]byte, error) {
	if len(data) > MaxSize || limit <= 0 || limit > MaxSize {
		return nil, errors.New("CXP size limit")
	}
	b := bitReader{data: data}
	lit, err := b.bits(8)
	if err != nil {
		return nil, err
	}
	dict, err := b.bits(8)
	if err != nil {
		return nil, err
	}
	if lit > 1 || dict < 4 || dict > 6 {
		return nil, errors.New("invalid PKWARE DCL header")
	}
	out := make([]byte, 0, min(limit, 4096))
	for {
		match, err := b.bits(1)
		if err != nil {
			return nil, err
		}
		if match == 0 {
			var c int
			if lit == 0 {
				c, err = b.bits(8)
			} else {
				c, err = b.symbol(literalTable)
			}
			if err != nil {
				return nil, err
			}
			if len(out) == limit {
				return nil, errors.New("CXP expansion limit")
			}
			out = append(out, byte(c))
			continue
		}
		sym, err := b.symbol(lengthTable)
		if err != nil {
			return nil, err
		}
		extra, err := b.bits(lengthExtra[sym])
		if err != nil {
			return nil, err
		}
		size := lengthBase[sym] + extra
		if size == 519 {
			if (b.pos+7)/8 != len(data) {
				return nil, errors.New("trailing data after CXP stream")
			}
			return out, nil
		}
		n := dict
		if size == 2 {
			n = 2
		}
		high, err := b.symbol(distanceTable)
		if err != nil {
			return nil, err
		}
		low, err := b.bits(n)
		if err != nil {
			return nil, err
		}
		distance := (high << n) + low + 1
		if distance > len(out) {
			return nil, fmt.Errorf("CXP back reference %d before output", distance)
		}
		if size > limit-len(out) {
			return nil, errors.New("CXP expansion limit")
		}
		for i := 0; i < size; i++ {
			out = append(out, out[len(out)-distance])
		}
	}
}
