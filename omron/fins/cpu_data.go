package fins

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

// DecodeIdentityData decodes 0501 response DATA, without command/end code.
// Version is internal system version, not a decoded unit-version identifier.
func DecodeIdentityData(raw []byte) (Identity, error) {
	if len(raw) < 40 {
		return Identity{}, errors.New("short CPU UNIT DATA response")
	}
	trim := func(b []byte) string { return strings.Trim(string(b), " \x00") }
	return Identity{Model: trim(raw[:20]), Version: trim(raw[20:40]), Raw: bytes.Clone(raw)}, nil
}

func DecodeStatusData(raw []byte) (Status, error) {
	if len(raw) < 26 {
		return Status{}, errors.New("short CPU UNIT STATUS response")
	}
	if raw[1] != 0 && raw[1] != 2 && raw[1] != 4 {
		return Status{}, fmt.Errorf("unknown CPU mode %02X", raw[1])
	}
	return Status{Status: raw[0], Mode: raw[1], Fatal: binary.BigEndian.Uint16(raw[2:4]), NonFatal: binary.BigEndian.Uint16(raw[4:6]), Raw: bytes.Clone(raw)}, nil
}

// CPUUnitData describes only the documented base fields of 0501 DATA.
// Reserved bytes and all data after the 92-byte base remain uninterpreted.
// ReportedProgramAreaBytes is a unit conversion, not N1 admissibility proof.
type CPUUnitData struct {
	Model                    string `json:"model"`
	InternalSystemVersion    string `json:"internalSystemVersion"`
	PayloadBytes             int    `json:"payloadBytes"`
	PayloadSHA256            string `json:"payloadSHA256"`
	DIPSwitchBits            byte   `json:"dipSwitchBits"`
	LargestEMBank            byte   `json:"largestEMBank"`
	ProgramAreaKWords        uint16 `json:"programAreaKWords"`
	ReportedProgramAreaBytes uint32 `json:"reportedProgramAreaBytes"`
	IOMKBytes                byte   `json:"iomKBytes"`
	DMWords                  uint16 `json:"dmWords"`
	TimerCounterUnits1024    byte   `json:"timerCounterUnits1024"`
	EMNonFileBanks           byte   `json:"emNonFileBanks"`
	MemoryCardType           byte   `json:"memoryCardType"`
	MemoryCardKBytes         uint16 `json:"memoryCardKBytes"`
	UninterpretedTailBytes   int    `json:"uninterpretedTailBytes"`
}

// DecodeCPUUnitData accepts a base-bearing 0501 DATA response. A 92-byte
// prefix cannot establish that a full response or extension was captured.
// W342-E1-18 section 5-3-15, printed pages 191-192: 20+20+40+12 bytes.
func DecodeCPUUnitData(raw []byte) (CPUUnitData, error) {
	if len(raw) < 92 {
		return CPUUnitData{}, errors.New("CPU UNIT DATA base requires at least 92 bytes")
	}
	identity, err := DecodeIdentityData(raw)
	if err != nil {
		return CPUUnitData{}, err
	}
	program := binary.BigEndian.Uint16(raw[80:82])
	return CPUUnitData{
		Model: identity.Model, InternalSystemVersion: identity.Version, PayloadBytes: len(raw), PayloadSHA256: fmt.Sprintf("%x", sha256.Sum256(raw)),
		DIPSwitchBits: raw[40], LargestEMBank: raw[41], ProgramAreaKWords: program, ReportedProgramAreaBytes: uint32(program) * 1024 * 2,
		IOMKBytes: raw[82], DMWords: binary.BigEndian.Uint16(raw[83:85]), TimerCounterUnits1024: raw[85], EMNonFileBanks: raw[86],
		MemoryCardType: raw[89], MemoryCardKBytes: binary.BigEndian.Uint16(raw[90:92]), UninterpretedTailBytes: len(raw) - 92,
	}, nil
}
