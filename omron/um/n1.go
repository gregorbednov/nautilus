// Package um inspects native compiler images without connecting to a PLC.
package um

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
)

const (
	MaxSize          = 16 << 20
	n1HeaderWords    = 0x14a
	n1DirectoryWords = 0x120
)

// ErrN1Layout includes unsupported variants, not just corrupt input.
var ErrN1Layout = errors.New("unsupported or malformed N1 layout")

// N1Image retains every input byte, including uninterpreted register/FB data.
// The supported layout was observed from CX-Programmer 9.6 CJ1H CPU65 and
// CJ1M CPU21 compiler captures.
// There is no identifying magic; callers must explicitly select N1 inspection.
// Acceptance does not establish CPU compatibility or a deployable wire payload.
type N1Image struct {
	Flags                uint32
	DirectoryOffsetWords uint32
	Tasks                []N1Task
	raw                  []byte
}

type N1Task struct {
	Number uint16
	Flags  uint32
	// NameASCII is empty for non-ASCII names. NameBytes preserves the original
	// code-page bytes without guessing a text encoding.
	NameASCII   string
	NameBytes   []byte
	OffsetWords uint32
	LengthWords uint32
	// IndependentIRDR identifies an emitted cyclic-task IR/DR prologue,
	// rather than the project's global UI setting for interrupt tasks.
	IndependentIRDR bool
	Header          []uint32
	// Code excludes the task header and compiler entry/exit wrapper. These
	// numeric containers are not decoded IL or PLC memory addresses.
	Code []uint32
}

type N1Report struct {
	Format                   string         `json:"format"`
	SHA256                   string         `json:"sha256"`
	ByteLength               int            `json:"byteLength"`
	Flags                    uint32         `json:"flags"`
	DirectoryOffsetWords     uint32         `json:"directoryOffsetWords"`
	UninterpretedPrefixBytes int            `json:"uninterpretedPrefixBytes"`
	Tasks                    []N1TaskReport `json:"tasks"`
	HardwareQualified        bool           `json:"hardwareQualified"`
}

type N1TaskReport struct {
	Number          uint16 `json:"number"`
	NameASCII       string `json:"nameASCII,omitempty"`
	NameHex         string `json:"nameHex"`
	Flags           uint32 `json:"flags"`
	OffsetWords     uint32 `json:"offsetWords"`
	LengthWords     uint32 `json:"lengthWords"`
	CodeWords       int    `json:"codeWords"`
	IndependentIRDR bool   `json:"independentIRDR"`
}

func (image *N1Image) Summary() N1Report {
	report := N1Report{
		Format: "cx-programmer-n1", SHA256: fmt.Sprintf("%x", sha256.Sum256(image.raw)),
		ByteLength: len(image.raw), Flags: image.Flags,
		DirectoryOffsetWords:     image.DirectoryOffsetWords,
		UninterpretedPrefixBytes: int(image.DirectoryOffsetWords)*4 - n1HeaderWords*4,
		Tasks:                    make([]N1TaskReport, 0, len(image.Tasks)),
	}
	for _, task := range image.Tasks {
		report.Tasks = append(report.Tasks, N1TaskReport{
			Number: task.Number, NameASCII: task.NameASCII, NameHex: fmt.Sprintf("%x", task.NameBytes),
			Flags: task.Flags, OffsetWords: task.OffsetWords, LengthWords: task.LengthWords,
			CodeWords: len(task.Code), IndependentIRDR: task.IndependentIRDR,
		})
	}
	return report
}

// Original returns a copy of the complete input. Editing decoded fields does
// not compile or rewrite an image: this is a read-only format inspector.
func (image *N1Image) Original() []byte { return bytes.Clone(image.raw) }

func ParseN1(data []byte) (*N1Image, error) {
	fail := func(reason string) (*N1Image, error) { return nil, fmt.Errorf("%w: %s", ErrN1Layout, reason) }
	if len(data) < (n1HeaderWords+n1DirectoryWords)*4 || len(data) > MaxSize || len(data)%4 != 0 {
		return fail("invalid image size or alignment")
	}
	word := func(index uint32) uint32 { return binary.BigEndian.Uint32(data[int(index)*4:]) }
	count := uint32(len(data) / 4)
	if word(0x20) != count || word(0x21) != 0x2a {
		return fail("header length or layout marker")
	}
	directory := word(0x22)
	if directory < n1HeaderWords || uint64(directory)+n1DirectoryWords > uint64(count) {
		return fail("task directory outside image")
	}
	for _, field := range []uint32{0x23, 0x24} {
		if offset := word(field); offset != 0 && (offset < n1HeaderWords || offset >= directory) {
			return fail("auxiliary section outside prefix")
		}
	}
	image := &N1Image{Flags: word(6), DirectoryOffsetWords: directory}
	for number := uint32(0); number < n1DirectoryWords; number++ {
		relative := word(directory + number)
		if relative == 0 {
			continue
		}
		start64 := uint64(directory) + uint64(relative)
		if relative < n1DirectoryWords || start64+32 > uint64(count) {
			return fail("task header outside code area")
		}
		start := uint32(start64)
		length := word(start + 0x1a)
		end64 := start64 + uint64(length)
		if length < 37 || end64 > uint64(count) || word(start+0x1b) != 32 {
			return fail("task record length or header size")
		}
		// Check before allocating decoded code: overlapping records must not
		// multiply a bounded input into hundreds of large allocations.
		for _, previous := range image.Tasks {
			if start64 < uint64(previous.OffsetWords)+uint64(previous.LengthWords) && end64 > uint64(previous.OffsetWords) {
				return fail("overlapping task records")
			}
		}
		if word(start)&0x3ff != number || word(start+32) != 0 || word(start+33) != 0xbbc00f43 ||
			word(start+34) != 0x61440000+number || word(start+35) != 0x65440000 ||
			word(uint32(end64)-1) != 0xbb800f48 {
			return fail("unsupported task entry/exit wrapper")
		}
		body := start + 36
		independent := false
		if number < 256 && length >= 39 && word(body) == 0x80400820 && word(body+1) == 0x653c0000 {
			independent = true
			body += 2
		}
		name := make([]byte, 0, 24)
		for index := start + 1; index < start+9; index++ {
			value := word(index)
			name = append(name, byte(value>>16), byte(value>>8), byte(value))
		}
		if nul := bytes.IndexByte(name, 0); nul >= 0 {
			name = name[:nul]
		}
		ascii := string(name)
		for _, value := range name {
			if value >= 0x80 {
				ascii = ""
				break
			}
		}
		header := make([]uint32, 32)
		for index := range header {
			header[index] = word(start + uint32(index))
		}
		code := make([]uint32, uint32(end64)-1-body)
		for index := range code {
			code[index] = word(body + uint32(index))
		}
		image.Tasks = append(image.Tasks, N1Task{Number: uint16(number), Flags: word(start) &^ uint32(0x3ff),
			NameASCII: ascii, NameBytes: name, OffsetWords: start, LengthWords: length,
			IndependentIRDR: independent, Header: header, Code: code})
	}
	// Reject overlapping records and unexplained gaps/trailing task data.
	ordered := append([]N1Task(nil), image.Tasks...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].OffsetWords < ordered[j].OffsetWords })
	next := directory + n1DirectoryWords
	for _, task := range ordered {
		if task.OffsetWords != next {
			return fail("overlapping tasks or unexplained code-area gap")
		}
		next += task.LengthWords
	}
	if next != count {
		return fail("unexplained data after tasks")
	}
	image.raw = bytes.Clone(data)
	return image, nil
}
