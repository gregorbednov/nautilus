package um

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

var ErrN1FunctionBlocks = errors.New("unsupported or malformed N1 function-block sections")

// N1FunctionBlocks is a read-only inventory of the observed N1 prefix layout.
// Code retains numeric containers, including opaque relative operands. Nothing
// here allocates variables, rewrites FB code or qualifies a deployment image.
type N1FunctionBlocks struct {
	Definitions           []N1FBDefinition `json:"definitions"`
	Instances             []N1FBInstance   `json:"instances"`
	InstanceSectionHeader []uint32         `json:"instanceSectionHeader"`
	HardwareQualified     bool             `json:"hardwareQualified"`
}

type N1FBDefinition struct {
	Number         uint16   `json:"number"`
	Flags          uint32   `json:"flags"`
	BodyNumber     uint16   `json:"bodyNumber"`
	NameASCII      string   `json:"nameASCII,omitempty"`
	NameHex        string   `json:"nameHex"`
	OffsetWords    uint32   `json:"offsetWords"`
	LengthWords    uint32   `json:"lengthWords"`
	PortCountsWord uint32   `json:"portCountsWord"`
	Header         []uint32 `json:"header"`
	Code           []uint32 `json:"code"`
}

type N1FBInstance struct {
	Number      uint16   `json:"number"`
	OffsetWords uint32   `json:"offsetWords"`
	LengthWords uint32   `json:"lengthWords"`
	Counts      []uint32 `json:"counts"`
	Header      []uint32 `json:"header"`
	Code        []uint32 `json:"code"`
}

// FunctionBlocks decodes the explicitly selected observed prefix variant.
// Unknown variants fail atomically; Original still retains their complete data.
// Use the immutable raw header rather than caller-editable inspection fields.
func (image *N1Image) FunctionBlocks() (*N1FunctionBlocks, error) {
	fail := func(reason string) (*N1FunctionBlocks, error) {
		return nil, fmt.Errorf("%w: %s", ErrN1FunctionBlocks, reason)
	}
	if image == nil || len(image.raw) < n1HeaderWords*4 {
		return fail("no parsed N1 image")
	}
	data := image.raw
	word := func(i uint32) uint32 { return binary.BigEndian.Uint32(data[int(i)*4:]) }
	copyWords := func(start, length uint32) []uint32 {
		result := make([]uint32, int(length))
		for i := range result {
			result[i] = word(start + uint32(i))
		}
		return result
	}
	directory, defs, instances := word(0x22), word(0x23), word(0x24)
	if uint64(directory)*4 > uint64(len(data)) || directory < n1HeaderWords {
		return fail("invalid prefix boundary")
	}
	result := &N1FunctionBlocks{Definitions: []N1FBDefinition{}, Instances: []N1FBInstance{}, InstanceSectionHeader: []uint32{}}
	next := uint32(n1HeaderWords)
	if defs != 0 {
		end := directory
		if instances != 0 {
			end = instances
		}
		if defs != next || end > directory || uint64(defs)+4 > uint64(end) {
			return fail("definition section boundaries")
		}
		count := word(defs)
		if count == 0 || uint64(count)*27 > uint64(end-defs-4) {
			return fail("definition count outside section")
		}
		if word(defs+1) != 0 || word(defs+2) != 0 || word(defs+3) != 0 {
			return fail("unqualified definition section header")
		}
		next = defs + 4
		seen := map[uint16]bool{}
		for index := uint32(0); index < count; index++ {
			if uint64(next)+23 > uint64(end) {
				return fail("truncated definition header")
			}
			length := word(next + 2)
			if length < 27 || uint64(next)+uint64(length) > uint64(end) {
				return fail("definition record length")
			}
			number := uint16(word(next) & 0x3ff)
			if seen[number] {
				return fail("duplicate definition number")
			}
			seen[number] = true
			for _, offset := range []uint32{1, 19, 20, 21} {
				if word(next+offset) != 0 {
					return fail("unqualified definition reserved field")
				}
			}
			code := copyWords(next+23, length-23)
			if code[0] != 0xb0400809 || code[1]&0xffff0000 != 0x65440000 || code[len(code)-2] != 0xbb800805 || code[len(code)-1] != 0x80000000 {
				return fail("unqualified definition entry/return")
			}
			name := data[(next+3)*4 : (next+19)*4]
			ascii := strings.TrimRight(string(name), " \x00")
			for _, b := range []byte(ascii) {
				if b >= 0x80 {
					ascii = ""
					break
				}
			}
			result.Definitions = append(result.Definitions, N1FBDefinition{Number: number, Flags: word(next) &^ 0x3ff, BodyNumber: uint16(code[1]), NameASCII: ascii, NameHex: fmt.Sprintf("%x", name), OffsetWords: next, LengthWords: length, PortCountsWord: word(next + 22), Header: copyWords(next, 23), Code: code})
			next += length
		}
		if next != end {
			return fail("unexplained data after definitions")
		}
	}
	if instances != 0 {
		if instances != next || uint64(instances)+8 > uint64(directory) {
			return fail("instance section boundaries")
		}
		count := word(instances)
		if count == 0 || uint64(count)*14 > uint64(directory-instances-8) {
			return fail("instance count outside section")
		}
		result.InstanceSectionHeader = copyWords(instances+1, 7)
		next = instances + 8
		seen := map[uint16]bool{}
		for index := uint32(0); index < count; index++ {
			if uint64(next)+10 > uint64(directory) {
				return fail("truncated instance header")
			}
			length, number := word(next+2), word(next)
			if length < 14 || uint64(next)+uint64(length) > uint64(directory) {
				return fail("instance record length")
			}
			if number > 65535 || seen[uint16(number)] || word(next+1) != 0 {
				return fail("invalid or duplicate instance number/header")
			}
			seen[uint16(number)] = true
			code := copyWords(next+10, length-10)
			if code[0] != 0xb040080a || code[1] != 0x65440000|number || code[len(code)-2] != 0xbb800803 || code[len(code)-1] != 0x80000000 {
				return fail("unqualified instance entry/return")
			}
			result.Instances = append(result.Instances, N1FBInstance{Number: uint16(number), OffsetWords: next, LengthWords: length, Counts: copyWords(next+3, 7), Header: copyWords(next, 10), Code: code})
			next += length
		}
	}
	if next != directory {
		return fail("unexplained prefix data")
	}
	return result, nil
}
