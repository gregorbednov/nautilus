package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/joyautomation/nautilus/internal/nbhmi"
	"github.com/joyautomation/nautilus/internal/nbhmi/nbpanel"
)

func runHMIExportNB(args []string) int {
	flags := flag.NewFlagSet("naut hmi export-nb", flag.ContinueOnError)
	out := flags.String("out", "", "new output directory for the NB screen model")
	bindings := flags.String("bindings", "", "JSON mapping from Nautilus tags to NB bit addresses")
	flags.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: naut hmi export-nb --out dir [--bindings tags.json] screen.mimic.json")
	}
	if err := flags.Parse(args); errors.Is(err, flag.ErrHelp) {
		return 0
	} else if err != nil {
		return 2
	}
	if *out == "" || flags.NArg() != 1 || !strings.HasSuffix(strings.ToLower(flags.Arg(0)), ".mimic.json") {
		flags.Usage()
		return 2
	}
	if err := nbhmi.ExportMimic(flags.Arg(0), *bindings, *out); err != nil {
		fmt.Fprintln(os.Stderr, "naut hmi export-nb:", err)
		return 1
	}
	fmt.Printf("exported NB screen model to %s\n", filepath.Join(*out, "hmi", filepath.FromSlash(nbhmi.ModelPath)))
	return 0
}

func runHMINBNetwork(args []string, download bool) int {
	name, suffix, files := "naut hmi nb-probe", "", 0
	if download {
		name, suffix, files = "naut hmi nb-download", " package.pkg", 1
	}
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	endpoint := flags.String("endpoint", "", "panel IPv4 address with optional UDP port")
	flags.Usage = func() { fmt.Fprintf(os.Stderr, "usage: %s --endpoint ipv4[:port]%s\n", name, suffix) }
	if err := flags.Parse(args); errors.Is(err, flag.ErrHelp) {
		return 0
	} else if err != nil {
		return 2
	}
	if *endpoint == "" || flags.NArg() != files {
		flags.Usage()
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	var err error
	if download {
		var pkg *nbpanel.Package
		pkg, err = nbpanel.ReadPackage(flags.Arg(0))
		if err == nil {
			err = pkg.Download(ctx, *endpoint, func(name string, size int) {
				fmt.Printf("transferred %s (%d bytes)\n", name, size)
			})
		}
		if err == nil {
			fmt.Printf("panel confirmed download of %s\n", flags.Arg(0))
		}
	} else {
		var version nbpanel.PanelVersion
		version, err = nbpanel.ProbeVersion(ctx, *endpoint)
		if err == nil {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			err = enc.Encode(version)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, name+":", err)
		return 1
	}
	return 0
}

func runHMIInspectNB(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: naut hmi nb-inspect package.pkg")
		return 2
	}
	pkg, err := nbpanel.ReadPackage(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "naut hmi nb-inspect:", err)
		return 1
	}
	info := struct {
		Model   string           `json:"model"`
		ModelID uint16           `json:"model_id"`
		Screens []nbpanel.Screen `json:"screens"`
	}{Model: nbpanel.ModelName, ModelID: pkg.ModelID, Screens: pkg.Screens}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(info); err != nil {
		fmt.Fprintln(os.Stderr, "naut hmi nb-inspect:", err)
		return 1
	}
	return 0
}

func runHMIBuildNB(args []string) int {
	flags := flag.NewFlagSet("naut hmi nb-build", flag.ContinueOnError)
	offline := flags.Bool("offline", false, "enable the offline simulator build")
	target := flags.String("target", nbpanel.ModelName, "panel model")
	reference := flags.String("reference", "", "local NB10W reference package")
	out := flags.String("out", "", "write the resulting test package to a new file")
	flags.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: naut hmi nb-build --offline [--target NB10W-TW01B] --reference seed.pkg --out test.pkg project")
	}
	if err := flags.Parse(args); errors.Is(err, flag.ErrHelp) {
		return 0
	} else if err != nil {
		return 2
	}
	if !*offline || *reference == "" || flags.NArg() != 1 || !strings.EqualFold(filepath.Ext(*out), ".pkg") {
		flags.Usage()
		return 2
	}
	if !strings.EqualFold(*target, nbpanel.ModelName) {
		fmt.Fprintf(os.Stderr, "naut hmi nb-build: unsupported target %q, want %s\n", *target, nbpanel.ModelName)
		return 2
	}
	pkg, err := nbhmi.BuildOfflinePanel(flags.Arg(0), *reference)
	if err != nil {
		fmt.Fprintln(os.Stderr, "naut hmi nb-build:", err)
		return 1
	}
	if err := writeNewPanelPackage(*out, pkg); err != nil {
		fmt.Fprintln(os.Stderr, "naut hmi nb-build:", err)
		return 1
	}
	fmt.Printf("built offline test package %s for NBSimulator\n", *out)
	return 0
}

func writeNewPanelPackage(name string, data []byte) error {
	file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(data)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return errors.Join(err, os.Remove(name))
	}
	return nil
}
