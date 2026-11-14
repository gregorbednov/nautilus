package main

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"

	"github.com/joyautomation/nautilus/omron/um"
)

func omronImage(args []string) error {
	flags := flag.NewFlagSet("omron inspect-image", flag.ContinueOnError)
	format := flags.String("format", "n1", "explicit compiler image format (n1)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 || *format != "n1" {
		return errors.New("expected -format n1 and one compiler image filename")
	}
	file, err := os.Open(flags.Arg(0))
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, um.MaxSize+1))
	if err != nil {
		return err
	}
	image, err := um.ParseN1(data)
	if err != nil {
		return err
	}
	output := json.NewEncoder(os.Stdout)
	output.SetIndent("", "  ")
	return output.Encode(image.Summary())
}
