// Package nbhmi converts NB-Designer project sources to a Nautilus HMI.
package nbhmi

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

//go:embed app/*
var app embed.FS

type Result struct {
	Source                    string
	Windows, Components, Tags int
}

const ModelPath = "src/lib/nb-hmi.json"

// ModelBindings reports the first use of each PLC tag in the native HMI.
// The verifier uses the same model as the browser to check edited bindings.
func ModelBindings(project fs.FS, appDir string) (map[string]string, error) {
	if appDir == "" {
		return nil, nil
	}
	model := path.Join(appDir, ModelPath)
	data, err := fs.ReadFile(project, model)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var doc document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", model, err)
	}
	if doc.Version != 1 {
		return nil, fmt.Errorf("%s: unsupported NB HMI model version %d", model, doc.Version)
	}
	bindings := make(map[string]string)
	for _, w := range doc.Windows {
		for _, p := range w.Parts {
			refs := []*register{p.Read, p.Write, p.Low, p.High}
			for _, cond := range []*condition{p.Visible, p.Enabled} {
				if cond != nil {
					refs = append(refs, cond.Register)
				}
			}
			for _, r := range refs {
				if r != nil && r.Area == "CIO" && bindings[r.Key] == "" {
					bindings[r.Key] = fmt.Sprintf("%s: window %d (%s), %s %q", model, w.ID, w.Name, p.Kind, p.Name)
				}
			}
		}
	}
	return bindings, nil
}

type document struct {
	Version int      `json:"version"`
	Name    string   `json:"name"`
	Title   string   `json:"title"`
	Start   int      `json:"start"`
	Common  int      `json:"common"`
	Fast    int      `json:"fast"`
	Width   int      `json:"width"`
	Height  int      `json:"height"`
	Logo    string   `json:"logo,omitempty"`
	Windows []window `json:"windows"`
}

type window struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	Background string `json:"background"`
	Bounds     bounds `json:"bounds"`
	Parts      []part `json:"parts"`
}

type bounds struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type label struct {
	Text  string `json:"text"`
	Size  int    `json:"size"`
	Color string `json:"color"`
	Align int    `json:"align"`
	Bold  bool   `json:"bold"`
}

type register struct {
	Key   string `json:"key"`
	Area  string `json:"area"`
	Index int    `json:"index"`
	Count int    `json:"count"`
}

type condition struct {
	Register *register `json:"register,omitempty"`
	On       bool      `json:"on"`
	Login    bool      `json:"login,omitempty"`
}

type part struct {
	Kind       string            `json:"kind"`
	Name       string            `json:"name"`
	Bounds     bounds            `json:"bounds"`
	Mode       int               `json:"mode,omitempty"`
	Interval   int               `json:"interval,omitempty"`
	Function   string            `json:"function,omitempty"`
	Char       string            `json:"char,omitempty"`
	Target     *int              `json:"target,omitempty"`
	Keyboard   *int              `json:"keyboard,omitempty"`
	Confirm    bool              `json:"confirm,omitempty"`
	AutoLogin  bool              `json:"autoLogin,omitempty"`
	Parameters []int             `json:"parameters,omitempty"`
	Labels     []label           `json:"labels,omitempty"`
	LabelBox   *bounds           `json:"labelBox,omitempty"`
	Assets     map[int]string    `json:"assets,omitempty"`
	Read       *register         `json:"read,omitempty"`
	Write      *register         `json:"write,omitempty"`
	Low        *register         `json:"low,omitempty"`
	High       *register         `json:"high,omitempty"`
	Visible    *condition        `json:"visible,omitempty"`
	Enabled    *condition        `json:"enabled,omitempty"`
	Drawing    *drawing          `json:"drawing,omitempty"`
	Meta       map[string]string `json:"meta,omitempty"`
}

type converter struct {
	root   *os.Root
	files  map[string][]byte
	tags   map[string]string // tag to canonical FINS address
	shapes map[string]map[int]string
	plcID  string
}

// Import creates a new project. It never modifies the source directory.
func Import(source, destination string) (Result, error) {
	root, err := os.OpenRoot(source)
	if err != nil {
		return Result{}, err
	}
	defer root.Close()
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return Result{}, err
	}
	var projects []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(path.Ext(entry.Name()), ".nbp") {
			projects = append(projects, entry.Name())
		}
	}
	if len(projects) != 1 {
		return Result{}, fmt.Errorf("%s: want one root .nbp file, found %d", source, len(projects))
	}
	name := projects[0]
	nbp, err := readXML(root, name)
	if err != nil {
		return Result{}, err
	}
	if nbp.XMLName.Local != "NBProject" || nbp.attr("DBVersion") != "137" {
		return Result{}, fmt.Errorf("%s: unsupported project format (DBVersion %q, want 137)", name, nbp.attr("DBVersion"))
	}
	c := &converter{root: root, files: make(map[string][]byte), tags: make(map[string]string), shapes: make(map[string]map[int]string)}
	doc, err := c.convert(name, nbp)
	if err != nil {
		return Result{}, err
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return Result{}, err
	}
	c.files["hmi/"+ModelPath] = append(data, '\n')
	c.files["nautilus.yaml"] = c.manifest(doc.Name)
	c.files["main.st"] = []byte("PROGRAM Main\nEND_PROGRAM\n")
	for _, file := range []string{"+page.svelte", "Screen.svelte", "model.ts"} {
		data, err := app.ReadFile("app/" + file)
		if err != nil {
			return Result{}, err
		}
		target := "lib"
		if file == "+page.svelte" {
			target = "routes"
		}
		c.files["hmi/src/"+target+"/"+file] = data
	}
	if err := publish(destination, c.files); err != nil {
		return Result{}, err
	}
	result := Result{Source: name, Windows: len(doc.Windows), Tags: len(c.tags)}
	for _, w := range doc.Windows {
		for _, p := range w.Parts {
			if p.Drawing == nil {
				result.Components++
			}
		}
	}
	return result, nil
}

