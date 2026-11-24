package transfer_test

import (
	"errors"
	"testing"

	"github.com/joyautomation/nautilus/omron/transfer"
	"github.com/joyautomation/nautilus/omron/um"
)

func TestTaskRecordIsNotAWholeProgramImage(t *testing.T) {
	source := nativeImage(t)
	image, err := um.ParseN1(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(image.Tasks) == 0 {
		t.Fatal("fixture has no task")
	}
	record, err := um.ExtractN1TaskRecord(source, image.Tasks[0].Number)
	if err != nil {
		t.Fatal(err)
	}
	if plan, err := transfer.PlanProgramWrite(record, 996); plan != nil || !errors.Is(err, um.ErrN1Layout) {
		t.Fatalf("task record accepted by whole-program planner: %v", err)
	}
	plan, err := transfer.PlanProgramWrite(source, 996)
	if err != nil {
		t.Fatal(err)
	}
	if plan.NetworkAccess || plan.WireImageMappingQualified || plan.WriteQualified {
		t.Fatal("extraction qualified a hardware transfer")
	}
}
