package nbhmi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/joyautomation/nautilus/internal/nbhmi/nbpanel"
)

// BuildOfflinePanel compiles Nautilus screen records for the Omron simulator.
func BuildOfflinePanel(project, reference string) ([]byte, error) {
	root, err := os.OpenRoot(project)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	model := path.Join("hmi", ModelPath)
	projectFS := root.FS()
	data, err := fs.ReadFile(projectFS, model)
	if err != nil {
		return nil, err
	}
	var doc document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", model, err)
	}
	if doc.Version != 1 || doc.Width != 800 || doc.Height != 480 || doc.Start != 0 {
		return nil, fmt.Errorf("%s: expected a version 1 project, 800×480, starting at screen 0", model)
	}
	screens := make([]nbpanel.OfflineScreen, 0, len(doc.Windows))
	for _, w := range doc.Windows {
		if w.Bounds != (bounds{W: 800, H: 480}) || (w.Background != "" && w.Background != "#000000") {
			return nil, fmt.Errorf("window %q: expected a full-screen black window", w.Name)
		}
		screen := nbpanel.OfflineScreen{ID: w.ID, Objects: make([]nbpanel.Object, 0, len(w.Parts))}
		for _, p := range w.Parts {
			obj, err := panelObject(projectFS, p)
			if err != nil {
				return nil, fmt.Errorf("window %q, component %q: %w", w.Name, p.Name, err)
			}
			screen.Objects = append(screen.Objects, obj)
		}
		screens = append(screens, screen)
	}
	pkg, err := nbpanel.ReadPackage(reference)
	if err != nil {
		return nil, err
	}
	return pkg.BuildOffline(screens)
}

func panelObject(project fs.FS, p part) (nbpanel.Object, error) {
	var result nbpanel.Object
	wait := 0
	if p.Confirm {
		if p.Kind != "Number Input" && p.Kind != "Switch" {
			return result, fmt.Errorf("operation confirmation requires a Number Input or Switch")
		}
		wait = nbpanel.DefaultConfirmWaitSeconds
		if p.ConfirmWaitSeconds != nil {
			wait = *p.ConfirmWaitSeconds
		}
		if wait < 1 || wait > nbpanel.MaxConfirmWaitSeconds {
			return result, fmt.Errorf("confirmWaitSeconds must be between 1 and %d", nbpanel.MaxConfirmWaitSeconds)
		}
	} else if p.ConfirmWaitSeconds != nil {
		return result, fmt.Errorf("confirmWaitSeconds requires confirm: true")
	}
	if p.AutoLogin || p.Visible != nil || p.Enabled != nil ||
		p.Low != nil || p.High != nil {
		return result, fmt.Errorf("unsupported conditions or indirect limits")
	}
	if len(p.Assets) > 0 && p.Kind != "Bit Lamp" && p.Kind != "Switch" {
		return result, fmt.Errorf("custom state artwork requires a Bit Lamp or Switch")
	}
	switch p.Kind {
	case "draw":
		d := p.Drawing
		if d == nil || p.Read != nil || p.Write != nil {
			return result, fmt.Errorf("expected an unbound drawing")
		}
		return panelDrawing(project, *d)
	case "Function Key":
		if p.Function != "Change Frame" || p.Target == nil || p.Read != nil || p.Write != nil {
			return result, fmt.Errorf("expected an unbound Change Frame key with a destination")
		}
		caption, err := panelButtonCaption(p)
		if err != nil {
			return result, err
		}
		result = nbpanel.Navigation{
			X: p.Bounds.X, Y: p.Bounds.Y, Width: p.Bounds.W, Height: p.Bounds.H,
			Target: *p.Target, Label: caption,
		}
	case "Number Input":
		if p.Keyboard != nil && *p.Keyboard != 3 {
			return result, fmt.Errorf("expected the stock numeric keyboard in window 3")
		}
		if !panelRegister(p.Read, "LW", 2) || p.Write == nil || *p.Read != *p.Write {
			return result, fmt.Errorf("expected identical two-word local LW read and write bindings")
		}
		if p.Meta["numberFormat"] != "1" || len(p.Parameters) != 3 {
			return result, fmt.Errorf("expected decimal number format 1 with literal minimum, maximum and decimal count")
		}
		if len(p.Labels) != 0 && (len(p.Labels) != 1 ||
			p.Labels[0] != (label{Size: 32, Color: "#8b0000", Align: 1})) {
			return result, fmt.Errorf("expected the stock numeric field font")
		}
		result = nbpanel.NumberInput{
			X: p.Bounds.X, Y: p.Bounds.Y, Width: p.Bounds.W, Height: p.Bounds.H,
			Word: p.Write.Index, Min: p.Parameters[0], Max: p.Parameters[1], Decimals: p.Parameters[2],
			ConfirmWaitSeconds: wait,
		}
	case "Bit Lamp", "Switch":
		if !panelBitRegister(p.Read) {
			return result, fmt.Errorf("expected an LB or CIO bit read binding")
		}
		bit := nbpanel.BitControl{
			X: p.Bounds.X, Y: p.Bounds.Y, Width: p.Bounds.W, Height: p.Bounds.H,
			Index: p.Read.Index, CIO: p.Read.Area == "CIO", ConfirmWaitSeconds: wait,
		}
		if p.Kind == "Switch" {
			if p.Mode != 2 || p.Write == nil || *p.Read != *p.Write {
				return result, fmt.Errorf("expected a toggle switch with identical bit read and write bindings")
			}
			caption, err := panelButtonCaption(p)
			if err != nil {
				return result, err
			}
			bit.Caption = &caption
		} else if p.Write != nil || len(p.Labels) != 0 {
			return result, fmt.Errorf("expected an unlabeled read-only bit lamp")
		}
		if len(p.Assets) > 0 {
			artwork, err := panelBitArtwork(project, p.Assets)
			if err != nil {
				return result, err
			}
			bit.Artwork = artwork
		}
		result = bit
	default:
		return result, fmt.Errorf("unsupported offline component type %q", p.Kind)
	}
	return result, nil
}

