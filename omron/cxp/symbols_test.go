package cxp

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestOriginalSymbolTablesAndScopes(t *testing.T) {
	for _, name := range []string{"cj1m.cxp", "cj1h.cxp"} {
		data, err := os.ReadFile("testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := Read(data)
		if err != nil {
			t.Fatal(err)
		}
		before := append([]byte{}, doc.Source...)
		first, err := doc.SymbolsForProgram("Resource[0].Programs.Program[0]")
		if err != nil {
			t.Fatal(err)
		}
		second, err := doc.SymbolsForProgram("Resource[0].Programs.Program[1]")
		if err != nil {
			t.Fatal(err)
		}
		if len(first.Globals) != 16 || len(first.Locals) != 10 || len(second.Locals) != 31 {
			t.Fatalf("%s: unexpected symbol counts %d/%d/%d", name, len(first.Globals), len(first.Locals), len(second.Locals))
		}
		found := false
		auto := 0
		anonymous := 0
		for _, symbol := range first.Locals {
			if symbol.Name == "L1_Key_start" {
				if symbol.Address != "0.08" || symbol.DataType != "BOOL" || symbol.Scope != first.Program || symbol.Line < 1 {
					t.Fatal("local symbol lost metadata", symbol)
				}
				found = true
			}
		}
		for _, symbol := range first.Globals {
			if symbol.DataType == "FUNCTION BLOCK" {
				if symbol.Address != " [Auto]" || symbol.Flags != "A" || symbol.AuxType != "BLINK" {
					t.Fatal(symbol)
				}
				auto++
			}
			if symbol.Name == "" {
				anonymous++
			}
		}
		if !found || auto != 2 || anonymous != 2 {
			t.Fatal("declarations lost", found, auto, anonymous)
		}
		for _, symbol := range second.Locals {
			if symbol.Name == "L1_Key_start" {
				t.Fatal("merged a different program's locals")
			}
		}
		if string(before) != string(doc.Source) {
			t.Fatal("symbol extraction modified source")
		}
	}
}

func symbolDocument(raw string, count string) *Document {
	return &Document{Fields: []Field{{Name: "Resource[0]", Kind: "object", Fields: []Field{
		{Name: "GlobalVariables", Kind: "object", Fields: []Field{{Name: "VariableList", Kind: "list", Raw: raw, Line: 10}, {Name: "VariableCount", Kind: "scalar", Value: count, Line: 20}}},
		{Name: "Programs", Kind: "object", Fields: []Field{{Name: "Program[0]", Kind: "object"}}},
	}}}}
}

func TestSymbolCommentsCannotCreateBindings(t *testing.T) {
	raw := "\"Пуск, \"\"A\"\"\",\"W1.00\",BOOL,,,комментарий с , и \"кавычкой;\nOther,D0,WORD,,,\n$?St$Bk?_#[42]\nForged,W99.00,BOOL,,,;\n$?St$Bk?_#[99];\nпосторонний текст$?St$Bk?_#[42];\nLast,W0.01,BOOL,,,;"
	tables, err := symbolDocument(raw, "3").SymbolsForProgram("Resource[0].Programs.Program[0]")
	if err != nil {
		t.Fatal(err)
	}
	if len(tables.Globals) != 3 || tables.Globals[0].Name != "Пуск, \"A\"" || tables.Globals[0].Address != "W1.00" || tables.Globals[1].Name != "Other" || tables.Globals[2].Name != "Last" || tables.Globals[2].Line != 18 {
		t.Fatalf("raw comments/locations misread: %+v", tables.Globals)
	}
	for _, symbol := range tables.Globals {
		if symbol.Name == "Forged" {
			t.Fatal("raw comment became symbol declaration")
		}
	}
}

func TestMalformedSymbolTablesFailClosed(t *testing.T) {
	for _, test := range []struct{ raw, count string }{
		{"Alias,W0.00,BOOL,,,;\nAlias,W0.01,BOOL,,,;", "2"},
		{"Alias,W0.00,BOOL,,,;", "2"},
		{"Alias,W0.00,BOOL,,,;", "-1"},
		{"Alias,W0.00,BOOL,,,;", "n"},
		{"Alias,W0.00,BOOL,,;", "1"},
		{"\"Alias,W0.00,BOOL,,,;", "1"},
		{"\"Alias\"junk,W0.00,BOOL,,,;", "1"},
		{"Alias,W0.00,BOOL,,,\n$?St$Bk?_#[42]\nFake,W0.01,BOOL,,,;", "1"},
		{"Alias,W0.00,BOOL,,,unfinished", "1"},
		{"Alias,W0.00,BOOL,,,\nnot a raw marker", "1"},
	} {
		if result, err := symbolDocument(test.raw, test.count).SymbolsForProgram("Resource[0].Programs.Program[0]"); result != nil || !errors.Is(err, ErrSymbolTable) {
			t.Fatalf("accepted malformed table %q/%s: %v", test.raw, test.count, err)
		}
	}
	doc := symbolDocument("", "0")
	for _, scope := range []string{"", "Resource[1].Programs.Program[0]", "Resource[0].Programs.Program[1]", "Resource[0].FunctionBlocks.Block[0]", "Resource[0].Programs.Program[0].Sections.Sec[0]"} {
		if result, err := doc.SymbolsForProgram(scope); result != nil || !errors.Is(err, ErrSymbolTable) {
			t.Fatal("accepted missing/unsupported scope", scope, err)
		}
	}
	doc.Fields = append(doc.Fields, doc.Fields[0])
	if _, err := doc.SymbolsForProgram("Resource[0].Programs.Program[0]"); !errors.Is(err, ErrSymbolTable) {
		t.Fatal("accepted duplicate resource")
	}
	var absent *Document
	if _, err := absent.SymbolsForProgram("Resource[0].Programs.Program[0]"); !errors.Is(err, ErrSymbolTable) {
		t.Fatal("accepted nil source")
	}
	large := symbolDocument(strings.Repeat(" ", MaxSymbolBytes+1), "0")
	if _, err := large.SymbolsForProgram("Resource[0].Programs.Program[0]"); !errors.Is(err, ErrSymbolTable) {
		t.Fatal("accepted oversized symbol table")
	}
}

func FuzzSymbolTableRows(f *testing.F) {
	for _, raw := range []string{"Alias,W0.00,BOOL,,,;", ",D0,WORD,,,;", "Alias,D0,WORD,,,\n$?St$Bk?_#[42]\ncomment$?St$Bk?_#[42];", "\"A,B\",\"W0.00\",BOOL,,,;", ""} {
		f.Add(raw)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > 4096 {
			return
		}
		rows, err := parseSymbolRows(Field{Raw: raw, Line: 10}, "Resource[0]")
		if err != nil {
			if rows != nil {
				t.Fatal("partial symbol inventory on failure")
			}
			return
		}
		seen := map[string]bool{}
		for _, symbol := range rows {
			if symbol.Scope != "Resource[0]" || symbol.Line < 12 || symbol.Line > 12+strings.Count(raw, "\n") {
				t.Fatal("incorrect symbol scope/line")
			}
			if symbol.Name != "" && seen[symbol.Name] {
				t.Fatal("duplicate symbol escaped validation")
			}
			seen[symbol.Name] = true
		}
	})
}
