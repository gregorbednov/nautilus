package nbhmi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sourceProject = `<NBProject Name="fixture" DBVersion="137">
 <PLCs><PLC ProjectID="0" SystemID="448"><NetPort><IP><DefaultParam><Type>UDP</Type></DefaultParam></IP></NetPort></PLC></PLCs>
 <HMIs><HMI URL=".\HMI0\hmi0.WHE"><Info><ExtInfo><DefaultWindow>0</DefaultWindow><Password0>fixture-password</Password0></ExtInfo></Info></HMI></HMIs>
</NBProject>`

const sourceScreen = `<HMI Name="Fixture">
 <Frames><Window ProjectID="0" Name="Main" BackgroundColor="0x123456">
  <Position><Width>800</Width><Height>480</Height></Position>
  <Parts><Group><GParts>
   <draw type="text" src="Текст$BK_RETURN$Пример" pos_sx="10" pos_sy="20" pos_ex="100" pos_ey="40"/>
   <Part Name="S0" Type="Switch"><Mode>2</Mode>
    <Address Function="Read"><RegAddr WordCount="1"><PLCID>0</PLCID><AddressType>CIO_bit</AddressType><AddressValue>2.03</AddressValue></RegAddr></Address>
    <Address Function="Write"><RegAddr WordCount="1"><PLCID>0</PLCID><AddressType>CIO_bit</AddressType><AddressValue>2.03</AddressValue></RegAddr></Address>
    <Lable><States><State><Text><Font>Pressed</Font></Text></State><State><Text><Font>Released</Font></Text></State></States></Lable>
    <Shape URL=".\vg\button.vg"/><Position><Left>20</Left><Top>50</Top><Right>79</Right><Bottom>79</Bottom></Position>
   </Part>
  </GParts></Group></Parts>
 </Window></Frames>
</HMI>`

