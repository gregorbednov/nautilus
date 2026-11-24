package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
)

func buildSTILProject(dir string, p omronProject) error {
	settingsData, err := readOmronFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		return err
	}
	settings, err := parseOmronImageSettings(settingsData)
	if err != nil {
		return err
	}
	raw, err := readOmronFile(filepath.Join(dir, "program.json"))
	if err != nil {
		return err
	}
	if len(raw) > 1<<20 {
		return fmt.Errorf("program.json exceeds 1 MiB")
	}
	if err := checkOmronImageJSON(json.NewDecoder(bytes.NewReader(raw)), 0); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var program compiler.STILProgram
	if err := dec.Decode(&program); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return fmt.Errorf("trailing program.json data")
	}
	if len(program.Definitions) == 0 || len(program.Definitions) > 1024 || len(program.Definitions) != len(settings.DefinitionNumbers) {
		return fmt.Errorf("program definitions must correspond to settings.json")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	sources := map[string]string{}
	// Hash the manifest and the source contents in manifest order, with fixed-size
	// digest boundaries; descriptor names and the ordered list are in program.json.
	hash := sha256.New()
	digest := sha256.Sum256(raw)
	hash.Write(digest[:])
	for _, d := range program.Definitions {
		file := d.Name + ".st"
		_, configured := settings.DefinitionNumbers[d.Name]
		if _, exists := sources[d.Name]; !configured || exists || len(file) > 67 || !filepath.IsLocal(file) || filepath.Base(file) != file {
			return fmt.Errorf("invalid or duplicate FB file %q", file)
		}
		f, err := root.Open(file)
		if err != nil {
			return err
		}
		data, err := io.ReadAll(io.LimitReader(f, 65537))
		f.Close()
		if err != nil {
			return err
		}
		if len(data) > 65536 {
			return fmt.Errorf("%s exceeds 64 KiB", file)
		}
		sources[d.Name] = string(data)
		digest = sha256.Sum256(data)
		hash.Write(digest[:])
	}
	image, err := compiler.CompileSTILImage(codec.Profile{Model: p.Model, TimerCounterMode: codec.TimerModeBCD}, program, sources, settings)
	if err != nil {
		return err
	}
	return publishOmronImage(image, fmt.Sprintf("%x", hash.Sum(nil)), settingsData, settings, p.Model, p.TimerMode, p.Resource, "", filepath.Join(dir, "build"), p.CheckVersion)
}
