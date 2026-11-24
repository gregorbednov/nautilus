package main

import (
	"encoding/json"
	"errors"
	"flag"
	"os"

	"github.com/joyautomation/nautilus/omron/cxp"
)

func omronFBCall(args []string) error {
	flags := flag.NewFlagSet("omron inspect-fb-call", flag.ContinueOnError)
	encoding := flags.String("encoding", "auto", "CXT text encoding")
	program := flags.String("program", "", "fully qualified program path")
	rung := flags.String("rung", "", "fully qualified graphical call rung path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 || *program == "" || *rung == "" {
		return errors.New("inspect-fb-call requires -program PATH -rung PATH FILE")
	}
	data, err := readOmronFile(flags.Arg(0))
	if err != nil {
		return err
	}
	doc, err := cxp.ReadEncoding(data, *encoding)
	if err != nil {
		return err
	}
	plan, err := doc.PlanFBCall(*program, *rung)
	if err != nil {
		return err
	}
	identified := true
	for _, variable := range plan.Variables {
		identified = identified && variable.TypeName != ""
	}
	// Enum identity is now known. Full native typing/operand and graph checks
	// are still partial, so the existing qualification/compiled flags stay false.
	report := struct {
		SchemaVersion              int             `json:"schemaVersion"`
		Format                     string          `json:"format"`
		SourceSHA256               string          `json:"sourceSHA256"`
		TypeCodesQualified         bool            `json:"typeCodesQualified"`
		DeclarationTypesIdentified bool            `json:"declarationTypesIdentified"`
		Compiled                   bool            `json:"compiled"`
		HardwareQualified          bool            `json:"hardwareQualified"`
		Plan                       *cxp.FBCallPlan `json:"plan"`
	}{SchemaVersion: 1, Format: "cxp-fb-call-plan", SourceSHA256: doc.SHA256, DeclarationTypesIdentified: identified, Plan: plan}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}
