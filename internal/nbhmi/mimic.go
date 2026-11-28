package nbhmi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/joyautomation/nautilus/internal/mimicdoc"
	"github.com/joyautomation/nautilus/internal/nbhmi/nbpanel"
)

const (
	mimicPanelWidth  = 800
	mimicPanelHeight = 480
)

type mimicScreen struct {
	Name      string
	Canvas    struct{ Width, Height float64 }
	Equipment []mimicEquipment
	Pipes     []mimicPipe
	Labels    []struct {
		Text, Bind string
		X, Y       float64
	}
}

type mimicEquipment struct {
	ID, Component, Label string
	X, Y, Width, Height  float64
	Behind               bool
	Props                json.RawMessage
	Bind                 map[string]string
}

type mimicPipe struct {
	ID, Routing string
	Points      [][]float64
	From, To    json.RawMessage
	Props       map[string]json.RawMessage
	Bind        map[string]string
}

type mimicLayout struct {
	width, height    float64
	scale, left, top float64
}

// ExportMimic writes a new NB10W project directory from one Nautilus mimic.
// Bindings is an optional JSON object mapping tag names to NB bit addresses.
func ExportMimic(source, bindings, destination string) error {
	raw, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	bits, err := loadMimicBindings(bindings)
	if err != nil {
		return err
	}
	title := strings.TrimSuffix(filepath.Base(source), ".mimic.json")
	doc, err := convertMimic(raw, title, bits)
	if err != nil {
		return fmt.Errorf("%s: %w", source, err)
	}
	model, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return publish(destination, map[string][]byte{path.Join("hmi", ModelPath): append(model, '\n')})
}

func convertMimic(raw []byte, title string, bits map[string]register) (document, error) {
	raw, _, err := mimicdoc.ExpandUnits(raw)
	if err != nil {
		return document{}, err
	}
	var source mimicScreen
	if err := json.Unmarshal(raw, &source); err != nil {
		return document{}, err
	}
	w, h := source.Canvas.Width, source.Canvas.Height
	k := min(mimicPanelWidth/w, mimicPanelHeight/h)
	if w <= 0 || h <= 0 || math.IsInf(k, 0) || w*k < 1 || h*k < 1 {
		return document{}, fmt.Errorf("expected positive canvas dimensions that fit the panel")
	}
	layout := mimicLayout{w, h, k, (mimicPanelWidth - w*k) / 2, (mimicPanelHeight - h*k) / 2}
	if source.Name != "" {
		title = source.Name
	}
	screen := window{
		Name: title, Background: "#000000", Bounds: bounds{W: mimicPanelWidth, H: mimicPanelHeight},
	}
	var foreground []part
	seen := make(map[string]bool)
	for i, eq := range source.Equipment {
		if eq.ID == "" || seen[eq.ID] {
			return document{}, fmt.Errorf("equipment %d: expected a unique nonempty id, got %q", i+1, eq.ID)
		}
		seen[eq.ID] = true
		parts, err := layout.equipment(eq, bits)
		if err != nil {
			return document{}, fmt.Errorf("equipment %q (%s): %w", eq.ID, eq.Component, err)
		}
		if eq.Behind {
			screen.Parts = append(screen.Parts, parts...)
		} else {
			foreground = append(foreground, parts...)
		}
	}
	for _, pipe := range source.Pipes {
		parts, err := layout.pipe(pipe)
		if err != nil {
			return document{}, fmt.Errorf("pipe %q: %w", pipe.ID, err)
		}
		screen.Parts = append(screen.Parts, parts...)
	}
	screen.Parts = append(screen.Parts, foreground...)
	for i, item := range source.Labels {
		if item.Bind != "" {
			return document{}, fmt.Errorf("label %d (%q): live numeric readouts are unsupported", i+1, item.Text)
		}
		if item.Text == "" {
			continue
		}
		if !layout.contains(item.X, item.Y) {
			return document{}, fmt.Errorf("label %d (%q): position is outside the canvas", i+1, item.Text)
		}
		p := layout.pixel(item.X, item.Y)
		font := layout.fontSize()
		if mimicPanelHeight-p.Y < font {
			return document{}, fmt.Errorf("label %d (%q): too close to the bottom of the panel", i+1, item.Text)
		}
		screen.Parts = append(screen.Parts, mimicText(item.Text, item.Text, bounds{
			X: p.X, Y: p.Y, W: mimicPanelWidth - p.X, H: min(font+4, mimicPanelHeight-p.Y),
		}, font))
	}
	if len(screen.Parts) == 0 {
		return document{}, fmt.Errorf("mimic has no exportable screen elements")
	}
	return document{Version: 1, Name: title, Title: title, Width: mimicPanelWidth, Height: mimicPanelHeight, Windows: []window{screen}}, nil
}

func (l mimicLayout) contains(x, y float64) bool {
	return x >= 0 && y >= 0 && x <= l.width && y <= l.height
}

