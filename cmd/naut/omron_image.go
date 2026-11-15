package main

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/um"
)

func omronImage(args []string) error {
	flags := flag.NewFlagSet("omron inspect-image", flag.ContinueOnError)
	format := flags.String("format", "n1", "explicit compiler image format (n1)")
	functionBlocks := flags.Bool("function-blocks", false, "inspect the qualified FB prefix layout; relative code remains opaque")
	decodeBodies := flags.Bool("decode-fb-bodies", false, "decode observed relative FB body forms; requires -model and -timer-mode bcd")
	model := flags.String("model", "", "explicit CPU model for relative body decoding")
	timerMode := flags.String("timer-mode", "", "explicit timer mode for relative body decoding")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 || *format != "n1" {
		return errors.New("expected -format n1 and one compiler image filename")
	}
	if *decodeBodies {
		if (*model != codec.CJ1MCPU21 && *model != codec.CJ1HCPU65) || *timerMode != "bcd" {
			return errors.New("decode-fb-bodies requires a qualified -model and -timer-mode bcd")
		}
	} else if *model != "" || *timerMode != "" {
		return errors.New("-model and -timer-mode require -decode-fb-bodies")
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
	if *functionBlocks || *decodeBodies {
		blocks, err := image.FunctionBlocks()
		if err != nil {
			return err
		}
		type relativeBody struct {
			DefinitionNumber  uint16                        `json:"definitionNumber"`
			BodyNumber        uint16                        `json:"bodyNumber"`
			PrefixOffsetWords uint32                        `json:"prefixOffsetWords"`
			Instructions      []codec.FBRelativeInstruction `json:"instructions"`
		}
		var relativeBodies []relativeBody
		if *decodeBodies {
			for _, definition := range blocks.Definitions {
				instructions, err := codec.DecodeFBRelativeBody(codec.Profile{Model: *model, TimerCounterMode: codec.TimerModeBCD}, definition.Code[2:len(definition.Code)-2])
				if err != nil {
					return err
				}
				relativeBodies = append(relativeBodies, relativeBody{definition.Number, definition.BodyNumber, definition.OffsetWords + uint32(len(definition.Header)) + 2, instructions})
			}
		}
		return output.Encode(struct {
			um.N1Report
			FunctionBlocks *um.N1FunctionBlocks `json:"functionBlocks"`
			RelativeBodies []relativeBody       `json:"relativeBodies,omitempty"`
		}{image.Summary(), blocks, relativeBodies})
	}
	return output.Encode(image.Summary())
}
