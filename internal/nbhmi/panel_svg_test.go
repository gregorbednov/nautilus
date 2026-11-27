package nbhmi

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
)

func TestPanelSVGArtworkReadsImporterOutput(t *testing.T) {
	const start = `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100" viewBox="0 0 100 100" preserveAspectRatio="none">`
	t.Run("rounded gradient", func(t *testing.T) {
		var svg strings.Builder
		svg.WriteString(start)
		d := drawing{Kind: "roundrect", Bounds: bounds{5, 7, 50, 60}, Radius: 9,
			Fill: "#f0ab00", EndFill: "#231155", Stroke: "#001122", StrokeWidth: 1}
		d.writeSVG(&svg, 0)
		invisible := drawing{Kind: "rect", Bounds: bounds{0, 0, 100, 100},
			Fill: "none", Stroke: "none", StrokeWidth: 1}
		invisible.writeSVG(&svg, 1)
		svg.WriteString("</svg>")
		got, raster, err := panelSVGArtwork(nil, []byte(svg.String()))
		if err != nil || raster != nil || got.Width != 100 || got.Height != 100 || len(got.Shapes) != 1 {
			t.Fatalf("reading imported gradient: state %+v, error %v", got, err)
		}
		shape := got.Shapes[0]
		if shape.Kind != "roundrect" || shape.Radius != 9 || shape.Stroke != 0x001122 ||
			shape.Fill == nil || *shape.Fill != 0xf0ab00 || shape.EndFill == nil || *shape.EndFill != 0x231155 {
			t.Fatalf("unexpected gradient shape: %+v", shape)
		}
	})
	t.Run("embedded raster uses original bytes", func(t *testing.T) {
		bitmap := []byte{0x42, 0x4d, 0x00}
		href := "data:image/bmp;base64," + base64.StdEncoding.EncodeToString(bitmap)
		var svg strings.Builder
		svg.WriteString(start)
		d := drawing{Kind: "image", Bounds: bounds{0, 0, 100, 100}, Image: href}
		d.writeSVG(&svg, 0)
		svg.WriteString("</svg>")
		_, raster, err := panelSVGArtwork(nil, []byte(svg.String()))
		if err != nil || !bytes.Equal(raster, bitmap) {
			t.Fatalf("extracting embedded raster: got %x, error %v", raster, err)
		}
	})
	t.Run("external image is rejected", func(t *testing.T) {
		source := fmt.Sprintf(`%s<image x="0" y="0" width="100" height="100" href="https://example.com/test.png" preserveAspectRatio="none"/></svg>`, start)
		if _, _, err := panelSVGArtwork(nil, []byte(source)); err == nil {
			t.Fatal("external SVG image was accepted")
		}
	})
}