func (l mimicLayout) pixel(x, y float64) point {
	return point{
		min(int(math.Round(l.left+x*l.scale)), mimicPanelWidth-1),
		min(int(math.Round(l.top+y*l.scale)), mimicPanelHeight-1),
	}
}

func (l mimicLayout) box(e mimicEquipment, defaultWidth, defaultHeight float64) (bounds, error) {
	w, h := e.Width, e.Height
	if w == 0 {
		w = defaultWidth
	}
	if h == 0 {
		h = w * defaultHeight / defaultWidth
	}
	if w <= 0 || h <= 0 || !l.contains(e.X, e.Y) || !l.contains(e.X+w, e.Y+h) {
		return bounds{}, fmt.Errorf("bounds are outside the canvas")
	}
	p := l.pixel(e.X, e.Y)
	right := int(math.Round(l.left + (e.X+w)*l.scale))
	bottom := int(math.Round(l.top + (e.Y+h)*l.scale))
	if right <= p.X || bottom <= p.Y {
		return bounds{}, fmt.Errorf("element is smaller than one panel pixel")
	}
	return bounds{X: p.X, Y: p.Y, W: right - p.X, H: bottom - p.Y}, nil
}

func (l mimicLayout) fontSize() int {
	return int(math.Round(max(8, min(32, 14*l.scale))))
}

func (l mimicLayout) pipe(p mimicPipe) ([]part, error) {
	if len(p.From) != 0 && string(p.From) != "null" || len(p.To) != 0 && string(p.To) != "null" {
		return nil, fmt.Errorf("expected explicit pipe points without equipment port anchors")
	}
	if len(p.Props) != 0 || len(p.Bind) != 0 {
		return nil, fmt.Errorf("only static pipes are supported")
	}
	if len(p.Points) < 2 {
		return nil, fmt.Errorf("expected at least two pipe points")
	}
	if p.Routing != "" && p.Routing != "direct" && p.Routing != "orthogonal" {
		return nil, fmt.Errorf("unsupported routing %q", p.Routing)
	}
	const stroke = 2
	points := make([]point, 0, len(p.Points))
	for i, xy := range p.Points {
		if len(xy) != 2 || !l.contains(xy[0], xy[1]) {
			return nil, fmt.Errorf("point %d must be an XY pair inside the canvas", i+1)
		}
		pixel := l.pixel(xy[0], xy[1])
		pixel.X = max(stroke, min(mimicPanelWidth-stroke, pixel.X))
		pixel.Y = max(stroke, min(mimicPanelHeight-stroke, pixel.Y))
		points = append(points, pixel)
	}
	if p.Routing == "orthogonal" {
		points = orthogonalMimicPoints(points)
	}
	var parts []part
	for i := 1; i < len(points); i++ {
		a, b := points[i-1], points[i]
		if a != b {
			parts = append(parts, part{Kind: "draw", Name: p.ID, Drawing: &drawing{
				Kind: "line", Bounds: bounds{X: a.X, Y: a.Y}, X2: b.X, Y2: b.Y,
				Stroke: "#9a9a9a", StrokeWidth: stroke,
			}})
		}
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("all pipe points coincide at panel resolution")
	}
	return parts, nil
}

// Match the Mimic orthogonal route: continue along the previous
// arrival axis, or start with the larger delta.
func orthogonalMimicPoints(points []point) []point {
	out := append(make([]point, 0, len(points)*2), points[0])
	var axisSet, horizontal bool
	for i := 1; i < len(points); i++ {
		a, b := points[i-1], points[i]
		if a.X != b.X && a.Y != b.Y {
			if !axisSet {
				horizontal = max(a.X-b.X, b.X-a.X) >= max(a.Y-b.Y, b.Y-a.Y)
			}
			corner := point{X: a.X, Y: b.Y}
			if horizontal {
				corner = point{X: b.X, Y: a.Y}
			}
			out = append(out, corner)
			horizontal, axisSet = !horizontal, true
		} else if a != b {
			horizontal, axisSet = a.Y == b.Y, true
		}
		out = append(out, b)
	}
	return out
}

func mimicText(name, text string, box bounds, font int) part {
	return part{Kind: "draw", Name: name, Drawing: &drawing{
		Kind: "text", Bounds: box, Text: &label{Text: text, Size: font, Color: "#ffffff"},
	}}
}

