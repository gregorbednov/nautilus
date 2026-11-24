package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joyautomation/nautilus/omron/compiler"
	"github.com/joyautomation/nautilus/omron/cxp"
)

//go:embed settings.json
var readbackSettings []byte

// buildReadback is an experiment on the identified CX upload. Its input
// boundary deliberately excludes program.json, standalone ST files and UM.
// The archive's global image settings and definition numbers remain explicit.
func buildReadback(file string) ([]byte, error) {
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	doc, err := cxp.ReadEncoding(raw, "windows-1251")
	if err != nil {
		return nil, err
	}
	if doc.SHA256 != "6328c941cb5061d5b14ab18111ea9e7b875b75b334b002fcc655df8dc05eacd8" {
		return nil, fmt.Errorf("expected the 20 November CPU21 upload, got %s", doc.SHA256)
	}
	const resource = "Resource[0]"
	const task = resource + ".Programs.Program[0]"
	var program compiler.STILProgram
	sources := map[string]string{}
	byName := map[string]int{}
	fields := map[string]cxp.Field{}
	doc.Walk(func(path string, f cxp.Field) { fields[path] = f })
	count := func(path string) (int, error) { return strconv.Atoi(fields[path].Text()) }
	blocks := resource + ".FunctionBlocks"
	n, err := count(blocks + ".FunctionBlockCount")
	if err != nil {
		return nil, err
	}
	for i := 0; i < n; i++ {
		path := fmt.Sprintf("%s.FunctionBlockWrapper[%d].FunctionBlock", blocks, i)
		f, err := doc.STFloatSourceForDefinition(path)
		if err != nil {
			return nil, err
		}
		if f == nil {
			return nil, fmt.Errorf("%s is not an ST definition", path)
		}
		name := fields[path+".Name"].Text()
		program.Definitions = append(program.Definitions, compiler.STILDefinition{Name: name, BodyNumber: f.TaskID})
		sources[name] = f.Text
	}
	placements := resource + ".GlobalVariables.POUInstances"
	n, err = count(placements + ".NumInstances")
	if err != nil {
		return nil, err
	}
	for i := 0; i < n; i++ {
		instance, err := doc.SourceForFBInstance(fmt.Sprintf("%s.Instance[%d]", placements, i))
		if err != nil {
			return nil, err
		}
		h, err := strconv.Atoi(strings.TrimPrefix(instance.Bases[3].Address, "H"))
		if err != nil || instance.Bases[3].Size != 8 {
			return nil, fmt.Errorf("unexpected BR3 for %s", instance.Name)
		}
		program.Instances = append(program.Instances, compiler.STILInstance{Number: instance.Number, Definition: instance.Definition, Holding: h})
		byName[instance.Name] = instance.Number
	}
	program.Task.Name = fields[task+".Name"].Text()
	program.Task.Number, err = count(task + ".TaskID")
	if err != nil {
		return nil, err
	}
	start, err := strconv.ParseBool(fields[task+".OpStart"].Text())
	if err != nil {
		return nil, err
	}
	program.Task.OperationStart = &start
	source, err := doc.SourceForProgram(task)
	if err != nil {
		return nil, err
	}
	for i, section := range source.Sections {
		if i > 0 {
			program.Task.Instructions = append(program.Task.Instructions, "CMP #DEAD #DEAD")
		}
		for _, rung := range section.Rungs {
			if len(rung.Graphic) > 0 {
				cells := map[string]string{}
				for _, f := range rung.Graphic {
					c := strings.Split(f.Text(), "\x11")
					if len(c) == 3 {
						continue
					} // horizontal wires in the pinned layout
					if len(c) != 8 {
						return nil, fmt.Errorf("%s: unsupported graphic", rung.Path)
					}
					cells[c[5]] = c[6]
				}
				id, ok := byName[cells["FBCALL"]]
				if !ok || cells["LD"] != "P_On" {
					return nil, fmt.Errorf("%s: unresolved call or enable", rung.Path)
				}
				program.Task.Instructions = append(program.Task.Instructions, "LD CF113", fmt.Sprintf("CALL %d %s %s", id, cells["FB_INPUT"], cells["FB_OUTPUT"]), "OUT "+cells["OUT"])
			}
			for _, ins := range rung.Instructions {
				line := ins.Text
				if line == "LD P_On" {
					line = "LD CF113"
				}
				program.Task.Instructions = append(program.Task.Instructions, line)
			}
		}
	}
	var settings compiler.N1ImageSettings
	if err := json.Unmarshal(readbackSettings, &settings); err != nil {
		return nil, err
	}
	image, err := compiler.CompileSTILImage(profile, program, sources, settings)
	if err != nil {
		return nil, err
	}
	return image.Bytes(), nil
}
