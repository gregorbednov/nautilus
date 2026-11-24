package main

import (
	"strconv"
	"strings"

	"github.com/joyautomation/nautilus/lang/ld"
	"github.com/joyautomation/nautilus/omron/cxp"
	"github.com/joyautomation/nautilus/omron/fins"
	"github.com/joyautomation/nautilus/omron/monitor"
)

type omronWatchRow struct {
	Name    string `json:"name"`
	Scope   string `json:"scope"`
	Address string `json:"address"`
	Type    string `json:"type"`
	Key     string `json:"key,omitempty"`
}

type omronWatchModel struct {
	Rows     []omronWatchRow   `json:"rows"`
	Bindings []monitor.Binding `json:"bindings"`
	Error    string            `json:"error,omitempty"`
}

type omronMemoryKey struct {
	address  fins.Memory
	dataType string
}

type omronWatchBindings struct {
	model *omronWatchModel
	keys  map[omronMemoryKey]string
	names map[[2]string]bool
}

func newOmronWatchBindings(model *omronWatchModel) *omronWatchBindings {
	model.Rows = []omronWatchRow{}
	model.Bindings = []monitor.Binding{}
	return &omronWatchBindings{model, make(map[omronMemoryKey]string), make(map[[2]string]bool)}
}

func (w *omronWatchBindings) add(row omronWatchRow, monitorType string, declaration bool) {
	name := [2]string{row.Scope, row.Name}
	if w.names[name] {
		return
	}
	binding := monitor.Binding{Address: row.Address, Type: monitorType}
	address, err := monitor.BindingMemory(binding)
	if err != nil && !declaration {
		return
	}
	if err == nil {
		key := omronMemoryKey{address, monitorType}
		id, found := w.keys[key]
		if !found {
			id = "v" + strconv.Itoa(len(w.model.Bindings))
			w.keys[key] = id
			binding.Name = id
			w.model.Bindings = append(w.model.Bindings, binding)
		}
		row.Key = id
	}
	w.model.Rows = append(w.model.Rows, row)
	w.names[name] = true
}

func (w *omronWatchBindings) readSymbols(doc *cxp.Document, path, scope string) {
	tables, err := doc.SymbolsForProgram(path)
	if err != nil {
		w.model.Error = err.Error()
		return
	}
	for _, section := range []struct {
		scope   string
		symbols []cxp.Symbol
	}{{"", tables.Globals}, {scope, tables.Locals}} {
		for _, s := range section.symbols {
			if s.Name == "" {
				continue
			}
			typ := strings.ToUpper(s.DataType)
			if typ == "CHANNEL" || typ == "UINT_BCD" {
				typ = "WORD"
			}
			if s.Flags != "" || s.AuxType != "" {
				typ = ""
			}
			address := strings.TrimSpace(s.Address)
			if typ == "BOOL" {
				address = omronWatchBitAddress(address)
			}
			w.add(omronWatchRow{Name: s.Name, Scope: section.scope, Address: address, Type: s.DataType}, typ, true)
		}
	}
}

func omronWatchBitAddress(name string) string {
	address, err := fins.ParseMemory(name)
	if err == nil && address.Area == 0x89 {
		return name + ".Q"
	}
	return name
}

func (w *omronWatchBindings) readOperands(scope string, elements []ld.Element) {
	for _, element := range elements {
		for _, leg := range element.Legs {
			w.readOperands(scope, leg)
		}
		if element.Ref == "" {
			continue
		}
		if element.Kind == "fn" {
			w.add(omronWatchRow{Name: element.Ref + ".PV", Scope: scope, Address: element.Ref, Type: "WORD"}, "WORD", false)
			continue
		}
		if w.names[[2]string{scope, element.Ref}] || w.names[[2]string{"", element.Ref}] {
			continue
		}
		w.add(omronWatchRow{Name: element.Ref, Scope: scope, Address: omronWatchBitAddress(element.Ref), Type: "BOOL"}, "BOOL", false)
	}
}