func decodeMimicProps(raw json.RawMessage, target any) error {
	if len(raw) == 0 {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func (l mimicLayout) equipment(e mimicEquipment, bits map[string]register) ([]part, error) {
	switch e.Component {
	case "WriteToggle":
		var props struct {
			Tag, Label, OnColor, OffColor, ReadonlyReason string
			Value, Alarm, Invert, Readonly                bool
			Present                                       *bool
		}
		if err := decodeMimicProps(e.Props, &props); err != nil {
			return nil, err
		}
		if props.Alarm || props.Invert || props.Readonly || props.Present != nil && !*props.Present {
			return nil, fmt.Errorf("alarm, inverted, read-only or absent toggles are unsupported")
		}
		if len(e.Bind) != 1 || props.Tag == "" || e.Bind["value"] != props.Tag {
			return nil, fmt.Errorf("WriteToggle requires the same tag in props.tag and bind.value")
		}
		bit, err := mimicBinding(bits, props.Tag)
		if err != nil {
			return nil, err
		}
		box, err := l.box(e, 120, 48)
		if err != nil {
			return nil, err
		}
		font := min(l.fontSize(), box.H-4)
		if font < 8 || box.W < 9 {
			return nil, fmt.Errorf("toggle is too small for a caption at panel resolution")
		}
		caption := &bounds{X: 4, Y: (box.H - font) / 2, W: box.W - 8, H: font + 2}
		text := mimicCaption(props.Label, e.Label, e.ID)
		return []part{{
			Kind: "Switch", Name: e.ID, Bounds: box, Mode: 2,
			Read: &bit, Write: &bit, LabelBox: caption,
			Labels: []label{{Text: text, Size: font, Color: "#ffffff"}},
		}}, nil
	case "Pump":
		var props struct {
			Label, Look string
			Running     bool
			SpeedPct    float64
		}
		if err := decodeMimicProps(e.Props, &props); err != nil {
			return nil, err
		}
		if len(e.Bind) > 1 || len(e.Bind) == 1 && e.Bind["running"] == "" {
			return nil, fmt.Errorf("Pump supports the running bit binding")
		}
		box, err := l.box(e, 120, 110)
		if err != nil {
			return nil, err
		}
		font := min(l.fontSize(), box.H/2)
		if font < 8 {
			return nil, fmt.Errorf("pump is too small for a label at panel resolution")
		}
		marker := min(box.W, box.H-font-4)
		markerBox := bounds{X: box.X, Y: box.Y, W: marker, H: marker}
		indicator := part{Kind: "draw", Name: e.ID, Drawing: &drawing{
			Kind: "rect", Bounds: markerBox, Fill: "#5b5b5b",
		}}
		if tag := e.Bind["running"]; tag != "" {
			bit, err := mimicBinding(bits, tag)
			if err != nil {
				return nil, err
			}
			indicator = part{Kind: "Bit Lamp", Name: e.ID, Bounds: markerBox, Read: &bit}
		} else if props.Running {
			indicator.Drawing.Fill = "#00b060"
		}
		caption := mimicCaption(props.Label, e.Label, e.ID)
		text := mimicText(e.ID+" label", caption, bounds{X: box.X, Y: box.Y + marker + 2, W: box.W, H: font + 2}, font)
		return []part{indicator, text}, nil
	default:
		return nil, fmt.Errorf("component cannot be exported to NB")
	}
}

func mimicCaption(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

var mimicBitAddress = regexp.MustCompile(`^(?:LB([0-9]+)|CIO([0-9]+)\.([0-9]{1,2}))$`)

func loadMimicBindings(filename string) (map[string]register, error) {
	if filename == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var addresses map[string]string
	if err := json.Unmarshal(raw, &addresses); err != nil || addresses == nil {
		return nil, fmt.Errorf("%s: expected a JSON object mapping tags to NB bit addresses", filename)
	}
	bits := make(map[string]register, len(addresses))
	for _, tag := range sortedKeys(addresses) {
		address := strings.ToUpper(strings.TrimSpace(addresses[tag]))
		match := mimicBitAddress.FindStringSubmatch(address)
		if len(match) == 4 && match[1] != "" {
			index, err := strconv.Atoi(match[1])
			if err == nil && index < nbpanel.LocalBitLimit {
				bits[tag] = register{Key: fmt.Sprintf("LB%d", index), Area: "LB", Index: index, Count: 1}
				continue
			}
		} else if len(match) == 4 {
			word, wordErr := strconv.Atoi(match[2])
			bit, bitErr := strconv.Atoi(match[3])
			if wordErr == nil && bitErr == nil && word < nbpanel.CIOBitLimit/16 && bit < 16 {
				bits[tag] = register{Key: cioBitTag(word, bit), Area: "CIO", Index: word*16 + bit, Count: 1}
				continue
			}
		}
		return nil, fmt.Errorf("%s: invalid NB bit address %q for tag %q", filename, addresses[tag], tag)
	}
	return bits, nil
}

func mimicBinding(bits map[string]register, tag string) (register, error) {
	if tag != strings.TrimSpace(tag) || strings.HasPrefix(tag, "!") {
		return register{}, fmt.Errorf("unsupported tag expression %q", tag)
	}
	bit, ok := bits[tag]
	if !ok {
		return register{}, fmt.Errorf("tag %q needs an explicit NB bit address in --bindings", tag)
	}
	return bit, nil
}