func (c *converter) convert(name string, nbp *element) (document, error) {
	for _, section := range []string{"PLCControlObjects", "AlarmObjects", "EventObjects", "FieldbusDevices"} {
		if e := nbp.child(section); e != nil && len(e.Children) != 0 {
			return document{}, fmt.Errorf("%s: unsupported %s", name, section)
		}
	}
	hmis := nbp.child("HMIs").children("HMI")
	if len(hmis) != 1 {
		return document{}, fmt.Errorf("%s: want one HMI, found %d", name, len(hmis))
	}
	plcs := nbp.child("PLCs").children("PLC")
	if len(plcs) > 1 {
		return document{}, fmt.Errorf("%s: multiple PLCs are not supported", name)
	}
	if len(plcs) == 1 {
		if plcs[0].attr("SystemID") != "448" || plcs[0].child("NetPort").child("IP").child("DefaultParam").value("Type") != "UDP" {
			return document{}, fmt.Errorf("%s: expected Omron Ethernet UDP (SystemID 448)", name)
		}
		c.plcID = plcs[0].attr("ProjectID")
	}
	hmi := hmis[0]
	filename, err := resolveNB(c.root, ".", hmi.attr("URL"))
	if err != nil {
		return document{}, fmt.Errorf("%s: %w", name, err)
	}
	whe, err := readXML(c.root, filename)
	if err != nil {
		return document{}, err
	}
	if whe.XMLName.Local != "HMI" {
		return document{}, fmt.Errorf("%s: expected HMI root", filename)
	}
	for _, section := range []string{"Macros", "RecipeFile"} {
		if e := whe.child(section); e != nil && (len(e.Children) != 0 || strings.TrimSpace(e.Text) != "") {
			return document{}, fmt.Errorf("%s: unsupported %s", filename, section)
		}
	}
	ext := hmi.child("Info").child("ExtInfo")
	doc := document{
		Version: 1, Name: nbp.attr("Name"), Title: whe.attr("Name"),
		Start: number(ext.value("DefaultWindow")), Common: -1, Fast: -1,
	}
	if logo := whe.value("LogoFile"); logo != "" {
		doc.Logo, err = c.logo(logo)
		if err != nil {
			return document{}, fmt.Errorf("%s: logo: %w", filename, err)
		}
	}
	if doc.Name == "" {
		doc.Name = strings.TrimSuffix(name, path.Ext(name))
	}
	for _, key := range []struct {
		name string
		dest *int
	}{{"CommonWindow", &doc.Common}, {"FastSelectWindow", &doc.Fast}} {
		if id := ext.value(key.name); id != "" && id != "65535" {
			*key.dest = number(id)
		}
	}
	ids := make(map[int]bool)
	for _, e := range whe.child("Frames").children("Window") {
		id, err := strconv.Atoi(e.attr("ProjectID"))
		box := windowBounds(e.child("Position"))
		if err != nil || id < 0 || ids[id] || box.W <= 0 || box.H <= 0 {
			return document{}, fmt.Errorf("%s: window %q has invalid or duplicate ID or geometry", filename, e.attr("Name"))
		}
		ids[id] = true
		w := window{ID: id, Name: e.attr("Name"), Background: color(e.attr("BackgroundColor")), Bounds: box, Parts: []part{}}
		if e.attr("Transparent") == "1" {
			w.Background = "transparent"
		}
		if err := c.parts(e.child("Parts"), &w.Parts); err != nil {
			return document{}, fmt.Errorf("%s: window %d (%s): %w", filename, w.ID, w.Name, err)
		}
		doc.Windows = append(doc.Windows, w)
		if id == doc.Start {
			doc.Width, doc.Height = box.W, box.H
		}
	}
	if !ids[doc.Start] {
		return document{}, fmt.Errorf("%s: missing default window %d", filename, doc.Start)
	}
	for _, w := range doc.Windows {
		for _, p := range w.Parts {
			for _, ref := range []*int{p.Target, p.Keyboard} {
				if ref != nil && *ref != 65535 && !ids[*ref] {
					return document{}, fmt.Errorf("%s: window %d: %s %q: missing target window %d", filename, w.ID, p.Kind, p.Name, *ref)
				}
			}
		}
	}
	slices.SortFunc(doc.Windows, func(a, b window) int { return a.ID - b.ID })
	return doc, nil
}

func (c *converter) manifest(name string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "name: %q\nserver:\n  addr: 127.0.0.1:8080\n  hmi: hmi\ntasks:\n  - program: main.st\n    scan: 100ms\n", name)
	if len(c.tags) == 0 {
		return []byte(b.String())
	}
	b.WriteString("tags:\n")
	for _, tag := range sortedKeys(c.tags) {
		fmt.Fprintf(&b, "  - { name: %s, role: state, init: false, alias: %q }\n", tag, c.tags[tag])
	}
	return []byte(b.String())
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func publish(destination string, files map[string][]byte) error {
	if err := os.Mkdir(destination, 0o755); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("destination already exists: %s", destination)
		}
		return err
	}
	// The successful Mkdir above owns this new directory. Write the manifest
	// last so a project loader cannot open a half-written project.
	complete := false
	defer func() {
		if !complete {
			_ = os.RemoveAll(destination)
		}
	}()
	for _, name := range sortedKeys(files) {
		if name == "nautilus.yaml" {
			continue
		}
		target := filepath.Join(destination, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, files[name], 0o644); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(destination, "nautilus.yaml"), files["nautilus.yaml"], 0o644); err != nil {
		return err
	}
	complete = true
	return nil
}
