package compiler

import (
	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/il"
)

// TaskConfiguration is the288-word header table at word42, distinct from the
// task directory. Unused slots retain native task-class/ID defaults.
type TaskConfiguration struct {
	Resource          string            `json:"resource"`
	HeaderOffsetWords int               `json:"headerOffsetWords"`
	Tasks             []*cxp.TaskSource `json:"tasks"`
	Words             []uint32          `json:"words"`
}

func makeTaskConfiguration(resource string, tasks []*cxp.TaskSource) *TaskConfiguration {
	words := make([]uint32, 288)
	for number := range words {
		class := uint32(0x2000)
		if number >= 256 {
			class = 0x1000
		}
		words[number] = class | uint32(number)
	}
	for _, task := range tasks {
		words[task.Number] |= 0x8000
		if task.OperationStart == 1 {
			words[task.Number] |= 0x4000
		}
	}
	return &TaskConfiguration{Resource: resource, HeaderOffsetWords: 42, Tasks: tasks, Words: words}
}

// CompileCXPTaskConfiguration compiles metadata only, not task bodies or a full
// header. The same bounded unprotected256..287 scope as TaskRecord is required.
func CompileCXPTaskConfiguration(profile codec.Profile, doc *cxp.Document, resource string) (*TaskConfiguration, error) {
	if profile.TimerCounterMode != codec.TimerModeBCD {
		return nil, codec.ErrUnqualifiedProfile
	}
	if _, err := codec.EncodeInstruction(profile, il.Instruction{Mnemonic: "NOP"}); err != nil {
		return nil, err
	}
	tasks, err := doc.SourceForTasks(resource)
	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return nil, ErrStructure
	}
	return makeTaskConfiguration(resource, tasks), nil
}