func panelBitArtwork(project fs.FS, assets map[int]string) (*nbpanel.BitArtwork, error) {
	if len(assets) != 2 || assets[0] == "" || assets[1] == "" {
		return nil, fmt.Errorf("expected artwork for states 0 and 1")
	}
	var raster [2][]byte
	var vectors [2]nbpanel.VectorArtwork
	for state := range raster {
		source, err := panelBitmapSource(project, assets[state])
		if err != nil {
			return nil, fmt.Errorf("state %d: %w", state, err)
		}
		if bytes.HasPrefix(bytes.TrimSpace(source), []byte("<")) {
			vectors[state], raster[state], err = panelSVGArtwork(project, source)
			if err != nil {
				return nil, fmt.Errorf("state %d: %w", state, err)
			}
		} else {
			raster[state] = source
		}
	}
	if raster[0] == nil && raster[1] == nil {
		return nbpanel.NewVectorBitArtwork(vectors[0], vectors[1])
	}
	if raster[0] == nil || raster[1] == nil {
		return nil, fmt.Errorf("both states must use vector artwork or both must use bitmap artwork")
	}
	return nbpanel.NewBitArtwork(raster[0], raster[1])
}

func panelRegister(ref *register, area string, count int) bool {
	return ref != nil && ref.Area == area && ref.Count == count && ref.Key == fmt.Sprintf("%s%d", area, ref.Index)
}

func panelBitRegister(ref *register) bool {
	if ref != nil && ref.Area == "CIO" {
		return ref.Count == 1 && ref.Index >= 0 && ref.Index < nbpanel.CIOBitLimit &&
			ref.Key == cioBitTag(ref.Index/16, ref.Index%16)
	}
	return panelRegister(ref, "LB", 1)
}

func panelButtonCaption(p part) (nbpanel.Text, error) {
	if p.LabelBox == nil || (len(p.Labels) != 1 && len(p.Labels) != 2) ||
		(len(p.Labels) == 2 && p.Labels[0] != p.Labels[1]) {
		return nbpanel.Text{}, fmt.Errorf("expected a caption box and the same label in both states")
	}
	return panelText(*p.LabelBox, p.Labels[0])
}

func panelText(box bounds, label label) (nbpanel.Text, error) {
	if label.Align != 0 || label.Bold {
		return nbpanel.Text{}, fmt.Errorf("expected plain left-aligned text")
	}
	rgb, err := panelColor(label.Color)
	if err != nil {
		return nbpanel.Text{}, fmt.Errorf("text: %w", err)
	}
	return nbpanel.Text{
		X: box.X, Y: box.Y, Width: box.W, Height: box.H,
		Value: label.Text, Size: label.Size, RGB: rgb,
	}, nil
}

