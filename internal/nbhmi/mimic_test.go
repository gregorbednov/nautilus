package nbhmi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportMimicScreen(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "test.mimic.json")
	bindings := filepath.Join(dir, "tags.json")
	out := filepath.Join(dir, "panel")
	raw := `{
	  "name": "Test",
	  "canvas": {"width": 1600, "height": 800},
	  "equipment": [
	    {"id":"pump","component":"Pump","x":240,"y":120,"width":160,"label":"P-1","bind":{"running":"Run"}},
	    {"id":"enable","component":"WriteToggle","x":480,"y":120,"width":200,
	     "props":{"label":"Enable","tag":"Enabled","value":true},"bind":{"value":"Enabled"}}
	  ],
	  "pipes": [{"id":"suction","routing":"orthogonal","points":[[40,120],[240,180]]}],
	  "labels": [{"text":"Training HMI","x":40,"y":30}]
	}`
	for filename, content := range map[string]string{source: raw, bindings: `{"Run":"CIO1.03","Enabled":"LB20"}`} {
		if err := os.WriteFile(filename, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := ExportMimic(source, bindings, out); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(out, "hmi", ModelPath))
	if err != nil {
		t.Fatal(err)
	}
	var doc document
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Start != 0 || doc.Width != 800 || doc.Height != 480 || len(doc.Windows) != 1 || doc.Windows[0].ID != 0 {
		t.Fatalf("unexpected target screen: %+v", doc)
	}
	parts := doc.Windows[0].Parts
	if len(parts) != 6 {
		t.Fatalf("got %d parts, want 6", len(parts))
	}
	for _, p := range parts {
		if _, err := panelObject(os.DirFS(out), p); err != nil {
			t.Errorf("panel compiler rejected %q: %v", p.Name, err)
		}
	}
	first := parts[0].Drawing
	if first.Bounds.X != 20 || first.Bounds.Y != 100 || first.X2 != 120 || first.Y2 != 100 {
		t.Fatalf("unexpected pipe scaling or routing: %+v", first)
	}
	pump, toggle := parts[2], parts[4]
	if pump.Kind != "Bit Lamp" || pump.Write != nil || pump.Read == nil || pump.Read.Area != "CIO" || pump.Read.Index != 19 {
		t.Fatalf("unexpected pump binding: %+v", pump)
	}
	if toggle.Kind != "Switch" || toggle.Mode != 2 || toggle.Read == nil || toggle.Write == nil ||
		*toggle.Read != *toggle.Write || toggle.Write.Area != "LB" || toggle.Write.Index != 20 {
		t.Fatalf("unexpected toggle binding: %+v", toggle)
	}
	if err := ExportMimic(source, bindings, out); err == nil {
		t.Fatal("export overwrote the existing destination")
	}
}

func TestMimicRejectsChangedControlBehavior(t *testing.T) {
	const prefix = `{"canvas":{"width":800,"height":480},"equipment":[{"id":"cmd","x":20,"y":20,`
	tests := []struct{ input, want string }{
		{`"component":"CommandButton","props":{"tag":"Run"}}]}`, "component cannot be exported"},
		{`"component":"WriteToggle","props":{"tag":"Run","readonly":true},"bind":{"value":"Run"}}]}`, "read-only"},
		{`"component":"WriteToggle","props":{"tag":"Run"},"bind":{"value":"Readback"}}]}`, "same tag"},
		{`"component":"WriteToggle","props":{"tag":"Run"},"bind":{"value":"Run"}}]}`, "explicit NB bit address"},
	}
	for _, test := range tests {
		_, err := convertMimic([]byte(prefix+test.input), "test", nil)
		if err == nil || !strings.Contains(err.Error(), test.want) || !strings.Contains(err.Error(), `"cmd"`) {
			t.Errorf("got %v, want equipment id and %q", err, test.want)
		}
	}
}
