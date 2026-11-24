package compiler

import (
	"fmt"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/cxp"
)

type FBCallLink struct {
	Program        string `json:"program"`
	Rung           string `json:"rung"`
	Instance       string `json:"instance"`
	InstanceNumber int    `json:"instanceNumber"`
	Definition     string `json:"definition"`
	BodyNumber     int    `json:"bodyNumber"`
}

// ResourceBody is the observed FB prefix + task block fragment. Offsets are
// relative to this fragment. TaskConfiguration is a separate header table;
// Words do not include it or the rest of the global N1 header.
type ResourceBody struct {
	FBPrefix                 *FBPrefix          `json:"fbPrefix"`
	TaskBlock                *TaskBlock         `json:"taskBlock"`
	TaskConfiguration        *TaskConfiguration `json:"taskConfiguration"`
	DefinitionOffsetWords    int                `json:"definitionOffsetWords"`
	InstanceOffsetWords      int                `json:"instanceOffsetWords"`
	TaskDirectoryOffsetWords int                `json:"taskDirectoryOffsetWords"`
	Calls                    []FBCallLink       `json:"calls"`
	Words                    []uint32           `json:"words"`
}

func CompileCXPResourceBody(profile codec.Profile, doc *cxp.Document, resource string, numbers map[string]int, nextEnable [7]uint32) (*ResourceBody, error) {
	return compileCXPResourceBody(profile, doc, resource, numbers, nextEnable, true)
}

func compileCXPResourceBody(profile codec.Profile, doc *cxp.Document, resource string, numbers map[string]int, nextEnable [7]uint32, sectionMarkers bool) (*ResourceBody, error) {
	prefix, err := compileResourceFBPrefix(profile, doc, resource, numbers, nextEnable)
	if err != nil {
		return nil, err
	}
	tasks, err := compileCXPTaskBlock(profile, doc, resource, sectionMarkers)
	if err != nil {
		return nil, err
	}
	definitions := map[string]*FBDefinition{}
	instances := map[string]*FBInstance{}
	taskIDs := map[int]bool{}
	var taskSources []*cxp.TaskSource
	for _, task := range tasks.Tasks {
		taskIDs[task.Source.Number] = true
		taskSources = append(taskSources, task.Source)
	}
	for _, definition := range prefix.Definitions.Definitions {
		if taskIDs[definition.BodyNumber] {
			return nil, fmt.Errorf("%w: overlapping FB body and task numbers require separate qualification", ErrStructure)
		}
		definitions[definition.Definition] = definition
	}
	for _, instance := range prefix.Instances.Instances.Instances {
		instances[instance.Source.Name] = instance
	}
	result := &ResourceBody{FBPrefix: prefix, TaskBlock: tasks, InstanceOffsetWords: prefix.InstanceOffsetWords, TaskDirectoryOffsetWords: len(prefix.Words), Calls: []FBCallLink{}}
	result.TaskConfiguration = makeTaskConfiguration(resource, taskSources)
	for _, task := range tasks.Tasks {
		source, err := doc.SourceForProgram(task.Source.Path)
		if err != nil {
			return nil, err
		}
		for _, section := range source.Sections {
			for _, rung := range section.Rungs {
				if len(rung.Graphic) == 0 {
					continue
				}
				plan, err := doc.PlanFBCall(task.Source.Path, rung.Path)
				if err != nil {
					return nil, err
				}
				instance, definition := instances[plan.Instance], definitions[plan.DefinitionPath]
				if instance == nil || definition == nil || instance.Source.Number != plan.InstanceNumber || instance.Source.DefinitionPath != definition.Definition || definition.BodyNumber != plan.TaskID {
					return nil, fmt.Errorf("%w: call does not resolve to generated FB records", ErrStructure)
				}
				result.Calls = append(result.Calls, FBCallLink{Program: task.Source.Path, Rung: rung.Path, Instance: plan.Instance, InstanceNumber: plan.InstanceNumber, Definition: plan.DefinitionPath, BodyNumber: plan.TaskID})
			}
		}
	}
	if len(prefix.Words) > MaxInstructions-len(tasks.Words) {
		return nil, ErrSourceLimit
	}
	result.Words = make([]uint32, 0, len(prefix.Words)+len(tasks.Words))
	result.Words = append(result.Words, prefix.Words...)
	result.Words = append(result.Words, tasks.Words...)
	return result, nil
}

// Empty FB sections are absent on disk, rather than count-only records. Keep
// the standalone FB-prefix API's nonempty contract and make omission a linker
// decision after auditing all saved instances and FB symbol declarations.
func compileResourceFBPrefix(profile codec.Profile, doc *cxp.Document, resource string, numbers map[string]int, nextEnable [7]uint32) (*FBPrefix, error) {
	definitions, err := doc.SourceForFBDefinitions(resource)
	if err != nil {
		return nil, err
	}
	if len(definitions) != 0 {
		return CompileCXPFBPrefix(profile, doc, resource, numbers, nextEnable)
	}
	if len(numbers) != 0 || nextEnable != ([7]uint32{}) {
		return nil, fmt.Errorf("%w: resources without FB require no definition numbers and zero next-enable state", ErrStructure)
	}
	instances, err := CompileCXPFBInstances(profile, doc, resource)
	if err != nil {
		return nil, err
	}
	if len(instances.Instances) != 0 {
		return nil, fmt.Errorf("%w: instances require generated definitions", ErrStructure)
	}
	return &FBPrefix{
		Definitions: &FBDefinitionSection{Resource: resource, Definitions: []*FBDefinition{}, Words: []uint32{}, Records: []FBDefinitionRecordSpan{}},
		Instances:   &FBInstanceSection{Instances: instances, Words: []uint32{}},
		Words:       []uint32{},
	}, nil
}
