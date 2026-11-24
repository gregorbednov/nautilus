package main

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"

	omrontransfer "github.com/joyautomation/nautilus/omron/transfer"
	"github.com/joyautomation/nautilus/omron/um"
)

func omronPlanProgramWrite(args []string) error {
	flags := flag.NewFlagSet("omron plan-program-write", flag.ContinueOnError)
	blockBytes := flags.Int("block-bytes", omrontransfer.MaxProgramWriteBytes, "candidate0307 data bytes per block; aligned4, max996")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("expected one N1 image filename; offline only, no connection flags")
	}
	f, err := os.Open(flags.Arg(0))
	if err != nil {
		return err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, um.MaxSize+1))
	if err != nil {
		return err
	}
	plan, err := omrontransfer.PlanProgramWrite(b, *blockBytes)
	if err != nil {
		return err
	}
	output := json.NewEncoder(os.Stdout)
	output.SetIndent("", "  ")
	return output.Encode(plan)
}
