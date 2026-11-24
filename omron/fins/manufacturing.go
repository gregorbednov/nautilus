package fins

import (
	"bytes"
	"context"
	"errors"
	"fmt"
)

// CPUManufacturingData retains 0226 selector0/index0 DATA. Raw includes all
// unparsed fields and extensions. The version observations reproduce the
// researched IOTableCS Ex475 consumers, not an independently verified CPU
// firmware/profile selection. They must not automatically select0307/0317.
type CPUManufacturingData struct {
	Raw                            []byte  `json:"raw"`
	VersionBytes                   [3]byte `json:"versionBytes"`
	DateYYMMBytes                  [4]byte `json:"dateYYMMBytes"`
	CXVersionDisplay               string  `json:"cxVersionDisplay"`
	CXRevisionDisplay              string  `json:"cxRevisionDisplay"`
	NativeVersionObjectGuardPassed bool    `json:"nativeVersionObjectGuardPassed"`
}

// DecodeCPUManufacturingData accepts DATA without command/end code. The local
// upper bound1000 is a defensive decoder bound, not a native exact/max length.
// The native date guard is not calendar validation: e.g. YYMM8999 passes.
// Dialog display is independent of that guard; a zero triplet remains0.0/0.
func DecodeCPUManufacturingData(raw []byte) (CPUManufacturingData, error) {
	if len(raw) < 26 || len(raw) > 1000 {
		return CPUManufacturingData{}, errors.New("CPU manufacturing DATA outside26..1000 bytes")
	}
	result := CPUManufacturingData{Raw: bytes.Clone(raw)}
	copy(result.VersionBytes[:], raw[21:24])
	copy(result.DateYYMMBytes[:], raw[6:10])
	v := result.VersionBytes
	blank := v == [3]byte{' ', ' ', ' '}
	if blank {
		result.CXVersionDisplay, result.CXRevisionDisplay = "-", "-"
	} else {
		result.CXVersionDisplay = fmt.Sprintf("%X.%X", v[0], v[1])
		result.CXRevisionDisplay = fmt.Sprintf("%X", v[2])
	}
	d := result.DateYYMMBytes
	digits := true
	for _, b := range d {
		digits = digits && b >= '0' && b <= '9'
	}
	if digits {
		year := int(d[0]-'0')*10 + int(d[1]-'0')
		month := int(d[2]-'0')*10 + int(d[3]-'0')
		result.NativeVersionObjectGuardPassed = !blank && year >= 3 && year < 90 && (year != 3 || month >= 12)
	}
	return result, nil
}

// ReadCPUManufacturingData sends exactly0226/0000/0000, with no retries or
// selector search. This read route is researched for CJ1M/CJ1H; callers must
// establish the target model separately. It does not read protection state or
// qualify executable transfer. Route/SID/command/end code are checked before
// decoding, and malformed payloads terminate the session.
func (c *Client) ReadCPUManufacturingData(ctx context.Context) (CPUManufacturingData, error) {
	limit := c.maxReadData()
	if limit < 26 {
		return CPUManufacturingData{}, errors.New("transport cannot carry CPU manufacturing DATA")
	}
	var result CPUManufacturingData
	_, err := c.callValidated(ctx, 0x0226, []byte{0, 0, 0, 0}, func(raw []byte) error {
		if len(raw) > limit {
			return errors.New("CPU manufacturing DATA exceeds transport limit")
		}
		var err error
		result, err = DecodeCPUManufacturingData(raw)
		return err
	})
	if err != nil {
		return CPUManufacturingData{}, err
	}
	return result, nil
}
