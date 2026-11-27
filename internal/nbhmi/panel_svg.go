package nbhmi

import (
	"encoding/xml"
	"fmt"
	"io/fs"
	"slices"
	"strconv"
	"strings"

	"github.com/joyautomation/nautilus/internal/nbhmi/nbpanel"
)

// panelSVGArtwork reads the SVG vocabulary emitted by the NB project importer.
func panelSVGArtwork(project fs.FS, source []byte) (nbpanel.VectorArtwork, []byte, error) {
	var root element
	var state nbpanel.VectorArtwork
	if err := xml.Unmarshal(source, &root); err != nil {
		return state, nil, fmt.Errorf("SVG: %w", err)
	}
	if root.XMLName != (xml.Name{Space: "http://www.w3.org/2000/svg", Local: "svg"}) {
		return state, nil, fmt.Errorf("expected an SVG artwork document")
	}
	if err := panelSVGAttrs(&root, "xmlns", "width", "height", "viewBox", "preserveAspectRatio"); err != nil {
		return state, nil, err
	}
	size, err := panelSVGInts(&root, "width", "height")
	if err != nil {
		return state, nil, err
	}
	state.Width, state.Height = size[0], size[1]
	if state.Width < 1 || state.Height < 1 ||
		!slices.Equal(strings.Fields(root.attr("viewBox")),
			[]string{"0", "0", strconv.Itoa(state.Width), strconv.Itoa(state.Height)}) ||
		root.attr("preserveAspectRatio") != "none" {
		return state, nil, fmt.Errorf("expected a matching origin-zero SVG viewBox and preserveAspectRatio=none")
	}
	if len(root.Children) == 1 && root.Children[0].XMLName.Local == "image" {
		data, err := panelSVGEmbeddedBitmap(project, root.Children[0], state.Width, state.Height)
		return state, data, err
	}

	gradients := make(map[string][2]string)
	for _, node := range root.children("defs") {
		if err := panelSVGGradients(node, gradients); err != nil {
			return state, nil, err
		}
	}
	for _, node := range root.Children {
		if node.XMLName.Local == "defs" {
			continue
		}
		shape, err := panelSVGPrimitive(node, gradients)
		if err != nil {
			return state, nil, err
		}
		if shape.Fill == nil && shape.StrokeWidth == 0 {
			continue
		}
		state.Shapes = append(state.Shapes, shape)
	}
	return state, nil, nil
}

func panelSVGEmbeddedBitmap(project fs.FS, node *element, width, height int) ([]byte, error) {
	if err := panelSVGAttrs(node, "x", "y", "width", "height", "href", "preserveAspectRatio"); err != nil {
		return nil, err
	}
	b, err := panelSVGInts(node, "x", "y", "width", "height")
	if err != nil {
		return nil, err
	}
	href := node.attr("href")
	if len(node.Children) != 0 || !slices.Equal(b, []int{0, 0, width, height}) ||
		node.attr("preserveAspectRatio") != "none" || !strings.HasPrefix(href, "data:") {
		return nil, fmt.Errorf("expected a single embedded bitmap covering the SVG canvas")
	}
	return panelBitmapSource(project, href)
}

func panelSVGGradients(defs *element, gradients map[string][2]string) error {
	if err := panelSVGAttrs(defs); err != nil {
		return err
	}
	for _, node := range defs.Children {
		if node.XMLName.Local != "linearGradient" || len(node.Children) != 2 ||
			node.attr("x2") != "0" || node.attr("y2") != "1" {
			return fmt.Errorf("expected a vertical SVG linear gradient with two stops")
		}
		if err := panelSVGAttrs(node, "id", "x2", "y2"); err != nil {
			return err
		}
		id := node.attr("id")
		if _, exists := gradients[id]; exists || id == "" {
			return fmt.Errorf("invalid or duplicate SVG gradient ID %q", id)
		}
		var colors [2]string
		for i, stop := range node.Children {
			if stop.XMLName.Local != "stop" || len(stop.Children) != 0 || stop.attr("offset") != strconv.Itoa(i) {
				return fmt.Errorf("expected SVG gradient stops at 0 and 1")
			}
			if err := panelSVGAttrs(stop, "offset", "stop-color"); err != nil {
				return err
			}
			colors[i] = stop.attr("stop-color")
			if _, err := panelColor(colors[i]); err != nil {
				return fmt.Errorf("SVG gradient %q: %w", id, err)
			}
		}
		gradients[id] = colors
	}
	return nil
}

