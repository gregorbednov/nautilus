package um

import (
	"fmt"
	"slices"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/il"
)

const maxTaskDecodeInstructions = 65536

type N1DecodedInstruction struct {
	CodeOffsetWords  uint32         `json:"codeOffsetWords"`
	ImageOffsetWords uint32         `json:"imageOffsetWords"`
	Words            []uint32       `json:"words"`
	Instruction      il.Instruction `json:"instruction"`
}

type N1DecodedTask struct {
	Number                         uint16                 `json:"number"`
	CodeOffsetWords                uint32                 `json:"codeOffsetWords"`
	CodeWords                      int                    `json:"codeWords"`
	DecodedWords                   int                    `json:"decodedWords"`
	Complete                       bool                   `json:"complete"`
	DecodedPrefixRoundTripVerified bool                   `json:"decodedPrefixRoundTripVerified"`
	Instructions                   []N1DecodedInstruction `json:"instructions"`
	StopReason                     string                 `json:"stopReason,omitempty"`
	OpaqueTail                     []uint32               `json:"opaqueTail,omitempty"`
}

type N1TaskDecoding struct {
	Model               string          `json:"model"`
	TimerCounterMode    string          `json:"timerCounterMode"`
	AllTaskCodeDecoded  bool            `json:"allTaskCodeDecoded"`
	SourceRestored      bool            `json:"sourceRestored"`
	SymbolNamesRestored bool            `json:"symbolNamesRestored"`
	HardwareQualified   bool            `json:"hardwareQualified"`
	Tasks               []N1DecodedTask `json:"tasks"`
}

// DecodeTasks walks each task's instruction body at proven boundaries. Unknown
// forms stop that task: the entire remaining tail is retained, without scanning
// for plausible opcodes inside operands. Each decoded instruction must encode
// back to its exact original words. Complete means body coverage, not restored
// rung/section structure, source names, valid control flow or safe execution.
// An explicit BCD profile is required; it is a caller assertion, not inferred
// CPU unit version or source-setting provenance. FB definitions remain separate.
func (image *N1Image) DecodeTasks(profile codec.Profile) (*N1TaskDecoding, error) {
	if profile.TimerCounterMode != codec.TimerModeBCD {
		return nil, codec.ErrUnqualifiedProfile
	}
	if _, err := codec.EncodeInstruction(profile, il.Instruction{Mnemonic: "NOP"}); err != nil {
		return nil, err
	}
	if image == nil {
		return nil, ErrN1Layout
	}
	// Public diagnostic fields are mutable; derive code/offsets from the retained
	// original image rather than trusting edits to Tasks or Code by a caller.
	original, err := ParseN1(image.raw)
	if err != nil {
		return nil, err
	}
	if original.Flags&0x40000000 != 0 {
		return nil, fmt.Errorf("%w: image declares binary timer mode", codec.ErrUnqualifiedProfile)
	}
	result := &N1TaskDecoding{Model: profile.Model, TimerCounterMode: "bcd", AllTaskCodeDecoded: true, Tasks: []N1DecodedTask{}}
	remaining := maxTaskDecodeInstructions
	for _, task := range original.Tasks {
		start := task.OffsetWords + 36
		if task.IndependentIRDR {
			start += 2
		}
		decoded := N1DecodedTask{Number: task.Number, CodeOffsetWords: start, CodeWords: len(task.Code), Instructions: []N1DecodedInstruction{}}
		for decoded.DecodedWords < len(task.Code) {
			offset := decoded.DecodedWords
			if remaining == 0 {
				decoded.StopReason = "instruction report limit reached"
				break
			}
			instruction, count, err := codec.DecodeInstruction(profile, task.Code[offset:])
			if err != nil {
				if profile.Model == codec.CJ1MCPU21 && task.Code[offset] == 0xb040080d {
					if call, ok := decodeWordCall(task.Code[offset:], uint32(offset), start); ok {
						if remaining < len(call) {
							decoded.StopReason = "instruction report limit reached"
							break
						}
						decoded.Instructions = append(decoded.Instructions, call...)
						last := call[len(call)-1]
						decoded.DecodedWords = int(last.CodeOffsetWords) + len(last.Words)
						remaining -= len(call)
						continue
					}
				}
				decoded.StopReason = err.Error()
				break
			}
			if count < 1 || count > len(task.Code)-offset {
				return nil, fmt.Errorf("invalid decoded instruction length at task%d word%d", task.Number, offset)
			}
			words := task.Code[offset : offset+count]
			reencoded, err := codec.EncodeInstruction(profile, instruction)
			if err != nil || !slices.Equal(words, reencoded) {
				return nil, fmt.Errorf("instruction round-trip mismatch at task%d word%d", task.Number, offset)
			}
			decoded.Instructions = append(decoded.Instructions, N1DecodedInstruction{CodeOffsetWords: uint32(offset), ImageOffsetWords: start + uint32(offset), Words: slices.Clone(words), Instruction: instruction})
			decoded.DecodedWords += count
			remaining--
		}
		decoded.Complete = decoded.DecodedWords == len(task.Code)
		decoded.DecodedPrefixRoundTripVerified = len(decoded.Instructions) != 0
		if !decoded.Complete {
			result.AllTaskCodeDecoded = false
			decoded.OpaqueTail = slices.Clone(task.Code[decoded.DecodedWords:])
		}
		result.Tasks = append(result.Tasks, decoded)
	}
	return result, nil
}
