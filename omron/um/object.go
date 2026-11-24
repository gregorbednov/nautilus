package um

import (
	"bytes"
	"errors"
	"fmt"
)

// PlainN1ObjectPrefixBytes is the zero prefix written by the observed
// CX-Programmer CJ1 local-file writer when extended protection is disabled.
// This is not part of the raw N1 or a PROGRAM AREA WRITE payload.
const PlainN1ObjectPrefixBytes = 0x30

var ErrPlainN1ObjectLayout = errors.New("unsupported or malformed plain N1 object")

// WrapPlainN1Object constructs only the observed plain CJ1 local container:
// 48 zero bytes followed by an unchanged, validated N1 image. It does not
// encrypt, add metadata, modify service sums, or qualify the image for a PLC.
func WrapPlainN1Object(n1 []byte) ([]byte, error) {
	if _, err := ParseN1(n1); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrPlainN1ObjectLayout, err)
	}
	data := make([]byte, PlainN1ObjectPrefixBytes+len(n1))
	copy(data[PlainN1ObjectPrefixBytes:], n1)
	return data, nil
}

// ParsePlainN1Object explicitly selects the plain CJ1 container subset.
// Protected headers, CJ2 headers, and appended metadata are unsupported.
// The returned N1 image owns its bytes; its Original method returns the body,
// without the container prefix. No format autodetection or decryption occurs.
func ParsePlainN1Object(data []byte) (*N1Image, error) {
	if len(data) < PlainN1ObjectPrefixBytes || len(data) > PlainN1ObjectPrefixBytes+MaxSize {
		return nil, fmt.Errorf("%w: invalid container size", ErrPlainN1ObjectLayout)
	}
	if !bytes.Equal(data[:PlainN1ObjectPrefixBytes], make([]byte, PlainN1ObjectPrefixBytes)) {
		return nil, fmt.Errorf("%w: nonzero container prefix", ErrPlainN1ObjectLayout)
	}
	image, err := ParseN1(data[PlainN1ObjectPrefixBytes:])
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrPlainN1ObjectLayout, err)
	}
	return image, nil
}
