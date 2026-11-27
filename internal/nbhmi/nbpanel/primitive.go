package nbpanel

import "fmt"

const (
	lineComponent      = 0x101
	rectangleComponent = 0x103
	polygonComponent   = 0x104
	opaqueBlack565     = 0x0821
	verticalGradient   = 0x100
)

type Point struct{ X, Y int }

type Primitive struct {
	Kind                string
	X, Y, Width, Height int
	Radius              int
	StrokeWidth         int
	Stroke              uint32
	Fill                *uint32
	EndFill             *uint32
	Points              []Point
}

func encodePrimitive(screen Screen, shape Primitive) (uint32, []byte, error) {
	if shape.StrokeWidth < 0 || shape.StrokeWidth > min(screen.Width, screen.Height) ||
		shape.Stroke > 0xffffff || (shape.Fill != nil && *shape.Fill > 0xffffff) {
		return 0, nil, fmt.Errorf("invalid primitive stroke or fill")
	}
	if shape.EndFill != nil &&
		(shape.Fill == nil || *shape.EndFill > 0xffffff || (shape.Kind != "rect" && shape.Kind != "roundrect")) {
		return 0, nil, fmt.Errorf("a gradient needs a rectangle and two fill colors")
	}
	switch shape.Kind {
	case "rect", "roundrect":
		if !fits(screen.Width, screen.Height, shape.X, shape.Y, shape.Width, shape.Height) {
			return 0, nil, fmt.Errorf("rectangle bounds are outside the screen")
		}
		if shape.StrokeWidth == 0 && shape.Fill == nil {
			return 0, nil, fmt.Errorf("a rectangle needs a stroke or fill")
		}
		record := make([]byte, 32)
		putBounds(record, shape.X, shape.Y, shape.Width, shape.Height)
		putPrimitiveColors(record, shape)
		if shape.Kind == "roundrect" {
			if shape.Radius < 1 || min(shape.Width, shape.Height) < 2 {
				return 0, nil, fmt.Errorf("a rounded rectangle needs a positive corner radius and at least two pixels on each side")
			}
			little.PutUint16(record[28:], uint16(min(shape.Radius, min(shape.Width, shape.Height)/2)))
		}
		return rectangleComponent, record, nil
	case "line":
		if len(shape.Points) != 2 || shape.StrokeWidth == 0 {
			return 0, nil, fmt.Errorf("a line needs two points and a stroke")
		}
		record, err := encodePrimitivePath(screen, shape, 20)
		if err != nil {
			return 0, nil, err
		}
		little.PutUint16(record[18:], 2)
		return lineComponent, record, nil
	case "polygon":
		if len(shape.Points) < 3 || len(shape.Points) > 65535 ||
			(shape.StrokeWidth == 0 && shape.Fill == nil) {
			return 0, nil, fmt.Errorf("a polygon needs three or more points and a stroke or fill")
		}
		record, err := encodePrimitivePath(screen, shape, 32)
		if err != nil {
			return 0, nil, err
		}
		little.PutUint32(record[28:], uint32(len(shape.Points)))
		return polygonComponent, record, nil
	default:
		return 0, nil, fmt.Errorf("unsupported vector primitive %q", shape.Kind)
	}
}

func encodePrimitivePath(screen Screen, shape Primitive, header int) ([]byte, error) {
	first := shape.Points[0]
	left, right, top, bottom := first.X, first.X, first.Y, first.Y
	for _, p := range shape.Points {
		if !fits(screen.Width, screen.Height, p.X, p.Y, 1, 1) {
			return nil, fmt.Errorf("point (%d, %d) is outside the screen", p.X, p.Y)
		}
		left, right = min(left, p.X), max(right, p.X)
		top, bottom = min(top, p.Y), max(bottom, p.Y)
	}
	margin := max(1, shape.StrokeWidth)
	left, top = left-margin, top-margin
	right, bottom = right+margin-1, bottom+margin-1
	record := make([]byte, header+len(shape.Points)*4)
	putBounds(record, left, top, right-left+1, bottom-top+1)
	putPrimitiveColors(record, shape)
	for i, p := range shape.Points {
		at := header + 4*i
		little.PutUint16(record[at:], uint16(p.X-left))
		little.PutUint16(record[at+2:], uint16(p.Y-top))
	}
	return record, nil
}

func putPrimitiveColors(record []byte, shape Primitive) {
	pen := uint16(0)
	color := uint16(opaqueBlack565)
	if shape.StrokeWidth > 0 {
		pen, color = 1, opaqueRGB565(shape.Stroke)
	}
	little.PutUint16(record[8:], pen)
	little.PutUint16(record[10:], uint16(max(1, shape.StrokeWidth)))
	little.PutUint16(record[12:], color)
	if shape.Fill != nil && shape.Kind != "line" {
		mode, end := uint16(1), color
		if shape.EndFill != nil && shape.Height > 1 {
			mode, end = verticalGradient, opaqueRGB565(*shape.EndFill)
		}
		little.PutUint16(record[16:], mode)
		little.PutUint16(record[20:], opaqueRGB565(*shape.Fill))
		little.PutUint16(record[24:], end)
	}
}

func opaqueRGB565(rgb uint32) uint16 {
	if packed := rgb565(rgb); packed != 0 {
		return packed
	}
	// The Omron renderer uses RGB565 zero as a transparency key.
	return opaqueBlack565
}