func writeSource(t *testing.T, dir, project, screen string) {
	t.Helper()
	for name, contents := range map[string]string{
		"fixture.nbp":   project,
		"Hmi0/HMI0.whe": screen,
		"vg/button.vg": `<vg width="20" height="10"><state id="0"><Group><GParts>
			<draw type="rect" pos_sx="2" pos_sy="2" pos_ex="17" pos_ey="7" fill="color" fillcolor="0xff0000"/>
			<draw type="rect" pos_ex="19" pos_ey="9" fill="color" fillcolor="0x808080"/>
		</GParts></Group></state></vg>`,
	} {
		name = filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestImport(t *testing.T) {
	source, output := t.TempDir(), t.TempDir()
	writeSource(t, source, sourceProject, sourceScreen)
	first, second := filepath.Join(output, "first"), filepath.Join(output, "second")
	result, err := Import(source, first)
	if err != nil {
		t.Fatal(err)
	}
	if result.Components != 1 || result.Windows != 1 || result.Tags != 1 {
		t.Fatalf("result: %+v", result)
	}
	if _, err := Import(source, first); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("overwrite error: %v", err)
	}
	if _, err := Import(source, second); err != nil {
		t.Fatal(err)
	}
	read := func(root, rel string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	const schema = "hmi/src/lib/nb-hmi.json"
	data := read(first, schema)
	if data != read(second, schema) || strings.Contains(data, "fixture-password") {
		t.Fatal("model is nondeterministic or includes a legacy password")
	}
	var doc document
	if err := json.Unmarshal([]byte(data), &doc); err != nil {
		t.Fatal(err)
	}
	text, button := doc.Windows[0].Parts[0], doc.Windows[0].Parts[1]
	if text.Drawing.Text.Text != "Текст\nПример" || button.Write.Key != "NB_CIO_0002_03" || button.Bounds.W != 60 {
		t.Fatalf("unexpected conversion: %+v, %+v", text, button)
	}
	if len(button.Labels) != 2 || button.Labels[0].Text != "Released" || button.Labels[1].Text != "Pressed" {
		t.Fatalf("labels do not match the vector states: %+v", button.Labels)
	}
	asset := read(first, "hmi/static"+button.Assets[0])
	base, face := strings.Index(asset, `fill="#808080"`), strings.Index(asset, `fill="#ff0000"`)
	if base < 0 || face <= base {
		t.Fatalf("grouped button base must paint before its face: %s", asset)
	}
	if !strings.Contains(read(first, "nautilus.yaml"), "CIO2.03") {
		t.Fatal("missing PLC alias")
	}
	bindings, err := ModelBindings(os.DirFS(first), "hmi")
	if err != nil || len(bindings) != 1 || !strings.Contains(bindings["NB_CIO_0002_03"], `window 0 (Main), Switch "S0"`) {
		t.Fatalf("model bindings: %v, %v", bindings, err)
	}
}

func TestUnsupportedInputDoesNotPublish(t *testing.T) {
	for _, tc := range []struct{ name, project, screen, want string }{
		{"version", strings.Replace(sourceProject, "137", "138", 1), sourceScreen, "DBVersion"},
		{"component", sourceProject, strings.Replace(sourceScreen, "Switch", "Macro", 1), "window 0 (Main): Macro"},
		{"missing", sourceProject, strings.Replace(sourceScreen, "button.vg", "missing.vg", 1), "missing project file"},
		{"traversal", sourceProject, strings.Replace(sourceScreen, `.\vg\button.vg`, `..\outside.vg`, 1), "invalid project path"},
		{"xml", sourceProject, sourceScreen + "<second-root/>", "unexpected content"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, dest := t.TempDir(), filepath.Join(t.TempDir(), "output")
			writeSource(t, source, tc.project, tc.screen)
			if _, err := Import(source, dest); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
			if _, err := os.Stat(dest); !os.IsNotExist(err) {
				t.Fatalf("unexpected output: %v", err)
			}
		})
	}
}

func TestImportNumberInputFormats(t *testing.T) {
	const component = `<Part Name="N0" Type="Number Input" LimitSwitch="Off">
 <DataSet DataType="1" Convert="No"><MinValue>0</MinValue><MaxValue>150</MaxValue><DecimalCount>2</DecimalCount></DataSet>
 <Address Function="Limit"><RegAddr WordCount="2"><AddressType>LW</AddressType><AddressValue>0</AddressValue></RegAddr></Address>
 <Address Function="Read"><RegAddr WordCount="1"><AddressType>LW</AddressType><AddressValue>10</AddressValue></RegAddr></Address>
 <Address Function="Write"><RegAddr WordCount="1"><AddressType>LW</AddressType><AddressValue>10</AddressValue></RegAddr></Address>
 <Enabled CtrlMode="1" bCondUser="1"/><CtrlEx AutoPWD="1"/>
 <Position><Left>20</Left><Top>100</Top><Right>79</Right><Bottom>129</Bottom></Position>
</Part>`
	for _, tc := range []struct {
		format   string
		decimals int
	}{
		{format: "1", decimals: 2},
		{format: "4", decimals: 0},
	} {
		t.Run(tc.format, func(t *testing.T) {
			source, dest := t.TempDir(), filepath.Join(t.TempDir(), "output")
			screen := strings.Replace(sourceScreen, "</GParts>", strings.Replace(component, `DataType="1"`, `DataType="`+tc.format+`"`, 1)+"</GParts>", 1)
			writeSource(t, source, sourceProject, screen)
			if _, err := Import(source, dest); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(dest, "hmi", ModelPath))
			if err != nil {
				t.Fatal(err)
			}
			var doc document
			if err := json.Unmarshal(data, &doc); err != nil {
				t.Fatal(err)
			}
			part := doc.Windows[0].Parts[2]
			if part.Meta["numberFormat"] != tc.format || part.Parameters[2] != tc.decimals || part.Write.Key != "LW10" {
				t.Fatalf("unexpected numeric model: %+v", part)
			}
			if part.High != nil || part.Low != nil {
				t.Fatalf("disabled limit address was bound: %+v", part)
			}
			if !part.AutoLogin || part.Enabled == nil || !part.Enabled.Login {
				t.Fatalf("protected input lost its login prompt: %+v", part)
			}
		})
	}
}
