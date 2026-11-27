package nbpanel

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
)

const (
	ModelName      = "NB10W-TW01B"
	modelIDNB10W   = 122
	maxPackageSize = 64 << 20
	pkgHeaderSize  = 86
	projectHeadLen = 160
	integrityField = 28
	integritySize  = 64
	integrityBytes = 0x4000
)

// NB encodes the digest into a table filled with padding. These indices
// are part of the PKGEXT 3 wire format.
var digestSource = [md5.Size]int{3, 29, 19, 61, 37, 52, 43, 37, 12, 24, 8, 55, 63, 5, 49, 21}
var digestDestination = [md5.Size]int{2, 16, 25, 11, 32, 14, 48, 57, 9, 62, 13, 22, 44, 33, 10, 60}

type Package struct {
	ModelID uint16
	Screens []Screen
	data    []byte
	project int
	hmi     int
	hmiEnd  int
}

// ReadPackage reads the one-terminal PKGEXT 3 layout produced for NB10W.
func ReadPackage(filename string) (*Package, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParsePackage(f)
}

func ParsePackage(source io.Reader) (*Package, error) {
	data, err := io.ReadAll(io.LimitReader(source, maxPackageSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) < pkgHeaderSize || len(data) > maxPackageSize ||
		u32(data, 0) != 1 || string(data[16:22]) != "PKGEXT" ||
		u32(data, 22) != 3 || u32(data, 26) != 3 {
		return nil, fmt.Errorf("expected a single-terminal Omron PKGEXT 3 package")
	}
	project := pkgHeaderSize
	for i := range 3 {
		at := 30 + i*16
		size := u32(data, at+8)
		if u32(data, at) != i+1 || u32(data, at+12) != project ||
			size < 4 || size > len(data)-project ||
			string(data[project:project+4]) != "\x7fELF" {
			return nil, fmt.Errorf("invalid terminal runtime entry %d", i+1)
		}
		project += size
	}
	if project > len(data)-projectHeadLen || u32(data, 8) != project ||
		u32(data, 12) != project || u32(data, project) != project+12 {
		return nil, fmt.Errorf("invalid package project offset")
	}
	modelID := little.Uint16(data[project+12:])
	if modelID != modelIDNB10W {
		return nil, fmt.Errorf("unsupported panel model ID %d, want %s", modelID, ModelName)
	}
	nameAt, hmiAt, hmiSize := u32(data, project+16), u32(data, project+24), u32(data, project+20)
	if nameAt < project+projectHeadLen || nameAt > len(data)-6 ||
		hmiAt != nameAt+6 || string(data[nameAt:hmiAt]) != "NB.dat" ||
		hmiSize < integrityBytes || hmiSize > len(data)-hmiAt ||
		u32(data, project+104) != hmiAt+hmiSize {
		return nil, fmt.Errorf("invalid compiled HMI resource range")
	}
	tail, trailer := u32(data, project+4), u32(data, project+8)
	if tail < hmiAt+hmiSize || trailer != tail+16 || trailer != len(data)-12 {
		return nil, fmt.Errorf("invalid package footer")
	}
	hmi := data[hmiAt : hmiAt+hmiSize]
	digest := md5.Sum(hmi[:integrityBytes]) // Required by the Omron file format.
	table := data[project+integrityField : project+integrityField+integritySize]
	for i, to := range digestDestination {
		if table[to]-byte(i)*table[digestSource[i]] != digest[i] {
			return nil, fmt.Errorf("invalid compiled HMI digest")
		}
	}
	screens, err := ParseHMI(hmi)
	if err != nil {
		return nil, fmt.Errorf("compiled HMI resource: %w", err)
	}
	return &Package{
		ModelID: modelID, Screens: screens, data: data,
		project: project, hmi: hmiAt, hmiEnd: hmiAt + hmiSize,
	}, nil
}

func writeHMIIntegrity(pkg, image []byte, project int) {
	table := pkg[project+integrityField : project+integrityField+integritySize]
	digest := md5.Sum(image[:integrityBytes])
	for i, to := range digestDestination {
		table[to] = byte(i)*table[digestSource[i]] + digest[i]
	}
}
