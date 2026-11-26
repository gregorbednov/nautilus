package nbhmi

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"html"
	"io"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

type drawing struct {
	Kind        string  `json:"kind"`
	Bounds      bounds  `json:"bounds"`
	X2          int     `json:"x2"`
	Y2          int     `json:"y2"`
	Stroke      string  `json:"stroke"`
	StrokeWidth int     `json:"strokeWidth"`
	Fill        string  `json:"fill"`
	EndFill     string  `json:"endFill,omitempty"`
	Radius      int     `json:"radius,omitempty"`
	Points      []point `json:"points,omitempty"`
	Text        *label  `json:"text,omitempty"`
	Image       string  `json:"image,omitempty"`
}

func color(s string) string {
	n, err := strconv.ParseUint(strings.TrimPrefix(strings.ToLower(s), "0x"), 16, 32)
	if err != nil {
		return "#000000"
	}
	return fmt.Sprintf("#%06x", n&0xffffff)
}

func (c *converter) drawing(base string, e *element) (drawing, error) {
	x, y := number(e.attr("pos_sx")), number(e.attr("pos_sy"))
	x2, y2 := number(e.attr("pos_ex")), number(e.attr("pos_ey"))
	d := drawing{
		Kind: e.attr("type"), Bounds: bounds{min(x, x2), min(y, y2), abs(x2-x) + 1, abs(y2-y) + 1},
		X2: x2, Y2: y2, Stroke: color(e.attr("pencolor")), StrokeWidth: max(1, number(e.attr("penwidth"))), Fill: "none",
	}
	if e.attr("pencolor") == "none" || e.attr("penstyle") == "6" {
		d.Stroke = "none"
	}
	if e.attr("fill") != "" && e.attr("fill") != "none" {
		d.Fill = color(e.attr("fillcolor"))
	}
	if e.attr("GradientFill") == "true" {
		d.Fill, d.EndFill = color(e.attr("BackgroundColor")), color(e.attr("ForegroundColor"))
	}
	switch d.Kind {
	case "rect":
	case "roundrect":
		d.Radius = max(1, min(number(e.attr("spoint_x")), number(e.attr("spoint_y"))))
	case "line":
		d.Bounds.X, d.Bounds.Y = x, y
		if d.Stroke == "none" && e.attr("pencolor") != "none" {
			d.Stroke = color(e.attr("pencolor"))
		}
	case "polygon":
		for _, p := range e.children("point") {
			d.Points = append(d.Points, point{number(p.attr("x")), number(p.attr("y"))})
		}
	case "text":
		l := fontLabel(e, e.attr("src"))
		d.Text = &l
	case "image":
		ref, err := resolveNB(c.root, base, e.attr("src"))
		if err != nil {
			return d, err
		}
		href, err := c.bitmap(ref)
		if err != nil {
			return d, err
		}
		d.Image = href
	default:
		return d, fmt.Errorf("unsupported primitive %q", d.Kind)
	}
	return d, nil
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func (c *converter) bitmap(name string) (string, error) {
	f, err := c.root.Open(filepath.FromSlash(name))
	if err != nil {
		return "", err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxSourceSize+1))
	if err != nil {
		return "", err
	}
	if len(b) < 54 || len(b) > maxSourceSize || !bytes.HasPrefix(b, []byte("BM")) {
		return "", fmt.Errorf("%s: expected BMP image", name)
	}
	return "data:image/bmp;base64," + base64.StdEncoding.EncodeToString(b), nil
}

