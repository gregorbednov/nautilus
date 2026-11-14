package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

func run(args []string, output io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: cj1-oracle verify -root REPOSITORY -manifest FILE | audit-capture -log FILE -stderr FILE")
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	root := flags.String("root", ".", "repository/corpus root")
	manifestPath := flags.String("manifest", "tools/omron-research/oracle/testdata/manifest.json", "manifest path")
	logPath := flags.String("log", "", "saved injector stdout")
	stderrPath := flags.String("stderr", "", "saved injector stderr (required)")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	switch args[0] {
	case "verify":
		file, err := os.Open(*manifestPath)
		if err != nil {
			return err
		}
		defer file.Close()
		report, err := verifyManifestReader(*root, file)
		if err != nil {
			return err
		}
		return encoder.Encode(report)
	case "audit-capture":
		if *logPath == "" || *stderrPath == "" {
			return errors.New("audit-capture requires -log and -stderr")
		}
		file, err := os.Open(*logPath)
		if err != nil {
			return err
		}
		defer file.Close()
		stderrFile, err := os.Open(*stderrPath)
		if err != nil {
			return err
		}
		defer stderrFile.Close()
		stderr, err := readBounded(stderrFile, 1<<20)
		if err != nil {
			return err
		}
		report, err := auditCapture(file, string(stderr))
		if err != nil {
			return err
		}
		if err := encoder.Encode(report); err != nil {
			return err
		}
		if !report.Complete {
			return errors.New("capture is incomplete; see problems in the JSON report")
		}
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "cj1-oracle:", err)
		os.Exit(1)
	}
}
