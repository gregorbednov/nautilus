package um

import "encoding/binary"

// N1ServiceFields exposes an observed relation in four CJ1M/CJ1H readbacks.
// It is diagnostic evidence, not a qualified checksum/stamping specification.
// Word4 matches ceil(image bytes/64) in the same readbacks. Native execution
// context and finalization semantics remain separately unqualified.
type N1ServiceFields struct {
	Available               bool      `json:"available"`
	Words                   [6]uint32 `json:"words"`
	Sum32FromWord6          uint32    `json:"sum32FromWord6"`
	StoredPairMatches       bool      `json:"storedPairMatches"`
	Blocks64FromImageLength uint32    `json:"blocks64FromImageLength"`
	StoredBlockCountMatches bool      `json:"storedBlockCountMatches"`
	// Candidate counts expose unresolved length/quantum rules; matching one
	// does not select it as the CPU algorithm.
	BlockCountCandidates []N1BlockCountCandidate `json:"blockCountCandidates"`
	// Full target-format/stamping confirmation, not just arithmetic replay.
	NativeAlgorithmConfirmed bool `json:"nativeAlgorithmConfirmed"`
	WriteQualified           bool `json:"writeQualified"`
}

// N1BlockCountCandidate describes one diagnostic hypothesis for word4.
// These candidates are not a complete specification or a stamping policy.
type N1BlockCountCandidate struct {
	QuantumBytes        uint32 `json:"quantumBytes"`
	ExcludedPrefixBytes uint32 `json:"excludedPrefixBytes"`
	Count               uint32 `json:"count"`
	StoredMatches       bool   `json:"storedMatches"`
}

func blockCountCandidates(imageBytes int, stored uint32) []N1BlockCountCandidate {
	result := make([]N1BlockCountCandidate, 0, 4)
	for _, quantum := range []uint32{64, 256} {
		for _, excluded := range []uint32{0, 24} {
			count := (uint32(imageBytes) - excluded + quantum - 1) / quantum
			result = append(result, N1BlockCountCandidate{quantum, excluded, count, stored == count})
		}
	}
	return result
}

// ServiceFields sums big-endian32-bit words from index6 through the end,
// modulo2^32. Observed word0 equals the low16 bits and word1 the high16 bits.
// Compiler-file zeros are not silently replaced; a mismatch is reported rather
// than declaring a structurally valid compiler file corrupt. No bytes change.
func (image *N1Image) ServiceFields() N1ServiceFields {
	var report N1ServiceFields
	if image == nil || len(image.raw) < 24 || len(image.raw)%4 != 0 {
		return report
	}
	report.Available = true
	for i := range report.Words {
		report.Words[i] = binary.BigEndian.Uint32(image.raw[4*i:])
	}
	report.Sum32FromWord6 = sum32BE(image.raw[24:])
	report.StoredPairMatches = report.Words[0] == report.Sum32FromWord6&0xffff && report.Words[1] == report.Sum32FromWord6>>16
	report.Blocks64FromImageLength = uint32((len(image.raw) + 63) / 64)
	report.StoredBlockCountMatches = report.Words[4] == report.Blocks64FromImageLength
	report.BlockCountCandidates = blockCountCandidates(len(image.raw), report.Words[4])
	return report
}

// sum32BE ignores an incomplete trailing container, as the isolated native
// width2 accumulator does. Parsed N1 input is always aligned to four bytes.
func sum32BE(data []byte) uint32 {
	var sum uint32
	for offset := 0; offset+4 <= len(data); offset += 4 {
		sum += binary.BigEndian.Uint32(data[offset:])
	}
	return sum
}
