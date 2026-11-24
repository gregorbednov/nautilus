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
	service := flags.Bool("service-fields", false, "inspect the stored checksum pair and word 4 length hypotheses in this image")
	functionBlocks := flags.Bool("function-blocks", false, "inspect the qualified FB prefix layout; relative code remains opaque")
	decodeBodies := flags.Bool("decode-fb-bodies", false, "decode observed relative FB body forms; requires -model and -timer-mode bcd")
	decodeTasks := flags.Bool("decode-tasks", false, "decode qualified task instructions with exact re-encoding; stop at unknown forms")
	decodeSections := flags.Bool("decode-fb-sections", false, "verify CPU21 floating-point definition and scalar instance instructions")
	model := flags.String("model", "", "explicit CPU model for task/FB instruction decoding")
	timerMode := flags.String("timer-mode", "", "explicit timer mode for task/FB instruction decoding")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 || *format != "n1" {
		return errors.New("expected -format n1 and one compiler image filename")
	}
	if *decodeSections && *model != codec.CJ1MCPU21 {
		return errors.New("-decode-fb-sections requires -model 'CJ1M CPU21'")
	}
	if *decodeBodies || *decodeTasks || *decodeSections {
		if (*model != codec.CJ1MCPU21 && *model != codec.CJ1HCPU65) || *timerMode != "bcd" {
			return errors.New("instruction decoding requires a qualified -model and -timer-mode bcd")
		}
	} else if *model != "" || *timerMode != "" {
		return errors.New("-model and -timer-mode require an instruction decoding option")
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
	type relativeBody struct {
		DefinitionNumber  uint16                        `json:"definitionNumber"`
		BodyNumber        uint16                        `json:"bodyNumber"`
		PrefixOffsetWords uint32                        `json:"prefixOffsetWords"`
		Instructions      []codec.FBRelativeInstruction `json:"instructions"`
	}
	report := struct {
		um.N1Report
		FunctionBlocks *um.N1FunctionBlocks `json:"functionBlocks,omitempty"`
		RelativeBodies []relativeBody       `json:"relativeBodies,omitempty"`
		TaskDecoding   *um.N1TaskDecoding   `json:"taskDecoding,omitempty"`
		FBCodeSections []um.FBCodeSection   `json:"fbCodeSections,omitempty"`
		ServiceFields  *um.N1ServiceFields  `json:"serviceFields,omitempty"`
	}{N1Report: image.Summary()}
	if *service {
		fields := image.ServiceFields()
		report.ServiceFields = &fields
	}
	if *decodeTasks {
		report.TaskDecoding, err = image.DecodeTasks(codec.Profile{Model: *model, TimerCounterMode: codec.TimerModeBCD})
		if err != nil {
			return err
		}
	}
	if *functionBlocks || *decodeBodies || *decodeSections {
		blocks, err := image.FunctionBlocks()
		if err != nil {
			return err
		}
		report.FunctionBlocks = blocks
		if *decodeSections {
			report.FBCodeSections, err = image.DecodeCPU21FBSections()
			if err != nil {
				return err
			}
		}
		if *decodeBodies {
			for _, definition := range blocks.Definitions {
				instructions, err := codec.DecodeFBRelativeBody(codec.Profile{Model: *model, TimerCounterMode: codec.TimerModeBCD}, definition.Code[2:len(definition.Code)-2])
				if err != nil {
					return err
				}
				report.RelativeBodies = append(report.RelativeBodies, relativeBody{definition.Number, definition.BodyNumber, definition.OffsetWords + uint32(len(definition.Header)) + 2, instructions})
			}
		}
	}
	return output.Encode(report)
}
