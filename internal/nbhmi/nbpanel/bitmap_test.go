package nbpanel

import (
	"bytes"
	"compress/lzw"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"testing"
)

func TestBitmapCompilesRGB565AndAlpha(t *testing.T) {
	opaque := bmpFixture(24, 2, []byte{
		// Bottom row, including row padding.
		255, 0, 0, 0, 0, 0, 0, 0,
		// Top row.
		0, 0, 255, 0, 255, 0, 0, 0,
	})
	alpha := bmpFixture(32, -2, []byte{
		0, 0, 255, 255, 0, 255, 0, 0,
		255, 0, 0, 120, 0, 0, 0, 255,
	})
	for _, tc := range []struct {
		name   string
		source []byte
		alpha  []byte
	}{
		{"bottom up, padded", opaque, nil},
		{"top down with alpha", alpha, []byte{255, 0, 120, 255}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			img, err := NewBitmap(tc.source, 40, 50, 200, 100)
			if err != nil {
				t.Fatal(err)
			}
			const offset = 1024
			record, err := encodeBitmap(Screen{Width: 800, Height: 480}, offset, img, nil)
			if err != nil {
				t.Fatal(err)
			}
			if u32(record, 8) != offset+bitmapRecordSize || little.Uint16(record[4:]) != 239 {
				t.Fatal("incorrect bitmap bounds or resource offset")
			}
			resource := record[bitmapRecordSize:]
			if little.Uint16(resource) != 2 || little.Uint16(resource[2:]) != 2 ||
				resource[4] != 16 || resource[5] != 1 {
				t.Fatalf("incorrect bitmap header: %x", resource[:bitmapHeaderSize])
			}
			if got := little.Uint16(resource[6:]); (got == 2) != (len(tc.alpha) != 0) {
				t.Fatalf("incorrect alpha flag %d", got)
			}
			checkBitmapPixels(t, resource, []byte{0, 0xf8, 0xe0, 0x07, 0x1f, 0, 0x21, 0x08}, tc.alpha)
		})
	}
	if _, err := NewBitmap(alpha[:len(alpha)-1], 0, 0, 2, 2); err == nil {
		t.Fatal("truncated BMP was accepted")
	}
}

func TestBitmapModernFormats(t *testing.T) {
	bounds := image.Rect(0, 0, 2, 2)
	mixed := image.NewNRGBA(bounds)
	mixed.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	mixed.SetNRGBA(0, 1, color.NRGBA{B: 255, A: 120})
	mixed.SetNRGBA(1, 1, color.NRGBA{A: 255})
	var rgbaPNG, clearPNG, opaqueJPEG bytes.Buffer
	if err := png.Encode(&rgbaPNG, mixed); err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(&clearPNG, image.NewNRGBA(bounds)); err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(&opaqueJPEG, image.NewGray(bounds), nil); err != nil {
		t.Fatal(err)
	}
	// Lossless WebP encoding of the same 2×2 RGBA pixels as mixed.
	rgbaWebP, err := base64.StdEncoding.DecodeString("UklGRiwAAABXRUJQVlA4TCAAAAAvAUAAEB8QAoIi/0ebfwFB0W3rBXgaFKRtwKL9RfQ/dA==")
	if err != nil {
		t.Fatal(err)
	}

	mixedRGB565 := []byte{0, 0xf8, 0x21, 0x08, 0x1f, 0, 0x21, 0x08}
	mixedAlpha := []byte{255, 0, 120, 255}
	blackRGB565 := bytes.Repeat([]byte{0x21, 0x08}, 4)
	for _, tc := range []struct {
		name   string
		source []byte
		pixels []byte
		alpha  []byte
	}{
		{"PNG with alpha", rgbaPNG.Bytes(), mixedRGB565, mixedAlpha},
		{"fully transparent PNG", clearPNG.Bytes(), blackRGB565, []byte{0, 0, 0, 0}},
		{"JPEG", opaqueJPEG.Bytes(), blackRGB565, nil},
		{"WebP with alpha", rgbaWebP, mixedRGB565, mixedAlpha},
	} {
		t.Run(tc.name, func(t *testing.T) {
			img, err := NewBitmap(tc.source, 0, 0, 2, 2)
			if err != nil {
				t.Fatal(err)
			}
			header := img.resource[:bitmapHeaderSize]
			if little.Uint16(header) != 2 || little.Uint16(header[2:]) != 2 {
				t.Fatalf("incorrect bitmap dimensions: %x", header[:4])
			}
			flag := uint16(0)
			if len(tc.alpha) > 0 {
				flag = 2
			}
			if got := little.Uint16(header[6:]); got != flag {
				t.Fatalf("alpha flag = %d, want %d", got, flag)
			}
			checkBitmapPixels(t, img.resource, tc.pixels, tc.alpha)
		})
	}
}

func checkBitmapPixels(t *testing.T, resource, wantPixels, wantAlpha []byte) {
	t.Helper()
	end := bitmapHeaderSize + u32(resource, 8)
	decoder := lzw.NewReader(bytes.NewReader(resource[bitmapHeaderSize:end]), lzw.LSB, 8)
	defer decoder.Close()
	pixels, err := io.ReadAll(decoder)
	if err != nil || !bytes.Equal(pixels, wantPixels) {
		t.Fatalf("RGB565 pixels = %x, want %x, error %v", pixels, wantPixels, err)
	}
	if !bytes.Equal(resource[end:], wantAlpha) {
		t.Fatalf("alpha plane = %x, want %x", resource[end:], wantAlpha)
	}
}

func bmpFixture(bits uint16, height int32, pixels []byte) []byte {
	const header = bmpFileHeader + bmpInfoHeader
	data := make([]byte, header+len(pixels))
	copy(data, "BM")
	little.PutUint32(data[2:], uint32(len(data)))
	little.PutUint32(data[10:], header)
	little.PutUint32(data[14:], bmpInfoHeader)
	little.PutUint32(data[18:], 2)
	little.PutUint32(data[22:], uint32(height))
	little.PutUint16(data[26:], 1)
	little.PutUint16(data[28:], bits)
	copy(data[header:], pixels)
	return data
}
