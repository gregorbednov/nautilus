package um

import (
	"bytes"
	"errors"
	"fmt"
)

// ErrN1TaskNotFound means a validated N1 image has no record for the task.
var ErrN1TaskNotFound = errors.New("task not present in N1 image")

// ExtractN1TaskRecord returns an owned, byte-exact task record from a validated
// full N1 image, including its header and entry/exit wrapper. It preserves
// opaque fields and unknown instructions without re-encoding them.
//
// The result excludes the global N1 header, directory and auxiliary blocks.
// It is a record for inspection/comparison, not a qualified standalone compiler
// file or task-service payload. In particular, FB dependencies are not added.
func ExtractN1TaskRecord(data []byte, number uint16) ([]byte, error) {
	image, err := ParseN1(data)
	if err != nil {
		return nil, err
	}
	for _, task := range image.Tasks {
		if task.Number == number {
			start := int(task.OffsetWords) * 4
			end := start + int(task.LengthWords)*4
			return bytes.Clone(image.raw[start:end]), nil
		}
	}
	return nil, fmt.Errorf("%w: %d", ErrN1TaskNotFound, number)
}
