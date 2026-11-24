package compiler

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/il"
	"github.com/joyautomation/nautilus/omron/internal/fbcode"
)

// FBInstances is an audited collection of standalone global instance records.
// It has no section header, storage initialization or task/N1 linking. Checks
// cover instance-to-instance overlap, not other symbols or instruction operands.
type FBInstances struct {
	Resource  string             `json:"resource"`
	Instances []*FBInstance      `json:"instances"`
	Regions   []FBInstanceRegion `json:"regions"`
}

// FBInstanceRegion is a half-open absolute word range, in a named PLC area.
// Timer slots are counted as words, consistent with the saved BR0 footprint.
type FBInstanceRegion struct {
	Instance     string `json:"instance"`
	BaseRegister int    `json:"baseRegister"`
	Area         string `json:"area"`
	StartWord    int    `json:"startWord"`
	EndWord      int    `json:"endWord"`
}

func CompileCXPFBInstances(profile codec.Profile, doc *cxp.Document, resource string) (*FBInstances, error) {
	// Validate the CPU even for an empty set.
	if profile.TimerCounterMode != codec.TimerModeBCD && profile.TimerCounterMode != codec.TimerModeBinary {
		return nil, fbcode.ErrInstance
	}
	if _, err := codec.EncodeInstruction(profile, il.Instruction{Mnemonic: "NOP"}); err != nil {
		return nil, err
	}
	sources, err := doc.SourceForFBInstances(resource)
	if err != nil {
		return nil, err
	}
	// Empty audited collections emit no FB record. Binary FB instances still
	// require separate qualification; never discard saved instances to proceed.
	if profile.TimerCounterMode != codec.TimerModeBCD && len(sources) != 0 {
		return nil, fbcode.ErrInstance
	}
	result := &FBInstances{Resource: resource, Instances: make([]*FBInstance, 0, len(sources)), Regions: []FBInstanceRegion{}}
	for _, source := range sources {
		instance, err := CompileCXPFBInstance(profile, doc, source.Path)
		if err != nil {
			return nil, err
		}
		result.Instances = append(result.Instances, instance)
		for _, bank := range []int{0, 3} {
			base := source.Bases[bank]
			if base.Size == 0 {
				continue
			}
			start, err := strconv.Atoi(base.Address[1:])
			if err != nil {
				return nil, err
			}
			result.Regions = append(result.Regions, FBInstanceRegion{Instance: source.Path, BaseRegister: bank, Area: base.Address[:1], StartWord: start, EndWord: start + base.Size})
		}
	}
	// Sort a copy so the report retains source and base-register order.
	regions := append([]FBInstanceRegion(nil), result.Regions...)
	sort.Slice(regions, func(i, j int) bool {
		if regions[i].Area != regions[j].Area {
			return regions[i].Area < regions[j].Area
		}
		return regions[i].StartWord < regions[j].StartWord
	})
	for i := 1; i < len(regions); i++ {
		a, b := regions[i-1], regions[i]
		if a.Area == b.Area && b.StartWord < a.EndWord {
			return nil, fmt.Errorf("%w: %s [%d,%d) overlaps %s [%d,%d) in %s", fbcode.ErrInstance, a.Instance, a.StartWord, a.EndWord, b.Instance, b.StartWord, b.EndWord, a.Area)
		}
	}
	return result, nil
}