func panelSVGPrimitive(node *element, gradients map[string][2]string) (nbpanel.Primitive, error) {
	d := drawing{Kind: node.XMLName.Local, Fill: node.attr("fill"), Stroke: node.attr("stroke")}
	if len(node.Children) != 0 {
		return nbpanel.Primitive{}, fmt.Errorf("unexpected children in SVG %s", d.Kind)
	}
	strokeWidth, err := panelSVGInt(node.attr("stroke-width"))
	if err != nil {
		return nbpanel.Primitive{}, fmt.Errorf("SVG %s stroke-width: %w", d.Kind, err)
	}
	d.StrokeWidth = strokeWidth
	if d.Fill == "" {
		d.Fill = "#000000"
	}
	if node.attr("stroke-width") != "" && strokeWidth == 0 {
		d.Stroke = "none"
	}
	switch d.Kind {
	case "rect":
		if err := panelSVGAttrs(node, "x", "y", "width", "height", "rx", "fill", "stroke", "stroke-width"); err != nil {
			return nbpanel.Primitive{}, err
		}
		b, err := panelSVGInts(node, "x", "y", "width", "height", "rx")
		if err != nil {
			return nbpanel.Primitive{}, err
		}
		d.Bounds, d.Radius = bounds{b[0], b[1], b[2], b[3]}, b[4]
		if d.Radius > 0 {
			d.Kind = "roundrect"
		} else if d.Radius < 0 {
			return nbpanel.Primitive{}, fmt.Errorf("invalid SVG rectangle radius")
		}
	case "line":
		if err := panelSVGAttrs(node, "x1", "y1", "x2", "y2", "fill", "stroke", "stroke-width"); err != nil {
			return nbpanel.Primitive{}, err
		}
		b, err := panelSVGInts(node, "x1", "y1", "x2", "y2")
		if err != nil {
			return nbpanel.Primitive{}, err
		}
		d.Bounds.X, d.Bounds.Y, d.X2, d.Y2 = b[0], b[1], b[2], b[3]
	case "polygon":
		if err := panelSVGAttrs(node, "points", "fill", "stroke", "stroke-width"); err != nil {
			return nbpanel.Primitive{}, err
		}
		for _, pair := range strings.Fields(node.attr("points")) {
			x, y, ok := strings.Cut(pair, ",")
			if !ok || x == "" || y == "" {
				return nbpanel.Primitive{}, fmt.Errorf("invalid SVG polygon point %q", pair)
			}
			px, xErr := panelSVGInt(x)
			py, yErr := panelSVGInt(y)
			if xErr != nil || yErr != nil {
				return nbpanel.Primitive{}, fmt.Errorf("invalid SVG polygon point %q", pair)
			}
			d.Points = append(d.Points, point{px, py})
		}
	default:
		return nbpanel.Primitive{}, fmt.Errorf("unsupported SVG artwork element %q", d.Kind)
	}
	if id, ok := strings.CutPrefix(d.Fill, "url(#"); ok {
		colors, found := gradients[strings.TrimSuffix(id, ")")]
		if !strings.HasSuffix(id, ")") || !found {
			return nbpanel.Primitive{}, fmt.Errorf("unknown SVG gradient reference %q", d.Fill)
		}
		d.Fill, d.EndFill = colors[0], colors[1]
	}
	return panelPrimitive(d)
}

func panelSVGAttrs(node *element, names ...string) error {
	if node.XMLName.Space != "http://www.w3.org/2000/svg" || strings.TrimSpace(node.Text) != "" {
		return fmt.Errorf("invalid SVG %s element", node.XMLName.Local)
	}
	seen := make(map[string]bool, len(node.Attrs))
	for _, attr := range node.Attrs {
		name := attr.Name.Local
		if attr.Name.Space != "" || !slices.Contains(names, name) || seen[name] {
			return fmt.Errorf("unsupported or duplicate SVG %s attribute %q", node.XMLName.Local, name)
		}
		seen[name] = true
	}
	return nil
}

func panelSVGInts(node *element, names ...string) ([]int, error) {
	values := make([]int, len(names))
	for i, name := range names {
		value, err := panelSVGInt(node.attr(name))
		if err != nil {
			return nil, fmt.Errorf("SVG %s %s: %w", node.XMLName.Local, name, err)
		}
		values[i] = value
	}
	return values, nil
}

func panelSVGInt(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(value, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("expected an integer coordinate or dimension, got %q", value)
	}
	return int(n), nil
}
