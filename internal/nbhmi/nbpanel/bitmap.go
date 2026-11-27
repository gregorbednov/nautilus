package nbpanel

import (
	"bytes"
	"compress/lzw"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"

	"golang.org/x/image/webp"
)

const (
	bitmapComponent  = 0x108
	bitmapRecordSize = 12
	bitmapHeaderSize = 12
	bmpFileHeader    = 14
	bmpInfoHeader    = 40
)

type Bitmap struct {
	X, Y, Width, Height int
	resource            []byte
}

func NewBitmap(source []byte, x, y, width, height int) (Bitmap, error) {
	resource, err := bitmapResource(source)
	if err != nil {
		return Bitmap{}, err
	}
	return Bitmap{X: x, Y: y, Width: width, Height: height, resource: resource}, nil
}

func encodeBitmap(screen Screen, offset int, image Bitmap, pool *artworkPool) ([]byte, error) {
	if !fits(screen.Width, screen.Height, image.X, image.Y, image.Width, image.Height) {
		return nil, fmt.Errorf("bitmap bounds are outside the screen")
	}
	if len(image.resource) == 0 {
		return nil, fmt.Errorf("missing bitmap resource")
	}
	record := make([]byte, bitmapRecordSize)
	putBounds(record, image.X, image.Y, image.Width, image.Height)
	little.PutUint32(record[8:], uint32(pool.offset(offset+bitmapRecordSize)))
	return pool.store(record, image.resource)
}

func bitmapResource(source []byte) ([]byte, error) {
	var format string
	var config func(io.Reader) (image.Config, error)
	var decode func(io.Reader) (image.Image, error)
	switch {
	case bytes.HasPrefix(source, []byte("BM")):
		return bmpResource(source)
	case bytes.HasPrefix(source, []byte("\x89PNG\r\n\x1a\n")):
		format, config, decode = "PNG", png.DecodeConfig, png.Decode
	case bytes.HasPrefix(source, []byte("\xff\xd8")):
		format, config, decode = "JPEG", jpeg.DecodeConfig, jpeg.Decode
	case len(source) >= 12 && string(source[:4]) == "RIFF" && string(source[8:12]) == "WEBP":
		format, config, decode = "WebP", webp.DecodeConfig, webp.Decode
	default:
		return nil, fmt.Errorf("expected a BMP, PNG, JPEG or WebP image")
	}

	cfg, err := config(bytes.NewReader(source))
	if err != nil {
		return nil, fmt.Errorf("%s header: %w", format, err)
	}
	if err := checkBitmapDimensions(format, cfg.Width, cfg.Height); err != nil {
		return nil, err
	}
	img, err := decode(bytes.NewReader(source))
	if err != nil {
		return nil, fmt.Errorf("%s image: %w", format, err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != cfg.Width || bounds.Dy() != cfg.Height {
		return nil, fmt.Errorf("%s pixel dimensions differ from the header", format)
	}

	count := cfg.Width * cfg.Height
	pixels := make([]byte, 2*count)
	var alpha []byte
	i := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pixel := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			rgb := uint32(pixel.R)<<16 | uint32(pixel.G)<<8 | uint32(pixel.B)
			little.PutUint16(pixels[2*i:], opaqueRGB565(rgb))
			if alpha == nil && pixel.A != 255 {
				alpha = make([]byte, count)
				for j := range alpha[:i] {
					alpha[j] = 255
				}
			}
			if alpha != nil {
				alpha[i] = pixel.A
			}
			i++
		}
	}
	return compressBitmap(cfg.Width, cfg.Height, pixels, alpha)
}

func checkBitmapDimensions(format string, width, height int) error {
	if width < 1 || width > 32767 || height < 1 || height > 32767 {
		return fmt.Errorf("invalid %s dimensions", format)
	}
	if width > (maxPackageSize/3)/height {
		return fmt.Errorf("decoded %s exceeds the panel package size limit", format)
	}
	return nil
}

func bmpResource(source []byte) ([]byte, error) {
	if len(source) < bmpFileHeader+bmpInfoHeader {
		return nil, fmt.Errorf("truncated BMP header")
	}
	dib, start := u32(source, 14), u32(source, 10)
	if dib < bmpInfoHeader || dib > len(source)-bmpFileHeader ||
		start < bmpFileHeader+dib || start > len(source) || little.Uint16(source[26:]) != 1 {
		return nil, fmt.Errorf("invalid BMP header")
	}
	width := int(int32(little.Uint32(source[18:])))
	signedHeight := int(int32(little.Uint32(source[22:])))
	if signedHeight < -32767 {
		return nil, fmt.Errorf("invalid BMP dimensions")
	}
	height := max(signedHeight, -signedHeight)
	if err := checkBitmapDimensions("BMP", width, height); err != nil {
		return nil, err
	}
	bits, compression := int(little.Uint16(source[28:])), u32(source, 30)
	switch {
	case (bits == 24 || bits == 32) && compression == 0:
	case bits == 16 && compression == 3 && start >= 66 &&
		u32(source, 54) == 0xf800 && u32(source, 58) == 0x07e0 && u32(source, 62) == 0x001f:
	default:
		return nil, fmt.Errorf("expected 24-bit or 32-bit BI_RGB, or 16-bit RGB565 BI_BITFIELDS BMP")
	}
	stride := ((width*bits + 31) / 32) * 4
	if height > (len(source)-start)/stride {
		return nil, fmt.Errorf("truncated BMP pixel data")
	}
	pixels := make([]byte, 2*width*height)
	var alpha []byte
	if bits == 32 {
		alpha = make([]byte, width*height)
	}
	hasNonzeroAlpha, hasTransparency := false, false
	for y := range height {
		row := y
		if signedHeight > 0 {
			row = height - 1 - y
		}
		for x := range width {
			src := start + row*stride + x*(bits/8)
			dst := y*width + x
			switch bits {
			case 16:
				packed := little.Uint16(source[src:])
				if packed == 0 {
					packed = opaqueBlack565
				}
				little.PutUint16(pixels[2*dst:], packed)
			case 24, 32:
				rgb := uint32(source[src+2])<<16 | uint32(source[src+1])<<8 | uint32(source[src])
				little.PutUint16(pixels[2*dst:], opaqueRGB565(rgb))
			}
			if bits == 32 {
				alpha[dst] = source[src+3]
				hasNonzeroAlpha = hasNonzeroAlpha || alpha[dst] != 0
				hasTransparency = hasTransparency || alpha[dst] != 255
			}
		}
	}
	if !hasNonzeroAlpha || !hasTransparency {
		// In BI_RGB the high byte may be reserved and zero in every pixel.
		alpha = nil
	}
	return compressBitmap(width, height, pixels, alpha)
}

func compressBitmap(width, height int, pixels, alpha []byte) ([]byte, error) {
	out := bytes.NewBuffer(make([]byte, bitmapHeaderSize))
	writer := lzw.NewWriter(out, lzw.LSB, 8)
	if _, err := writer.Write(pixels); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	header := out.Bytes()[:bitmapHeaderSize]
	little.PutUint16(header, uint16(width))
	little.PutUint16(header[2:], uint16(height))
	header[4], header[5] = 16, 1 // RGB565, LZW compression.
	if len(alpha) > 0 {
		little.PutUint16(header[6:], 2) // One alpha byte per uncompressed pixel.
	}
	little.PutUint32(header[8:], uint32(out.Len()-bitmapHeaderSize))
	out.Write(alpha)
	return out.Bytes(), nil
}
