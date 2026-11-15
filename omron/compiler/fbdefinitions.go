package compiler

import (
	"fmt"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/internal/fbcode"
)

type FBDefinitionRecordSpan struct {
	Definition  string `json:"definition"`
	OffsetWords int    `json:"offsetWords"`
	LengthWords int    `json:"lengthWords"`
}

// FBDefinitionSection uses the observed count + three zero words header.
// Record numbers are explicit caller controls, independent of source TaskIDs.
type FBDefinitionSection struct {
	Resource    string                   `json:"resource"`
	Definitions []*FBDefinition          `json:"definitions"`
	Words       []uint32                 `json:"words"`
	Records     []FBDefinitionRecordSpan `json:"records"`
}

func CompileCXPFBDefinitionSection(profile codec.Profile, doc *cxp.Document, resource string, numbers map[string]int) (*FBDefinitionSection, error) {
	sources, err := doc.SourceForFBDefinitions(resource)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, fmt.Errorf("%w: empty definition section must be omitted by the image linker", fbcode.ErrDefinition)
	}
	if len(numbers) != len(sources) {
		return nil, fmt.Errorf("%w: exactly one explicit number per definition is required", fbcode.ErrDefinition)
	}
	seen := map[int]bool{}
	for _, source := range sources {
		number, ok := numbers[source.Name]
		if !ok || number < 0 || number > 1023 || seen[number] {
			return nil, fmt.Errorf("%w: missing, invalid or duplicate number for %s", fbcode.ErrDefinition, source.Name)
		}
		seen[number] = true
	}
	result := &FBDefinitionSection{Resource: resource, Definitions: []*FBDefinition{}, Words: make([]uint32, 4), Records: []FBDefinitionRecordSpan{}}
	result.Words[0] = uint32(len(sources))
	for _, source := range sources {
		definition, err := CompileCXPFBDefinition(profile, doc, source.Path, numbers[source.Name])
		if err != nil {
			return nil, err
		}
		if len(result.Words) > MaxInstructions-len(definition.RecordWords) {
			return nil, ErrSourceLimit
		}
		result.Records = append(result.Records, FBDefinitionRecordSpan{Definition: source.Path, OffsetWords: len(result.Words), LengthWords: len(definition.RecordWords)})
		result.Words = append(result.Words, definition.RecordWords...)
		result.Definitions = append(result.Definitions, definition)
	}
	return result, nil
}

// FBPrefix is a fragment containing definition and instance sections, without
// the N1 file header or tasks. Section offsets are relative to this fragment.
type FBPrefix struct {
	Definitions           *FBDefinitionSection `json:"definitions"`
	Instances             *FBInstanceSection   `json:"instances"`
	DefinitionOffsetWords int                  `json:"definitionOffsetWords"`
	InstanceOffsetWords   int                  `json:"instanceOffsetWords"`
	Words                 []uint32             `json:"words"`
}

// CompileCXPFBPrefix validates definition identities and matches instance source
// references to the generated definitions. Both sections must be nonempty.
// This does not check callers/tasks, choose IDs or initialize stored variables.
func CompileCXPFBPrefix(profile codec.Profile, doc *cxp.Document, resource string, numbers map[string]int, nextEnable [7]uint32) (*FBPrefix, error) {
	definitions, err := CompileCXPFBDefinitionSection(profile, doc, resource, numbers)
	if err != nil {
		return nil, err
	}
	instances, err := CompileCXPFBInstanceSection(profile, doc, resource, nextEnable)
	if err != nil {
		return nil, err
	}
	byPath := map[string]*FBDefinition{}
	for _, definition := range definitions.Definitions {
		byPath[definition.Definition] = definition
	}
	for _, instance := range instances.Instances.Instances {
		definition := byPath[instance.Source.DefinitionPath]
		if definition == nil || definition.Name != instance.Source.Definition {
			return nil, fmt.Errorf("%w: instance definition not generated", fbcode.ErrInstance)
		}
	}
	if len(definitions.Words) > MaxInstructions-len(instances.Words) {
		return nil, ErrSourceLimit
	}
	words := make([]uint32, 0, len(definitions.Words)+len(instances.Words))
	words = append(words, definitions.Words...)
	words = append(words, instances.Words...)
	return &FBPrefix{Definitions: definitions, Instances: instances, InstanceOffsetWords: len(definitions.Words), Words: words}, nil
}
