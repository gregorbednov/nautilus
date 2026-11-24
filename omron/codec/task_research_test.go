package codec_test

import "testing"

type taskPropertiesInput struct {
	Number     int    `json:"number"`
	Name       string `json:"name"`
	Protection int    `json:"protection"`
}
type taskPropertiesResult struct {
	InitialNumber, NumberResult, Number int
	Name                                string
	ProtectionResult, Protection        int
	TaskType                            *int
	ProgramType                         int
	RecordFlags                         uint32
	DescriptorTaskCount                 int
}

func TestTaskPropertiesAgainstNativeSetters(t *testing.T) {
	accepted, rejected := 0, 0
	// NOP acceptance is independent of SetTaskID acceptance in this probe.
	for _, row := range loadLibraryProbeResults(t, "task-properties-manifest.json", 34, "task-properties-v1") {
		p := row.TaskProperties
		if p.InitialNumber != 256 || p.DescriptorTaskCount != 288 || p.ProgramType != 0 || p.RecordFlags != 0 || !row.Accepted {
			t.Fatal("unexpected task defaults/flags", row.ID, p)
		}
		// Recover the request from the fixture identity for the boundary matrix;
		// other cases all explicitly select256.
		number := 256
		switch row.ID {
		case "id--2":
			number = -2
		case "id--1":
			number = -1
		case "id-0":
			number = 0
		case "id-31":
			number = 31
		case "id-32":
			number = 32
		case "id-255":
			number = 255
		case "id-257":
			number = 257
		case "id-287":
			number = 287
		case "id-288":
			number = 288
		case "id-289":
			number = 289
		case "id-290":
			number = 290
		case "id-65535":
			number = 65535
		}
		expected := number
		if number < -1 || number > 289 {
			rejected++
			expected = 256
			if p.NumberResult != 0 {
				t.Fatal("invalid setter accepted", row.ID)
			}
		} else {
			accepted++
			if p.NumberResult != 1 {
				t.Fatal("valid setter rejected", row.ID)
			}
		}
		if p.Number != expected {
			t.Fatal("setter readback differs", row.ID, p)
		}
		if expected >= 256 && expected <= 287 && (p.TaskType == nil || *p.TaskType != 4) {
			t.Fatal("qualified task class changed", row.ID)
		}
		if expected < 0 {
			if p.ProtectionResult != 0 || p.TaskType != nil {
				t.Fatal("unassigned task protection/type changed")
			}
		} else if p.ProtectionResult != 1 {
			t.Fatal("protection setter failed", row.ID)
		}
		protection := 0
		if row.ID == "protected" {
			protection = 1
		}
		if p.Protection != protection {
			t.Fatal("protection getter differs", row.ID)
		}
		wantName := "TaskA"
		switch row.ID {
		case "name-0":
			wantName = ""
		case "name-24":
			wantName = "AAAAAAAAAAAAAAAAAAAAAAAA"
		case "name-25":
			wantName = "AAAAAAAAAAAAAAAAAAAAAAAAA"
		}
		if p.Name != wantName {
			t.Fatal("name getter changed", row.ID)
		}
	}
	if accepted != 28 || rejected != 6 {
		t.Fatal("task setter coverage changed", accepted, rejected)
	}
}