func (c *converter) shape(ref string) (map[int]string, error) {
	name, err := resolveNB(c.root, ".", ref)
	if err != nil {
		return nil, err
	}
	if states := c.shapes[name]; states != nil {
		return states, nil
	}
	g, err := readXML(c.root, name)
	if err != nil {
		return nil, err
	}
	w, h := number(g.attr("width")), number(g.attr("height"))
	if g.XMLName.Local != "vg" || w <= 0 || h <= 0 {
		return nil, fmt.Errorf("%s: invalid vector graphic", name)
	}
	states := map[int]string{}
	for _, state := range g.children("state") {
		id, err := strconv.Atoi(state.attr("id"))
		if err != nil || states[id] != "" {
			return nil, fmt.Errorf("%s: invalid or duplicate graphic state", name)
		}
		var svg strings.Builder
		fmt.Fprintf(&svg, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" preserveAspectRatio="none">`, w, h, w, h)
		var drawings []drawing
		if err := c.vectorDrawings(path.Dir(name), state, &drawings); err != nil {
			return nil, fmt.Errorf("%s state %d: %w", name, id, err)
		}
		for i, d := range drawings {
			d.writeSVG(&svg, i)
		}
		states[id] = c.svgAsset(&svg)
	}
	if len(states) == 0 {
		return nil, fmt.Errorf("%s: no graphic states", name)
	}
	c.shapes[name] = states
	return states, nil
}

func (c *converter) vectorDrawings(base string, parent *element, drawings *[]drawing) error {
	// The first NB vector primitive is frontmost. SVG paints the last one on top.
	for i := len(parent.Children) - 1; i >= 0; i-- {
		e := parent.Children[i]
		switch e.XMLName.Local {
		case "draw":
			d, err := c.drawing(base, e)
			if err != nil {
				return err
			}
			*drawings = append(*drawings, d)
		case "Group":
			parts := e.child("GParts")
			if parts == nil {
				return fmt.Errorf("vector group has no GParts")
			}
			if err := c.vectorDrawings(base, parts, drawings); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported vector element %q", e.XMLName.Local)
		}
	}
	return nil
}

func (c *converter) logo(ref string) (string, error) {
	name, err := resolveNB(c.root, ".", ref)
	if err != nil {
		return "", err
	}
	logo, err := readXML(c.root, name)
	if err != nil {
		return "", err
	}
	window := logo.child("Window")
	w, h := number(window.attr("Size_cx")), number(window.attr("Size_cy"))
	if logo.XMLName.Local != "Logo" || w <= 0 || h <= 0 {
		return "", fmt.Errorf("%s: invalid logo window", name)
	}
	var svg strings.Builder
	fmt.Fprintf(&svg, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d"><rect width="100%%" height="100%%" fill="%s"/>`,
		w, h, w, h, color(window.attr("BKColor")))
	for i, e := range window.children("draw") {
		d, err := c.drawing(path.Dir(name), e)
		if err != nil {
			return "", fmt.Errorf("%s: %w", name, err)
		}
		d.writeSVG(&svg, i)
	}
	return c.svgAsset(&svg), nil
}

func (c *converter) svgAsset(svg *strings.Builder) string {
	svg.WriteString("</svg>\n")
	data := []byte(svg.String())
	digest := sha256.Sum256(data)
	file := fmt.Sprintf("nb-assets/%x.svg", digest[:12])
	c.files["hmi/static/"+file] = data
	return "/" + file
}

func (d drawing) writeSVG(svg *strings.Builder, index int) {
	b := d.Bounds
	fill := d.Fill
	if d.EndFill != "" {
		fmt.Fprintf(svg, `<defs><linearGradient id="g%d" x2="0" y2="1"><stop offset="0" stop-color="%s"/><stop offset="1" stop-color="%s"/></linearGradient></defs>`, index, fill, d.EndFill)
		fill = fmt.Sprintf("url(#g%d)", index)
	}
	style := fmt.Sprintf(`stroke="%s" stroke-width="%d" fill="%s"`, d.Stroke, d.StrokeWidth, fill)
	switch d.Kind {
	case "rect", "roundrect":
		fmt.Fprintf(svg, `<rect x="%d" y="%d" width="%d" height="%d" rx="%d" %s/>`, b.X, b.Y, b.W, b.H, d.Radius, style)
	case "line":
		fmt.Fprintf(svg, `<line x1="%d" y1="%d" x2="%d" y2="%d" %s/>`, b.X, b.Y, d.X2, d.Y2, style)
	case "polygon":
		var points strings.Builder
		for _, p := range d.Points {
			fmt.Fprintf(&points, "%d,%d ", p.X, p.Y)
		}
		fmt.Fprintf(svg, `<polygon points="%s" %s/>`, points.String(), style)
	case "image":
		fmt.Fprintf(svg, `<image x="%d" y="%d" width="%d" height="%d" href="%s" preserveAspectRatio="none"/>`, b.X, b.Y, b.W, b.H, d.Image)
	case "text":
		l := d.Text
		for i, row := range strings.Split(l.Text, "\n") {
			fmt.Fprintf(svg, `<text x="%d" y="%d" font-size="%d" fill="%s">%s</text>`, b.X, b.Y+(i+1)*l.Size, l.Size, l.Color, html.EscapeString(row))
		}
	}
}
