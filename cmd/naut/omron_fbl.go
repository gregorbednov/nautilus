package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/joyautomation/nautilus/omron/cxp"
)

func omronBuildFBL(args []string) error {
	flags := flag.NewFlagSet("omron build-fbl", flag.ContinueOnError)
	output := flags.String("o", "", "new FBL output file")
	encoding := flags.String("encoding", "auto", "source CXT encoding")
	modified := flags.String("modified", "", "optional CX Modified timestamp, D M YYYY H M S")
	programsOnly := flags.Bool("programs-only", false, "use the CPU65H Programs-only archive layout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 || *output == "" {
		return errors.New("build-fbl requires -o NEW.FBL and one CXP/CXT source")
	}
	source, err := readOmronFile(flags.Arg(0))
	if err != nil {
		return err
	}
	doc, err := cxp.ReadEncoding(source, *encoding)
	if err != nil {
		return err
	}
	build := doc.BuildFBL
	if *programsOnly {
		build = doc.BuildProgramsOnlyFBL
	}
	data, err := build(*modified)
	if err != nil {
		return err
	}
	if err := writeNewOmronFile(*output, data); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Output string `json:"output"`
		Bytes  int    `json:"bytes"`
		SHA256 string `json:"sha256"`
	}{*output, len(data), fmt.Sprintf("%x", sha256.Sum256(data))})
}
