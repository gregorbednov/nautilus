package main

import (
	"encoding/json"
	"testing"

	"github.com/joyautomation/nautilus/omron/compiler"
)

func TestOmronCompileResourceFragmentsCLI(t *testing.T) {
	for _, spec := range []struct{ file, model string }{{"cj1m.cxp", "CJ1M CPU21"}, {"cj1h.cxp", "CJ1H-H CPU65"}} {
		file := "../../omron/cxp/testdata/" + spec.file
		args := []string{"-model", spec.model, "-resource", "Resource[0]", "-timer-mode", "bcd", file}
		data, err := captureOmronBodyCommand(t, omronCompileTaskBlock, args)
		if err != nil {
			t.Fatal(err)
		}
		var tasks struct {
			Format                                                                                                                  string
			TaskRecordsGenerated, TaskDirectoryGenerated, TaskConfigurationGenerated, StorageInitialized, Linked, HardwareQualified bool
			Result                                                                                                                  compiler.TaskBlock
		}
		if err := json.Unmarshal(data, &tasks); err != nil {
			t.Fatal(err)
		}
		if tasks.Format != "cj1-task-block-containers" || !tasks.TaskRecordsGenerated || !tasks.TaskDirectoryGenerated || tasks.TaskConfigurationGenerated || tasks.StorageInitialized || tasks.Linked || tasks.HardwareQualified || len(tasks.Result.Words) != 655 || len(tasks.Result.DirectoryWords) != 288 || len(tasks.Result.Tasks) != 2 || tasks.Result.DirectoryWords[257] != 479 {
			t.Fatal("invalid task block report", string(data))
		}
		args = append(args[:len(args)-1], "-number", "BLINK=0", "-next-enable", "0,0,0,0,0,0,0", file)
		data, err = captureOmronBodyCommand(t, omronCompileResourceBody, args)
		if err != nil {
			t.Fatal(err)
		}
		var body struct {
			Format                                                                                                                                              string
			InternalReferencesChecked, TaskDirectoryGenerated, GlobalHeaderGenerated, TaskConfigurationGenerated, StorageInitialized, Linked, HardwareQualified bool
			Result                                                                                                                                              compiler.ResourceBody
		}
		if err := json.Unmarshal(data, &body); err != nil {
			t.Fatal(err)
		}
		if body.Format != "cj1-resource-body-containers" || !body.InternalReferencesChecked || !body.TaskDirectoryGenerated || body.GlobalHeaderGenerated || !body.TaskConfigurationGenerated || body.StorageInitialized || body.Linked || body.HardwareQualified || len(body.Result.Words) != 821 || body.Result.TaskDirectoryOffsetWords != 166 || len(body.Result.Calls) != 1 {
			t.Fatal("invalid resource fragment report", string(data))
		}
	}
}

func TestOmronCompileResourceFragmentsRejectBadScopes(t *testing.T) {
	file := "../../omron/cxp/testdata/cj1m.cxp"
	for _, scope := range []string{"Resource[1]", "Resource[0].Programs.Program[0]"} {
		args := []string{"-model", "CJ1M CPU21", "-resource", scope, "-timer-mode", "bcd", file}
		data, err := captureOmronBodyCommand(t, omronCompileTaskBlock, args)
		if err == nil || len(data) != 0 {
			t.Fatal("invalid block scope emitted output", scope)
		}
	}
	for _, options := range [][]string{
		{"-number", "BLINK=0"},
		{"-number", "Other=0", "-next-enable", "0,0,0,0,0,0,0"},
		{"-number", "BLINK=0", "-next-enable", "1,0,0,0,0,0,0"},
	} {
		args := []string{"-model", "CJ1M CPU21", "-resource", "Resource[0]", "-timer-mode", "bcd"}
		args = append(args, options...)
		args = append(args, file)
		data, err := captureOmronBodyCommand(t, omronCompileResourceBody, args)
		if err == nil || len(data) != 0 {
			t.Fatal("invalid resource controls emitted output", options)
		}
	}
}
