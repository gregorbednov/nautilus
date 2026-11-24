package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/joyautomation/nautilus/omron/codec"
	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/um"
)

// Reject duplicate keys and nulls before typed decoding can silently replace
// explicit settings. Depth and file size are bounded independently.
func checkOmronImageJSON(dec *json.Decoder, depth int) error {
	if depth > 16 {
		return errors.New("image settings nesting limit")
	}
	token, err := dec.Token()
	if err != nil {
		return err
	}
	if token == nil {
		return errors.New("null image setting")
	}
	if delim, ok := token.(json.Delim); ok {
		if delim != '{' && delim != '[' {
			return errors.New("invalid image settings JSON")
		}
		seen := map[string]bool{}
		for dec.More() {
			if delim == '{' {
				key, err := dec.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok || seen[name] {
					return errors.New("duplicate image settings key")
				}
				seen[name] = true
			}
			if err := checkOmronImageJSON(dec, depth+1); err != nil {
				return err
			}
		}
		_, err = dec.Token()
		return err
	}
	return nil
}

func requiredOmronSettings(data []byte, names ...string) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, err
	}
	if len(object) != len(names) {
		return nil, errors.New("unexpected or missing image settings fields")
	}
	for _, name := range names {
		if _, ok := object[name]; !ok {
			return nil, fmt.Errorf("missing explicit image setting %s", name)
		}
	}
	return object, nil
}

