package main

import (
	"os"
	"reflect"
	"testing"

	nio "github.com/joyautomation/nautilus/io"
	"github.com/joyautomation/nautilus/omron/cxp"
)

func TestOmronWatchQuality(t *testing.T) {
	values := nio.Values{"v0": false, "v1": 0}
	overrides := map[string]nio.Quality{"v1": nio.Stale, "v2": nio.NotConnected}
	want := map[string]string{"v0": "good", "v1": "stale", "v2": "notConnected"}
	if got := omronWatchQuality(values, overrides); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestOmronWatchNew4Bindings(t *testing.T) {
	data, err := os.ReadFile("../../omron/commission/testdata/cj1m-new4/source.cxp")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := cxp.Read(data)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := omronLadderModel(doc, "Resource[0]")
	if err != nil {
		t.Fatal(err)
	}
	watch := graph.Omron
	if watch.Error != "" {
		t.Fatal(watch.Error)
	}
	rows := map[[2]string]omronWatchRow{}
	for _, row := range watch.Rows {
		rows[[2]string{row.Scope, row.Name}] = row
	}
	for _, example := range []struct{ scope, name, address, dataType string }{
		{"", "ON", "W1.00", "BOOL"},
		{"", "L1_DOWN", "W0.00", "BOOL"},
		{"TK0", "L1_DOWN", "1.06", "BOOL"},
		{"TK0", "T0000", "T0000.Q", "BOOL"},
		{"TK0", "T0000.PV", "T0000", "WORD"},
		{"TK1", "W5.10", "W5.10", "BOOL"},
	} {
		row := rows[[2]string{example.scope, example.name}]
		if row.Key == "" || row.Address != example.address || row.Type != example.dataType {
			t.Errorf("binding for %s %s: %+v", example.scope, example.name, row)
		}
	}
	for _, key := range [][2]string{{"", "BLINK_L1"}, {"TK0", "TR0"}, {"TK0", "TR1"}} {
		if rows[key].Key != "" {
			t.Errorf("runtime address assigned to %v", key)
		}
	}
	if rows[[2]string{"", "L1_DOWN"}].Key == rows[[2]string{"TK0", "L1_DOWN"}].Key {
		t.Fatal("local and global addresses were collapsed")
	}
}
