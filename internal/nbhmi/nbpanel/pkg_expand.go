package nbpanel

import (
	"bytes"
	"fmt"
)

// appendHMIData inserts HMI data before the package's other resources.
// NB.dat's internal offsets are relative to its start, so existing screen,
// bitmap and configuration pointers stay valid.
func (pkg *Package) appendHMIData(data, extra []byte) ([]byte, error) {
	growth := len(extra)
	if growth > maxPackageSize-len(data) {
		return nil, fmt.Errorf("expanded HMI exceeds the panel package size limit")
	}
	offsets, err := pkg.suffixOffsets(data)
	if err != nil {
		return nil, err
	}
	result := make([]byte, len(data)+growth)
	copy(result, data[:pkg.hmiEnd])
	copy(result[pkg.hmiEnd:], extra)
	copy(result[pkg.hmiEnd+growth:], data[pkg.hmiEnd:])

	for _, field := range offsets {
		target := field
		if target >= pkg.hmiEnd {
			target += growth
		}
		little.PutUint32(result[target:], uint32(u32(data, field)+growth))
	}
	little.PutUint32(result[pkg.project+20:], uint32(pkg.hmiEnd-pkg.hmi+growth))
	return result, nil
}

// suffixOffsets validates the NB10W resource layout before returning fields
// holding package-relative addresses in the part that will move.
func (pkg *Package) suffixOffsets(data []byte) ([]int, error) {
	const (
		resourceDirectorySize = 96
		driverEntrySize       = 12
	)
	invalid := fmt.Errorf("reference package has an unsupported layout for expanded HMI data")
	at := pkg.project
	tail, trailer := u32(data, at+4), u32(data, at+8)
	logo, resources, drivers := u32(data, at+104), u32(data, at+112), u32(data, at+120)
	module, helper, archive := u32(data, at+156), u32(data, at+132), u32(data, at+140)
	if logo != pkg.hmiEnd || resources < logo || drivers < resources ||
		module < drivers || helper < module || archive < helper || tail < archive ||
		u32(data, at+100) != resources-logo || u32(data, at+148) != drivers ||
		u32(data, at+152) != helper-module || u32(data, at+128) != archive-helper ||
		u32(data, at+136) != tail-archive || tail > len(data)-28 || trailer != tail+16 ||
		u32(data, at+108) != 1 || u32(data, at+116) != 0 {
		return nil, invalid
	}
	if helper-module < 4 || archive-helper < 4 || tail-archive < 7 ||
		!bytes.Equal(data[module:module+4], []byte("\x7fELF")) ||
		!bytes.Equal(data[helper:helper+4], []byte("\x7fELF")) ||
		!bytes.Equal(data[archive:archive+7], []byte("EVUNCOM")) {
		return nil, invalid
	}
	if drivers-resources < resourceDirectorySize || u32(data, resources) != 3 {
		return nil, invalid
	}
	first, second := u32(data, resources+12), u32(data, resources+20)
	if first != resources+resourceDirectorySize || second < first || second > drivers ||
		u32(data, resources+8) != second-first || u32(data, resources+16) != drivers-second ||
		len(bytes.Trim(data[resources+24:first], "\x00")) != 0 {
		return nil, invalid
	}
	offsets := []int{at + 4, at + 8, at + 104, at + 112, at + 120, at + 132, at + 140,
		at + 148, at + 156, resources + 12, resources + 20}
	count := u32(data, at+144)
	if count == 0 || count > (module-drivers)/driverEntrySize {
		return nil, invalid
	}
	next := drivers + count*driverEntrySize
	for i := range count {
		entry := drivers + i*driverEntrySize
		size := u32(data, entry+4)
		if u32(data, entry) != i+1 || u32(data, entry+8) != next || size == 0 || size > module-next {
			return nil, invalid
		}
		offsets = append(offsets, entry+8)
		next += size
	}
	if next != module {
		return nil, invalid
	}
	return offsets, nil
}
