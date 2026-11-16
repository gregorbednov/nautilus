package codec

import (
	"errors"
	"strconv"
	"strings"
)

var ErrN1FBMemory = errors.New("unqualified N1 FB memory settings")

type N1MemoryBase struct {
	Address string `json:"address"`
	Size    int    `json:"size"`
}

// Sizes use native word-address units, not the CXP fields' inferred units.
type N1MemoryStack struct {
	Address   string `json:"address"`
	FirstSize int    `json:"firstSize"`
	TotalSize int    `json:"totalSize"`
}

type N1FBMemorySettings struct {
	// Exactly five explicit bases, in native bank order. Banks 5/6 are unused
	// in this scope and their four descriptor words remain zero.
	Bases []N1MemoryBase `json:"bases"`
	Stack N1MemoryStack  `json:"stack"`
}

func memoryIndex(address, prefix string, low, end int) (int, bool) {
	if !strings.HasPrefix(address, prefix) {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(address, prefix))
	return n, err == nil && n >= low && n < end && address == prefix+strconv.Itoa(n)
}

// EncodeN1FBMemory computes the 17-word configured memory descriptor used at
// N1 header words 15..31 when FB definitions are emitted. The native writer
// leaves those words zero without definitions, even if FB mode is enabled.
// This does not decide whether to emit them, allocate instances, audit overlap,
// initialize storage, or infer settings from CXP. BCD and the observed bounded
// T/C/DIFF/H banks with AC/D stack regions are the qualified scope.
func EncodeN1FBMemory(profile Profile, settings N1FBMemorySettings) ([]uint32, error) {
	if err := profile.validate(); err != nil {
		return nil, err
	}
	if profile.TimerCounterMode != TimerModeBCD {
		return nil, ErrUnqualifiedProfile
	}
	if len(settings.Bases) != 5 {
		return nil, ErrN1FBMemory
	}
	words := make([]uint32, 17)
	banks := []struct {
		prefix   string
		low, end int
		base     uint32
	}{{"T", 3072, 4096, 0xe000}, {"C", 3072, 4096, 0xf000}, {"DIFF", 0, 8192, 0x8000}, {"H", 512, 1408, 0xd800}, {"H", 1408, 1536, 0xd800}}
	for i, bank := range banks {
		setting := settings.Bases[i]
		n, ok := memoryIndex(setting.Address, bank.prefix, bank.low, bank.end)
		if !ok || setting.Size < 0 || setting.Size > bank.end-n || (i == 2 && setting.Size != 0) {
			return nil, ErrN1FBMemory
		}
		words[2*i], words[2*i+1] = bank.base+uint32(n), uint32(setting.Size)
	}
	stack := settings.Stack
	var start uint32
	capacity := 0
	if n, ok := memoryIndex(stack.Address, "AC", 0, 1536); ok {
		start = 0xb200 + uint32(n)
		capacity = 1536 - n
	} else if n, ok := memoryIndex(stack.Address, "D", 0, 32768); ok {
		start = 0x10000 + uint32(n)
		capacity = 32768 - n
	} else {
		return nil, ErrN1FBMemory
	}
	if stack.FirstSize < 0 || stack.TotalSize <= 0 || stack.TotalSize > 4096 || stack.TotalSize > capacity || stack.FirstSize > stack.TotalSize {
		return nil, ErrN1FBMemory
	}
	words[14], words[15], words[16] = start, start+uint32(stack.FirstSize), uint32(stack.TotalSize)
	return words, nil
}
