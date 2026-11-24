// Package transfer describes offline candidate wire plans, validates retained
// acknowledgements and audits serialized evidence. It has no network,
// PLC client, mode switching, access-right or execution API.
package transfer

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/joyautomation/nautilus/omron/um"
)

const MaxProgramWriteBytes = 996 // W342 section5-3-11; read limit is992.
const maxPlanBlocks = 65536

var ErrProgramPlan = errors.New("invalid offline program-write plan")

type ProgramBlock struct {
	OffsetBytes uint32 `json:"offsetBytes"`
	DataBytes   int    `json:"dataBytes"`
	Final       bool   `json:"final"`
	DataSHA256  string `json:"dataSHA256"`
	// PayloadHex excludes the FINS header, command code and end code.
	PayloadHex string `json:"payloadHex"`
}

type ProgramPlan struct {
	Format                    string             `json:"format"`
	Command                   string             `json:"command"`
	ImageSHA256               string             `json:"imageSHA256"`
	ImageBytes                int                `json:"imageBytes"`
	BlockBytes                int                `json:"blockBytes"`
	ServiceFields             um.N1ServiceFields `json:"serviceFields"`
	RequiresProgramMode       bool               `json:"requiresProgramMode"`
	NetworkAccess             bool               `json:"networkAccess"`
	ImageModified             bool               `json:"imageModified"`
	WireImageMappingQualified bool               `json:"wireImageMappingQualified"`
	WriteQualified            bool               `json:"writeQualified"`
	Unresolved                []string           `json:"unresolved"`
	Blocks                    []ProgramBlock     `json:"blocks"`
}

// PlanProgramWrite partitions retained N1 bytes into documented0307 payloads:
// FFFF/program, uint32 big-endian byte offset, uint16 byte count, data. Only the
// final count has bit15 set. This describes block layout, not a proven complete
// download workflow or N1-to-wire conversion. Unknown fields are not normalized.
// There is deliberately no Send/Execute method and no endpoint input.
func PlanProgramWrite(data []byte, blockBytes int) (*ProgramPlan, error) {
	if blockBytes < 4 || blockBytes > MaxProgramWriteBytes || blockBytes%4 != 0 {
		return nil, fmt.Errorf("%w: block size must be aligned4 in4..996", ErrProgramPlan)
	}
	image, err := um.ParseN1(data)
	if err != nil {
		return nil, err
	}
	data = image.Original()
	count := (len(data) + blockBytes - 1) / blockBytes
	if count > maxPlanBlocks {
		return nil, fmt.Errorf("%w: block count limit", ErrProgramPlan)
	}
	plan := &ProgramPlan{
		Format: "cj1-offline-program-write-plan", Command: "0307",
		ImageSHA256: fmt.Sprintf("%x", sha256.Sum256(data)), ImageBytes: len(data), BlockBytes: blockBytes,
		ServiceFields: image.ServiceFields(), RequiresProgramMode: true,
		Unresolved: []string{
			"native dispatch depends on PLC_KBase.IsExtendedProtect (service3/0307 versus service532/0317); target protection state and execution remain unqualified",
			"simulator resident block quantum256 versus observed64-byte count remains unmapped; target finalization is unqualified",
			"static native service preserves source file bytes; target format selection, capacity and CPU finalization remain unqualified",
			"initial data, access rights, recovery and write/readback execution remain unverified",
		}, Blocks: make([]ProgramBlock, 0, count),
	}
	for offset := 0; offset < len(data); {
		n := min(blockBytes, len(data)-offset)
		final := offset+n == len(data)
		payload := make([]byte, 8+n)
		binary.BigEndian.PutUint16(payload, 0xffff)
		binary.BigEndian.PutUint32(payload[2:], uint32(offset))
		length := uint16(n)
		if final {
			length |= 0x8000
		}
		binary.BigEndian.PutUint16(payload[6:], length)
		copy(payload[8:], data[offset:offset+n])
		plan.Blocks = append(plan.Blocks, ProgramBlock{uint32(offset), n, final, fmt.Sprintf("%x", sha256.Sum256(data[offset:offset+n])), hex.EncodeToString(payload)})
		offset += n
	}
	return plan, nil
}
