package compiler

import (
	"fmt"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/internal/fbcode"
)

type FBInstanceRecordSpan struct {
	Instance    string `json:"instance"`
	OffsetWords int    `json:"offsetWords"`
	LengthWords int    `json:"lengthWords"`
}

// FBInstanceSection adds the observed eight-word section header to an audited
// record set. Offsets are relative to this section, not a complete N1 image.
type FBInstanceSection struct {
	Instances           *FBInstances           `json:"instances"`
	NextEnableAddresses [7]uint32              `json:"nextEnableAddresses"`
	Words               []uint32               `json:"words"`
	Records             []FBInstanceRecordSpan `json:"records"`
}

// CompileCXPFBInstanceSection requires explicit next-enable state. Only the
// all-zero variant observed in both saved images is qualified. Nonzero states
// accepted by the native setter are deliberately not generalized to executable
// project semantics. No storage initialization or task/image linking is done.
func CompileCXPFBInstanceSection(profile codec.Profile, doc *cxp.Document, resource string, nextEnable [7]uint32) (*FBInstanceSection, error) {
	for _, value := range nextEnable {
		if value != 0 {
			return nil, fmt.Errorf("%w: nonzero next-enable state is not qualified", fbcode.ErrInstance)
		}
	}
	instances, err := CompileCXPFBInstances(profile, doc, resource)
	if err != nil {
		return nil, err
	}
	if len(instances.Instances) == 0 {
		return nil, fmt.Errorf("%w: empty instance section must be omitted by the image linker", fbcode.ErrInstance)
	}
	result := &FBInstanceSection{Instances: instances, NextEnableAddresses: nextEnable, Words: make([]uint32, 8), Records: []FBInstanceRecordSpan{}}
	result.Words[0] = uint32(len(instances.Instances))
	copy(result.Words[1:], nextEnable[:])
	for _, instance := range instances.Instances {
		if len(result.Words) > MaxInstructions-len(instance.RecordWords) {
			return nil, ErrSourceLimit
		}
		result.Records = append(result.Records, FBInstanceRecordSpan{Instance: instance.Source.Path, OffsetWords: len(result.Words), LengthWords: len(instance.RecordWords)})
		result.Words = append(result.Words, instance.RecordWords...)
	}
	return result, nil
}
