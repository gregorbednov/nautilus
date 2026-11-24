package main

import (
	"errors"
	"flag"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/cxp"
)

func omronCompileFBCallBody(args []string) error {
	flags := flag.NewFlagSet("omron compile-fb-call-body", flag.ContinueOnError)
	model := flags.String("model", "", "explicit qualified CPU model")
	unit := flags.String("unit-version", "", "explicit target version for a version-qualified call")
	timer := flags.String("timer-mode", "", "timer mode for a version-qualified call (bcd)")
	program := flags.String("program", "", "fully qualified program path")
	rung := flags.String("rung", "", "fully qualified graphical call rung")
	encoding := flags.String("encoding", "auto", "CXT text encoding")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 || *model == "" || *program == "" || *rung == "" {
		return errors.New("compile-fb-call-body requires -model MODEL -program PATH -rung PATH FILE")
	}
	data, err := readOmronFile(flags.Arg(0))
	if err != nil {
		return err
	}
	doc, err := cxp.ReadEncoding(data, *encoding)
	if err != nil {
		return err
	}
	profile := codec.Profile{Model: *model, UnitVersion: *unit, TimerCounterMode: codec.TimerCounterMode(*timer)}
	body, err := compiler.CompileCXPFBCallBody(profile, doc, *program, *rung)
	if err != nil {
		return err
	}
	return writeOmronBodyReport("cj1-fb-call-body-containers", doc, *program, profile, body)
}
