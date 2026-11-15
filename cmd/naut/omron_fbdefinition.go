package main

import (
	"encoding/json"
	"errors"
	"flag"
	"os"

	"github.com/joyautomation/nautilus/omron/cxp"
)

func omronFBDefinition(args []string) error {
	flags := flag.NewFlagSet("omron inspect-fb-definition", flag.ContinueOnError)
	encoding := flags.String("encoding", "auto", "CXT text encoding")
	definition := flags.String("definition", "", "fully qualified FB definition path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 || *definition == "" {
		return errors.New("inspect-fb-definition requires -definition PATH FILE")
	}
	data, err := readOmronFile(flags.Arg(0))
	if err != nil {
		return err
	}
	doc, err := cxp.ReadEncoding(data, *encoding)
	if err != nil {
		return err
	}
	source, err := doc.SourceForFBDefinition(*definition)
	if err != nil {
		return err
	}
	report := struct {
		SchemaVersion      int                     `json:"schemaVersion"`
		Format             string                  `json:"format"`
		SourceSHA256       string                  `json:"sourceSHA256"`
		Compiled           bool                    `json:"compiled"`
		AddressesAllocated bool                    `json:"addressesAllocated"`
		HardwareQualified  bool                    `json:"hardwareQualified"`
		Source             *cxp.FBDefinitionSource `json:"source"`
	}{SchemaVersion: 1, Format: "cxp-fb-definition-source", SourceSHA256: doc.SHA256, Source: source}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}
