package codec

import "errors"

var ErrN1Header = errors.New("unsupported N1 header layout")

// Lengths describe independently generated sections, in 32-bit words. Zero
// omits an auxiliary section. TaskBlockWords includes its 288-word directory.
type N1HeaderLayout struct {
	DefinitionWords int `json:"definitionWords"`
	InstanceWords   int `json:"instanceWords"`
	TaskBlockWords  int `json:"taskBlockWords"`
}

// EncodeN1Header generates the 42 fixed words, excluding the separate 288-word
// task-configuration table. Scope is ordinary unprotected tasks and the qualified
// simple scalar FB records: no derived nested/string/protection flags, passwords
// or initialization data. Memory settings are explicit; configured memory is
// serialized only when definitions exist. This is not a complete image builder.
func EncodeN1Header(profile Profile, options N1TaskOptions, layout N1HeaderLayout, memory *N1FBMemorySettings) ([]uint32, error) {
	flags, err := EncodeN1TaskFlags(profile, options)
	if err != nil {
		return nil, err
	}
	const maxWords = 4 << 20 // 16 MiB in 32-bit words.
	for _, length := range []int{layout.DefinitionWords, layout.InstanceWords, layout.TaskBlockWords} {
		if length < 0 || length > maxWords {
			return nil, ErrN1Header
		}
	}
	if layout.TaskBlockWords < 288+38 || (layout.DefinitionWords != 0 && layout.DefinitionWords < 32) || (layout.InstanceWords != 0 && (layout.InstanceWords < 62 || layout.DefinitionWords == 0)) {
		return nil, ErrN1Header
	}
	total := 330 + int64(layout.DefinitionWords) + int64(layout.InstanceWords) + int64(layout.TaskBlockWords)
	if total > maxWords {
		return nil, ErrN1Header
	}
	var descriptor []uint32
	if memory != nil {
		descriptor, err = EncodeN1FBMemory(profile, *memory)
		if err != nil {
			return nil, err
		}
	} else if layout.DefinitionWords != 0 {
		return nil, ErrN1Header
	}
	words := make([]uint32, 42)
	words[6] = flags
	for i := 7; i < 15; i++ {
		words[i] = 0x00202020
	}
	if layout.DefinitionWords != 0 {
		copy(words[15:32], descriptor)
		words[35] = 330
	}
	if layout.InstanceWords != 0 {
		words[36] = 330 + uint32(layout.DefinitionWords)
	}
	words[32], words[33], words[34] = uint32(total), 42, 330+uint32(layout.DefinitionWords)+uint32(layout.InstanceWords)
	return words, nil
}