func panelDrawing(project fs.FS, d drawing) (nbpanel.Object, error) {
	if d.Kind == "text" {
		if d.Text == nil {
			return nil, fmt.Errorf("expected text settings")
		}
		text, err := panelText(d.Bounds, *d.Text)
		return text, err
	}
	if d.Kind == "image" {
		source, err := panelBitmapSource(project, d.Image)
		if err != nil {
			return nil, err
		}
		b := d.Bounds
		image, err := nbpanel.NewBitmap(source, b.X, b.Y, b.W, b.H)
		return image, err
	}
	shape, err := panelPrimitive(d)
	return shape, err
}

func panelPrimitive(d drawing) (nbpanel.Primitive, error) {
	shape := nbpanel.Primitive{
		Kind: d.Kind, X: d.Bounds.X, Y: d.Bounds.Y, Width: d.Bounds.W, Height: d.Bounds.H,
		Radius: d.Radius,
	}
	switch d.Kind {
	case "rect", "roundrect":
	case "line":
		shape.Points = []nbpanel.Point{{X: d.Bounds.X, Y: d.Bounds.Y}, {X: d.X2, Y: d.Y2}}
	case "polygon":
		shape.Points = make([]nbpanel.Point, 0, len(d.Points))
		for _, p := range d.Points {
			shape.Points = append(shape.Points, nbpanel.Point{X: p.X, Y: p.Y})
		}
	default:
		return shape, fmt.Errorf("unsupported vector primitive %q", d.Kind)
	}
	if d.Stroke != "" && d.Stroke != "none" {
		rgb, err := panelColor(d.Stroke)
		if err != nil {
			return shape, fmt.Errorf("stroke: %w", err)
		}
		shape.Stroke, shape.StrokeWidth = rgb, d.StrokeWidth
		if shape.StrokeWidth == 0 {
			shape.StrokeWidth = 1
		}
	}
	if d.Kind != "line" && d.Fill != "" && d.Fill != "none" {
		rgb, err := panelColor(d.Fill)
		if err != nil {
			return shape, fmt.Errorf("fill: %w", err)
		}
		shape.Fill = &rgb
	}
	if d.EndFill != "" {
		if (d.Kind != "rect" && d.Kind != "roundrect") || shape.Fill == nil {
			return shape, fmt.Errorf("a gradient needs a rectangle and a starting fill color")
		}
		rgb, err := panelColor(d.EndFill)
		if err != nil {
			return shape, fmt.Errorf("gradient end color: %w", err)
		}
		shape.EndFill = &rgb
	}
	return shape, nil
}

func panelBitmapSource(project fs.FS, ref string) ([]byte, error) {
	if payload, ok := strings.CutPrefix(ref, "data:"); ok {
		media, encoded, found := strings.Cut(payload, ",")
		if !found {
			return nil, fmt.Errorf("invalid image data URL")
		}
		switch media {
		case "image/bmp;base64", "image/png;base64", "image/jpeg;base64", "image/webp;base64":
		default:
			return nil, fmt.Errorf("expected a base64 BMP, PNG, JPEG or WebP data URL")
		}
		if len(encoded) > base64.StdEncoding.EncodedLen(maxSourceSize) {
			return nil, fmt.Errorf("embedded image exceeds %d bytes", maxSourceSize)
		}
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("invalid embedded image: %w", err)
		}
		if len(data) > maxSourceSize {
			return nil, fmt.Errorf("embedded image exceeds %d bytes", maxSourceSize)
		}
		return data, nil
	}
	u, err := url.Parse(ref)
	if err != nil || u.Scheme != "" || u.Host != "" || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("expected an image data URL or a file in hmi/static")
	}
	name := strings.TrimPrefix(u.Path, "/")
	if !fs.ValidPath(name) || strings.ContainsAny(name, "\\:") {
		return nil, fmt.Errorf("invalid local image path %q", ref)
	}
	f, err := project.Open(path.Join("hmi", "static", name))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxSourceSize {
		return nil, fmt.Errorf("expected a regular image file up to %d bytes", maxSourceSize)
	}
	data, err := io.ReadAll(io.LimitReader(f, maxSourceSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxSourceSize {
		return nil, fmt.Errorf("image exceeds %d bytes", maxSourceSize)
	}
	return data, nil
}

func panelColor(value string) (uint32, error) {
	digits, ok := strings.CutPrefix(value, "#")
	rgb, err := strconv.ParseUint(digits, 16, 24)
	if !ok || len(digits) != 6 || err != nil {
		return 0, fmt.Errorf("expected a #RRGGBB color")
	}
	return uint32(rgb), nil
}
