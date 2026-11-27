package nbpanel

import "fmt"

// VectorArtwork is a component state in its own coordinate space.
type VectorArtwork struct {
	Width, Height int
	Shapes        []Primitive
}

func NewVectorBitArtwork(off, on VectorArtwork) (*BitArtwork, error) {
	artwork := &BitArtwork{vector: true}
	for index, state := range [2]VectorArtwork{off, on} {
		record, err := encodeVectorArtwork(state)
		if err != nil {
			return nil, fmt.Errorf("state %d: %w", index, err)
		}
		artwork.states[index] = record
	}
	return artwork, nil
}

func encodeVectorArtwork(state VectorArtwork) ([]byte, error) {
	const (
		headerSize = 8
		slotSize   = 4
		maxSize    = 65535
	)
	if state.Width < 1 || state.Width > 32767 || state.Height < 1 || state.Height > 32767 {
		return nil, fmt.Errorf("invalid vector artwork dimensions")
	}
	if len(state.Shapes) > (maxSize-headerSize)/slotSize {
		return nil, fmt.Errorf("too many shapes in vector artwork")
	}
	canvas := Screen{Width: state.Width, Height: state.Height}
	data := make([]byte, headerSize+slotSize*len(state.Shapes))
	little.PutUint16(data, uint16(state.Width))
	little.PutUint16(data[2:], uint16(state.Height))
	little.PutUint32(data[4:], uint32(len(state.Shapes)))
	for i, shape := range state.Shapes {
		kind, record, err := encodePrimitive(canvas, shape)
		if err != nil {
			return nil, fmt.Errorf("shape %d: %w", i+1, err)
		}
		if len(record) > maxSize-len(data) {
			return nil, fmt.Errorf("vector artwork exceeds %d bytes", maxSize)
		}
		slot := headerSize + i*slotSize
		little.PutUint16(data[slot:], uint16(kind))
		little.PutUint16(data[slot+2:], uint16(len(data)))
		data = append(data, record...)
	}
	return data, nil
}