func parseOmronImageSettings(data []byte) (compiler.N1ImageSettings, error) {
	var zero compiler.N1ImageSettings
	if len(data) > 1<<20 {
		return zero, errors.New("image settings exceed 1 MiB")
	}
	check := json.NewDecoder(bytes.NewReader(data))
	if err := checkOmronImageJSON(check, 0); err != nil {
		return zero, err
	}
	if _, err := check.Token(); err != io.EOF {
		return zero, errors.New("trailing image settings JSON")
	}
	var input struct {
		OmitSectionMarkers *bool                    `json:"omitSectionMarkers,omitempty"`
		SchemaVersion      int                      `json:"schemaVersion"`
		Options            codec.N1TaskOptions      `json:"options"`
		Memory             codec.N1FBMemorySettings `json:"memory"`
		DefinitionNumbers  map[string]int           `json:"definitionNumbers"`
		InOutBOOLVersion   string                   `json:"inOutBOOLVersion,omitempty"`
		NextEnable         []uint32                 `json:"nextEnable"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		return zero, err
	}
	fields := []string{"schemaVersion", "options", "memory", "definitionNumbers", "nextEnable"}
	if input.OmitSectionMarkers != nil {
		fields = append(fields, "omitSectionMarkers")
	}
	if input.InOutBOOLVersion != "" {
		if input.InOutBOOLVersion != "4.0" {
			return zero, errors.New("inOutBOOLVersion requires 4.0")
		}
		fields = append(fields, "inOutBOOLVersion")
	}
	top, err := requiredOmronSettings(data, fields...)
	if err != nil {
		return zero, err
	}
	if input.SchemaVersion != 1 || len(input.NextEnable) != 7 {
		return zero, errors.New("settings require schemaVersion 1, definition numbers and seven next-enable values")
	}
	if _, err = requiredOmronSettings(top["options"], "independentIRDR", "extendedTIMHold", "copyProhibited", "overwriteProhibited"); err != nil {
		return zero, err
	}
	memory, err := requiredOmronSettings(top["memory"], "bases", "stack")
	if err != nil {
		return zero, err
	}
	if _, err = requiredOmronSettings(memory["stack"], "address", "firstSize", "totalSize"); err != nil {
		return zero, err
	}
	var bases []json.RawMessage
	if err = json.Unmarshal(memory["bases"], &bases); err != nil {
		return zero, err
	}
	for _, base := range bases {
		if _, err = requiredOmronSettings(base, "address", "size"); err != nil {
			return zero, err
		}
	}
	result := compiler.N1ImageSettings{Options: input.Options, Memory: input.Memory, DefinitionNumbers: input.DefinitionNumbers, InOutBOOLVersion: input.InOutBOOLVersion}
	copy(result.NextEnable[:], input.NextEnable)
	if input.OmitSectionMarkers != nil {
		result.OmitSectionMarkers = *input.OmitSectionMarkers
	}
	return result, nil
}

func omronCompileImage(args []string) error {
	flags := flag.NewFlagSet("omron compile-image", flag.ContinueOnError)
	model := flags.String("model", "", "explicit qualified CPU model")
	resource := flags.String("resource", "", "fully qualified resource path")
	timer := flags.String("timer-mode", "", "explicit timer mode: bcd or binary (binary requires no FB)")
	encoding := flags.String("encoding", "auto", "CXT text encoding")
	settingsPath := flags.String("settings", "", "explicit image settings JSON")
	output := flags.String("o", "", "new N1 file; never overwrite")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 || *model == "" || *resource == "" || (*timer != "bcd" && *timer != "binary") || *settingsPath == "" || *output == "" {
		return errors.New("compile-image requires -model MODEL -resource PATH -timer-mode bcd|binary -settings SETTINGS.json -o NEW.n1 FILE")
	}
	return compileOmronProjectImage(*model, *timer, *resource, *encoding, *settingsPath, flags.Arg(0), *output, "", "")
}

// The standalone command requires a new output. A project build can reuse
// a previously published image only after comparing its complete contents.
func compileOmronProjectImage(model, timer, resource, encoding, settingsPath, source, output, buildDir, checkVersion string) error {
	settingsData, err := readOmronFile(settingsPath)
	if err != nil {
		return err
	}
	settings, err := parseOmronImageSettings(settingsData)
	if err != nil {
		return err
	}
	data, err := readOmronFile(source)
	if err != nil {
		return err
	}
	doc, err := cxp.ReadEncoding(data, encoding)
	if err != nil {
		return err
	}
	if err := checkOmronProjectSource(doc, resource, checkVersion); err != nil {
		return err
	}
	image, err := compiler.CompileCXPImage(codec.Profile{Model: model, TimerCounterMode: codec.TimerCounterMode(timer)}, doc, resource, settings)
	if err != nil {
		return err
	}
	return publishOmronImage(image, doc.SHA256, settingsData, settings, model, timer, resource, output, buildDir, checkVersion)
}

func checkOmronProjectSource(doc *cxp.Document, resource, checkVersion string) error {
	if checkVersion == "3.0" {
		summary := doc.Summary()
		for _, f := range summary.FBDeclarationFindings {
			if strings.HasPrefix(f.Path, resource+".") && f.MinimumCJ1UnitVersion == "4.0" {
				return fmt.Errorf("%s.%s (%s %s), line %d requires Unit Ver. 4.0; project checks Unit Ver. 3.0", f.BlockName, f.Variable, f.UsageName, f.TypeName, f.Line)
			}
		}
		for _, rung := range summary.Rungs {
			if strings.HasPrefix(rung.Path, resource+".") {
				for _, ins := range rung.Instructions {
					if ins.MinimumCJ1UnitVersion == "4.0" {
						return fmt.Errorf("%s line %d: %s requires Unit Ver. 4.0; project checks Unit Ver. 3.0", rung.Path, ins.Line, ins.Text)
					}
				}
			}
		}
	}
	return nil
}

func publishOmronImage(image *compiler.N1Image, sourceHash string, settingsData []byte, settings compiler.N1ImageSettings, model, timer, resource, output, buildDir, checkVersion string) error {
	if buildDir != "" {
		if err := os.MkdirAll(buildDir, 0755); err != nil {
			return err
		}
		output = filepath.Join(buildDir, image.Summary().SHA256+".n1")
	}
	path, err := filepath.Abs(output)
	if err != nil {
		return err
	}
	report := struct {
		SchemaVersion                    int                      `json:"schemaVersion"`
		Format                           string                   `json:"format"`
		SourceSHA256                     string                   `json:"sourceSHA256"`
		SettingsSHA256                   string                   `json:"settingsSHA256"`
		Model                            string                   `json:"model"`
		TimerCounterMode                 string                   `json:"timerCounterMode"`
		ProgramCheckUnitVersion          string                   `json:"programCheckUnitVersion,omitempty"`
		Resource                         string                   `json:"resource"`
		Output                           string                   `json:"output"`
		Settings                         compiler.N1ImageSettings `json:"settings"`
		GlobalHeaderGenerated            bool                     `json:"globalHeaderGenerated"`
		TaskConfigurationGenerated       bool                     `json:"taskConfigurationGenerated"`
		InternalReferencesChecked        bool                     `json:"internalReferencesChecked"`
		InstanceMemoryContainmentChecked bool                     `json:"instanceMemoryContainmentChecked"`
		Linked                           bool                     `json:"linked"`
		SourceSettingsVerified           bool                     `json:"sourceSettingsVerified"`
		StorageInitialized               bool                     `json:"storageInitialized"`
		GlobalStorageAudited             bool                     `json:"globalStorageAudited"`
		HardwareQualified                bool                     `json:"hardwareQualified"`
		Image                            um.N1Report              `json:"image"`
	}{SchemaVersion: 1, Format: "cj1-n1-image", SourceSHA256: sourceHash, SettingsSHA256: fmt.Sprintf("%x", sha256.Sum256(settingsData)), Model: model, TimerCounterMode: timer, Resource: resource, Output: path, Settings: settings, GlobalHeaderGenerated: true, TaskConfigurationGenerated: true, InternalReferencesChecked: true, InstanceMemoryContainmentChecked: true, Linked: true, Image: image.Summary()}
	report.ProgramCheckUnitVersion = checkVersion
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err = writeNewOmronFile(path, image.Bytes()); err != nil {
		if buildDir == "" || !errors.Is(err, os.ErrExist) {
			return err
		}
		existing, readErr := readOmronFile(path)
		if readErr != nil {
			return readErr
		}
		if !bytes.Equal(existing, image.Bytes()) {
			return fmt.Errorf("existing build artifact differs: %s", path)
		}
	}
	_, err = os.Stdout.Write(append(encoded, '\n'))
	return err
}
